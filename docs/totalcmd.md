# TotalCMD-Style File Transfer (Design + Reference Code)

Målet er et todelt filbrowser (lokal/remote) med dobbeltklik-navigation og robust overførsel via en dedikeret reliable datachannel (`label: "file"`). Her er et selvstændigt referenceoplæg, der kan implementeres direkte i jeres controller/agent.

## Datachannel og protokol

```go
// Reliable + ordered for file transfer
fileDC, _ := pc.CreateDataChannel("file", &webrtc.DataChannelInit{
    Ordered:        ref(true),
    MaxRetransmits: nil,
})

// Wire-format (JSON header + binary payload for chunks)
type Msg struct {
    Op      string `json:"op"`               // "list","get","put","mkdir","rm","mv","ack","err","progress"
    Path    string `json:"path,omitempty"`
    Target  string `json:"target,omitempty"` // mv dest
    Size    int64  `json:"size,omitempty"`
    Mode    uint32 `json:"mode,omitempty"`   // optional chmod-ish
    FrameID uint16 `json:"fid,omitempty"`    // transfer/frame id
    Chunk   uint16 `json:"c,omitempty"`      // chunk index
    Total   uint16 `json:"t,omitempty"`      // total chunks
    Offset  int64  `json:"off,omitempty"`    // resume offset
    Error   string `json:"error,omitempty"`
    Entries []Entry `json:"entries,omitempty"` // for list response
}

type Entry struct {
    Name string `json:"name"`
    Path string `json:"path"`
    Dir  bool   `json:"dir"`
    Size int64  `json:"size"`
    Mod  int64  `json:"mod"`
}
```

### Chunking
- Max payload pr. chunk: fx 60 KB (under 64 KB datachannel-grænsen).
- `FrameID` unik per transfer.
- ACK hvert N. chunk (fx hver 64.) eller slut (`Op:"ack", fid, c`).
- Resume: ved reconnect eller fejl send `Op:"get"`/`"put"` med `Offset` = sidste bekræftede byte.

## Agent-side handlers (pseudokode i Go)

```go
func onFileMsg(msg Msg, payload []byte) {
    switch msg.Op {
    case "list":
        entries := listDir(safeRootJoin(msg.Path))
        send(Msg{Op:"list", Path:msg.Path, Entries:entries})

    case "mkdir":
        mkdirAll(safeRootJoin(msg.Path))
        send(Msg{Op:"ack", Path:msg.Path})

    case "rm":
        remove(safeRootJoin(msg.Path))
        send(Msg{Op:"ack", Path:msg.Path})

    case "mv":
        rename(safeRootJoin(msg.Path), safeRootJoin(msg.Target))
        send(Msg{Op:"ack", Path:msg.Path, Target:msg.Target})

    case "get": // controller henter fra agent
        streamFile(msg.Path, msg.FrameID, msg.Offset)

    case "put": // controller uploader til agent
        appendChunk(msg.Path, msg.FrameID, msg.Chunk, msg.Total, payload, msg.Offset)
        if msg.Chunk == msg.Total-1 {
            send(Msg{Op:"ack", Fid:msg.FrameID, Path:msg.Path})
        } else if msg.Chunk%64 == 0 {
            send(Msg{Op:"ack", Fid:msg.FrameID, C:msg.Chunk})
        }
    }
}
```

`streamFile` læser filen i 60 KB chunks og sender `Msg{Op:"put", ...}` med payload = file bytes, samt periodic progress:

```go
func streamFile(path string, fid uint16, offset int64) {
    f, _ := os.Open(path)
    defer f.Close()
    f.Seek(offset, 0)
    buf := make([]byte, 60000)
    var chunk uint16
    for {
        n, err := f.Read(buf)
        if n > 0 {
            sendMsgWithPayload(Msg{Op:"put", Fid:fid, Chunk:chunk, Total:0, Path:path}, buf[:n])
            chunk++
            if chunk%64 == 0 { send(Msg{Op:"progress", Fid:fid, Path:path, Size:int64(n)}) }
        }
        if err == io.EOF { break }
        if err != nil { send(Msg{Op:"err", Fid:fid, Error:err.Error()}); return }
    }
    send(Msg{Op:"ack", Fid:fid, Path:path})
}
```

## Controller UI (Total Commander-style)
- To paneler: `LocalPane` og `RemotePane` (hver har `path`, entries[], list widget).
- Dobbeltklik på mappe: `list(path/dir)` (lokal: OS; remote: send `Op:"list"`).
- Dobbeltklik på fil:
  - Hvis i RemotePane ⇒ start download til LocalPane’s current path (`Op:"get"`).
  - Hvis i LocalPane ⇒ start upload til RemotePane’s current path (`Op:"put"`).
- Toolbar: Up, Refresh, New Folder, Delete, Rename, Copy path.
- Status: aktiv fil, progress, hastighed, ETA, kø.

### Controller-queue (pseudokode)
```go
type Job struct {
    Fid     uint16
    Op      string // "download" or "upload"
    SrcPath string
    DstPath string
    Size    int64
    Offset  int64
}

func (q *Queue) startNext() {
    if q.active != nil { return }
    job := q.pop()
    q.active = job
    if job.Op == "download" {
        send(Msg{Op:"get", Path:job.SrcPath, Fid:job.Fid, Offset:job.Offset})
    } else {
        // chunk file locally and send Op:"put"
    }
}

func onAck(msg Msg) {
    if msg.Fid == q.active.Fid {
        q.finishActive()
        q.startNext()
    }
}
```

## Sikkerhed og roots
- Agenten arbejder under en “root” (fx `C:\Users\<User>\RemoteDesktopRoot` eller `Downloads\RemoteDesktop`). Afvis path traversal udenfor root.
- Frasortér skjulte/systemfiler hvis ønsket.

## Dobbelklik-nav + tastatur
- Doubleclick = open dir eller transfer fil.
- Enter = samme som doubleclick.
- F5 = kopiér til modsatte panel (download/upload afhængigt af fokus).
- Del = delete.
- F7 = ny mappe.

## Robusthed
- Reliable channel, chunk-ack hver 64. chunk + slut-ack.
- Resume: gem `lastAckedOffset` pr. fid; ved fejl/afbrud send ny `get/put` med `Offset`.
- Progress events (`Op:"progress"`) for UI.
- Timeout på ACK → pause job og tilbyd Resume/Restart.

## Minimum første inkrement
1) Opret reliable “file” datachannel.
2) Implementér `list/get/put` med chunking + ack (uden resume) + progress events.
3) Byg 2-panel UI med dobbeltklik navigation + download/upload.
4) Tilføj kø og simpel fejltoast.
5) (Senere) Resume og mv/rm/mkdir/rename/kopiér sti.

Det her er nok til at vise end-to-end flow og kan løftes direkte ind i jeres controller/agent kode. Brug det som blueprint og tilpas felter/navne til jeres eksisterende logging og typer. 
