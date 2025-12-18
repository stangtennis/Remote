package filetransfer

// Message types for file transfer protocol
type Message struct {
	Op      string      `json:"op"`               // "list","get","put","mkdir","rm","mv","ack","err","progress","drives"
	Path    string      `json:"path,omitempty"`   // File/directory path
	Target  string      `json:"target,omitempty"` // mv destination
	Size    int64       `json:"size,omitempty"`   // File size
	Mode    interface{} `json:"mode,omitempty"`   // File mode/permissions (can be string or number)
	FrameID uint16      `json:"fid,omitempty"`    // Transfer/frame ID
	Chunk   uint16      `json:"c,omitempty"`      // Chunk index
	Total   uint16      `json:"t,omitempty"`      // Total chunks
	Offset  int64       `json:"off,omitempty"`    // Resume offset
	Error   string      `json:"error,omitempty"`  // Error message
	Entries []Entry     `json:"entries,omitempty"`// Directory entries
	Data    []byte      `json:"data,omitempty"`   // Binary data for chunks
}

// Entry represents a file or directory
type Entry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"dir"`
	Size  int64  `json:"size"`
	Mod   int64  `json:"mod"` // Unix timestamp
}

// Job represents a transfer job in the queue
type Job struct {
	ID      uint16
	Op      string // "download" or "upload"
	SrcPath string
	DstPath string
	Size    int64
	Offset  int64
	Done    int64 // Bytes transferred
}

// Transfer represents a file transfer (legacy compatibility)
type Transfer struct {
	ID         string
	Filename   string
	Size       int64
	Received   int64
	Status     string
	onProgress func(progress, total int64)
	onComplete func(success bool, err error)
}

// SetOnProgress sets progress callback
func (t *Transfer) SetOnProgress(f func(progress, total int64)) {
	t.onProgress = f
}

// SetOnComplete sets completion callback
func (t *Transfer) SetOnComplete(f func(success bool, err error)) {
	t.onComplete = f
}

// Constants
const (
	ChunkSize    = 60000 // 60KB chunks (under 64KB datachannel limit)
	AckInterval  = 64    // ACK every N chunks
	MaxRetries   = 3
)

// Operation types
const (
	OpList     = "list"
	OpGet      = "get"
	OpPut      = "put"
	OpMkdir    = "mkdir"
	OpRm       = "rm"
	OpMv       = "mv"
	OpAck      = "ack"
	OpErr      = "err"
	OpProgress = "progress"
	OpDrives   = "drives"
)
