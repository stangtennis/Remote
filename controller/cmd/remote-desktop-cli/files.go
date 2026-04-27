package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// fileTransferRouter dispatches file-channel messages to subscribers keyed by
// fid (file id). Both upload and download use one fid per transfer; multiple
// transfers can run concurrently with distinct fids.
type fileTransferRouter struct {
	mu   sync.Mutex
	subs map[uint16]chan map[string]interface{}
}

func newFileTransferRouter() *fileTransferRouter {
	return &fileTransferRouter{subs: make(map[uint16]chan map[string]interface{})}
}

func (r *fileTransferRouter) Subscribe(fid uint16) chan map[string]interface{} {
	ch := make(chan map[string]interface{}, 256)
	r.mu.Lock()
	r.subs[fid] = ch
	r.mu.Unlock()
	return ch
}

func (r *fileTransferRouter) Unsubscribe(fid uint16) {
	r.mu.Lock()
	if ch, ok := r.subs[fid]; ok {
		close(ch)
		delete(r.subs, fid)
	}
	r.mu.Unlock()
}

func (r *fileTransferRouter) Dispatch(data []byte) {
	var msg map[string]interface{}
	if err := json.Unmarshal(data, &msg); err != nil {
		return
	}
	fidF, ok := msg["fid"].(float64)
	if !ok {
		return
	}
	fid := uint16(fidF)
	r.mu.Lock()
	ch := r.subs[fid]
	r.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- msg:
	default:
	}
}

// downloadRemoteFile fetches a remote file from the agent and writes it to
// localPath. Returns the number of bytes written.
func downloadRemoteFile(conn *DeviceConnection, remotePath, localPath string, timeout time.Duration) (int64, error) {
	fid := nextFileID()
	sub := conn.fileRouter.Subscribe(fid)
	defer conn.fileRouter.Unsubscribe(fid)

	getMsg, _ := json.Marshal(map[string]interface{}{
		"op":   "get",
		"path": remotePath,
		"fid":  fid,
		"off":  0,
	})
	if err := conn.SendFile(getMsg); err != nil {
		return 0, fmt.Errorf("send get: %w", err)
	}

	// Open local destination — create parent if needed
	if err := os.MkdirAll(filepath.Dir(localPath), 0755); err != nil {
		return 0, fmt.Errorf("mkdir: %w", err)
	}
	f, err := os.Create(localPath)
	if err != nil {
		return 0, fmt.Errorf("create local: %w", err)
	}
	defer f.Close()

	deadline := time.After(timeout)
	var written int64
	for {
		select {
		case msg, ok := <-sub:
			if !ok {
				return written, fmt.Errorf("channel closed unexpectedly")
			}
			op, _ := msg["op"].(string)
			switch op {
			case "put":
				if errStr, isErr := msg["error"].(string); isErr {
					return written, fmt.Errorf("agent error: %s", errStr)
				}
				dataStr, _ := msg["data"].(string)
				data, err := base64.StdEncoding.DecodeString(dataStr)
				if err != nil {
					return written, fmt.Errorf("decode chunk: %w", err)
				}
				if _, err := f.Write(data); err != nil {
					return written, fmt.Errorf("write local: %w", err)
				}
				written += int64(len(data))
				cF, _ := msg["c"].(float64)
				tF, _ := msg["t"].(float64)
				if tF > 0 && uint16(cF) == uint16(tF)-1 {
					// Last chunk written; the agent will send an ack but we can
					// finalize here once we observe the final chunk.
				}
			case "ack":
				return written, nil
			case "err":
				errStr, _ := msg["error"].(string)
				return written, fmt.Errorf("agent error: %s", errStr)
			}
		case <-deadline:
			return written, fmt.Errorf("download timeout after %s", timeout)
		}
	}
}

// uploadLocalFile streams a local file to the agent at remotePath using the
// existing put-chunked protocol. Returns total bytes uploaded.
func uploadLocalFile(conn *DeviceConnection, localPath, remotePath string, timeout time.Duration) (int64, error) {
	f, err := os.Open(localPath)
	if err != nil {
		return 0, fmt.Errorf("open local: %w", err)
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return 0, fmt.Errorf("stat local: %w", err)
	}
	fileSize := stat.Size()
	if fileSize == 0 {
		return 0, fmt.Errorf("refuse to upload empty file")
	}

	const chunkSize = 60000
	totalChunks := uint16((fileSize + chunkSize - 1) / chunkSize)
	if int64(totalChunks)*chunkSize < fileSize {
		// fileSize > max representable in uint16 chunks of 60KB (~3.9GB)
		return 0, fmt.Errorf("file too large (max ~3.9GB)")
	}

	fid := nextFileID()
	sub := conn.fileRouter.Subscribe(fid)
	defer conn.fileRouter.Unsubscribe(fid)

	buf := make([]byte, chunkSize)
	var written int64
	var chunk uint16
	for {
		n, readErr := f.Read(buf)
		if n > 0 {
			// Encode chunk data as base64 — wire format expected by handlePutOp
			b64 := base64.StdEncoding.EncodeToString(buf[:n])
			msg, _ := json.Marshal(map[string]interface{}{
				"op":   "put",
				"path": remotePath,
				"fid":  fid,
				"c":    chunk,
				"t":    totalChunks,
				"size": fileSize,
				"data": b64,
			})
			if err := conn.SendFile(msg); err != nil {
				return written, fmt.Errorf("send put: %w", err)
			}
			written += int64(n)
			chunk++

			// Drain any pending ACKs (non-blocking) for flow control awareness
			drainAcks(sub)
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return written, fmt.Errorf("read local: %w", readErr)
		}
	}

	// Wait for final ack
	deadline := time.After(timeout)
	for {
		select {
		case msg, ok := <-sub:
			if !ok {
				return written, fmt.Errorf("channel closed unexpectedly")
			}
			op, _ := msg["op"].(string)
			if op == "ack" {
				if _, hasC := msg["c"]; !hasC {
					// Final ack (no chunk index)
					return written, nil
				}
				continue
			}
			if op == "err" {
				errStr, _ := msg["error"].(string)
				return written, fmt.Errorf("agent error: %s", errStr)
			}
		case <-deadline:
			return written, fmt.Errorf("upload timeout after %s", timeout)
		}
	}
}

func drainAcks(ch chan map[string]interface{}) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}

var (
	fileIDMu  sync.Mutex
	fileIDCtr uint16
)

func nextFileID() uint16 {
	fileIDMu.Lock()
	defer fileIDMu.Unlock()
	fileIDCtr++
	if fileIDCtr == 0 {
		fileIDCtr = 1
	}
	return fileIDCtr
}
