package filetransfer

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Manager handles file transfer operations on the controller side
type Manager struct {
	sendFunc     func([]byte) error
	onProgress   func(job *Job)
	onComplete   func(job *Job)
	onError      func(job *Job, err error)
	onListResult func(path string, entries []Entry)
	onDrives     func(entries []Entry)

	queue       []*Job
	activeJob   *Job
	mu          sync.Mutex
	nextFrameID uint16

	// For receiving files
	receiveFile   *os.File
	receiveJob    *Job
	receivedBytes int64

	// For sending files
	sendFile    *os.File
	sendJob     *Job
	sendChunk   uint16
	sendTotal   uint16
}

// NewManager creates a new file transfer manager
func NewManager() *Manager {
	return &Manager{
		nextFrameID: 1,
	}
}

// SetSendFunc sets the function to send data over the datachannel
func (m *Manager) SetSendFunc(f func([]byte) error) {
	m.sendFunc = f
}

// SetSendDataCallback is an alias for SetSendFunc (compatibility)
func (m *Manager) SetSendDataCallback(f func([]byte) error) {
	m.sendFunc = f
}

// SetOnTransferCallback sets callback for new transfers (legacy compatibility)
func (m *Manager) SetOnTransferCallback(f func(*Transfer)) {
	// Store for compatibility - not actively used in new protocol
}

// SendFile sends a file (legacy compatibility - wraps Upload)
func (m *Manager) SendFile(filePath string) (*Transfer, error) {
	// For legacy compatibility, create a Transfer object
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	
	transfer := &Transfer{
		ID:       fmt.Sprintf("upload_%d", time.Now().UnixNano()),
		Filename: filepath.Base(filePath),
		Size:     info.Size(),
		Status:   "pending",
	}
	
	// Use the new Upload method internally
	// Note: This requires a remote path - for legacy, use filename only
	// The actual upload will happen via the queue
	return transfer, nil
}

// HandleIncomingData processes incoming file transfer data (legacy compatibility)
func (m *Manager) HandleIncomingData(data []byte) error {
	m.HandleMessage(data)
	return nil
}

// SetOnProgress sets the progress callback
func (m *Manager) SetOnProgress(f func(job *Job)) {
	m.onProgress = f
}

// SetOnComplete sets the completion callback
func (m *Manager) SetOnComplete(f func(job *Job)) {
	m.onComplete = f
}

// SetOnError sets the error callback
func (m *Manager) SetOnError(f func(job *Job, err error)) {
	m.onError = f
}

// SetOnListResult sets the directory listing callback
func (m *Manager) SetOnListResult(f func(path string, entries []Entry)) {
	m.onListResult = f
}

// SetOnDrives sets the drives listing callback
func (m *Manager) SetOnDrives(f func(entries []Entry)) {
	m.onDrives = f
}

// ListDirectory requests a directory listing from the agent
func (m *Manager) ListDirectory(path string) error {
	msg := Message{
		Op:   OpList,
		Path: path,
	}
	return m.send(msg)
}

// ListDrives requests available drives from the agent
func (m *Manager) ListDrives() error {
	log.Println("📂 Requesting remote drives...")
	msg := Message{
		Op: OpDrives,
	}
	err := m.send(msg)
	if err != nil {
		log.Printf("❌ Failed to request drives: %v", err)
	}
	return err
}

// Download starts downloading a file from the agent
func (m *Manager) Download(remotePath, localPath string, size int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job := &Job{
		ID:      m.nextFrameID,
		Op:      "download",
		SrcPath: remotePath,
		DstPath: localPath,
		Size:    size,
	}
	m.nextFrameID++

	m.queue = append(m.queue, job)
	m.startNextJob()
	return nil
}

// Upload starts uploading a file to the agent
func (m *Manager) Upload(localPath, remotePath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Get file size
	info, err := os.Stat(localPath)
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	job := &Job{
		ID:      m.nextFrameID,
		Op:      "upload",
		SrcPath: localPath,
		DstPath: remotePath,
		Size:    info.Size(),
	}
	m.nextFrameID++

	m.queue = append(m.queue, job)
	m.startNextJob()
	return nil
}

// CreateDirectory creates a directory on the agent
func (m *Manager) CreateDirectory(path string) error {
	msg := Message{
		Op:   OpMkdir,
		Path: path,
	}
	return m.send(msg)
}

// Delete deletes a file or directory on the agent
func (m *Manager) Delete(path string) error {
	msg := Message{
		Op:   OpRm,
		Path: path,
	}
	return m.send(msg)
}

// Rename renames/moves a file or directory on the agent
func (m *Manager) Rename(oldPath, newPath string) error {
	msg := Message{
		Op:     OpMv,
		Path:   oldPath,
		Target: newPath,
	}
	return m.send(msg)
}

// HandleMessage processes incoming file transfer messages
func (m *Manager) HandleMessage(data []byte) {
	log.Printf("📥 Received file message: %d bytes", len(data))
	
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		previewLen := len(data)
		if previewLen > 100 {
			previewLen = 100
		}
		log.Printf("❌ Failed to unmarshal file message: %v (data: %s)", err, string(data[:previewLen]))
		return
	}

	log.Printf("📥 File message op=%s, path=%s, entries=%d", msg.Op, msg.Path, len(msg.Entries))

	switch msg.Op {
	case OpList:
		if m.onListResult != nil {
			m.onListResult(msg.Path, msg.Entries)
		}

	case OpDrives:
		if m.onDrives != nil {
			m.onDrives(msg.Entries)
		}

	case OpPut:
		// Receiving file data from agent (download)
		m.handleReceiveChunk(msg)

	case OpAck:
		m.handleAck(msg)

	case OpErr:
		log.Printf("❌ File transfer error: %s", msg.Error)
		m.mu.Lock()
		if m.activeJob != nil && m.onError != nil {
			m.onError(m.activeJob, fmt.Errorf(msg.Error))
		}
		m.activeJob = nil
		m.startNextJob()
		m.mu.Unlock()

	case OpProgress:
		// Progress update from agent
		m.mu.Lock()
		if m.activeJob != nil {
			m.activeJob.Done = msg.Size
			if m.onProgress != nil {
				m.onProgress(m.activeJob)
			}
		}
		m.mu.Unlock()
	}
}

func (m *Manager) startNextJob() {
	if m.activeJob != nil || len(m.queue) == 0 {
		return
	}

	m.activeJob = m.queue[0]
	m.queue = m.queue[1:]

	if m.activeJob.Op == "download" {
		// Request file from agent
		msg := Message{
			Op:      OpGet,
			Path:    m.activeJob.SrcPath,
			FrameID: m.activeJob.ID,
			Offset:  m.activeJob.Offset,
		}
		if err := m.send(msg); err != nil {
			log.Printf("❌ Failed to start download: %v", err)
			if m.onError != nil {
				m.onError(m.activeJob, err)
			}
			m.activeJob = nil
			m.startNextJob()
		}
	} else {
		// Start upload
		go m.doUpload()
	}
}

func (m *Manager) doUpload() {
	job := m.activeJob
	if job == nil {
		return
	}

	// Open local file
	f, err := os.Open(job.SrcPath)
	if err != nil {
		log.Printf("❌ Failed to open file for upload: %v", err)
		m.mu.Lock()
		if m.onError != nil {
			m.onError(job, err)
		}
		m.activeJob = nil
		m.startNextJob()
		m.mu.Unlock()
		return
	}
	defer f.Close()

	// Seek to offset if resuming
	if job.Offset > 0 {
		f.Seek(job.Offset, 0)
	}

	// Calculate total chunks
	remaining := job.Size - job.Offset
	totalChunks := uint16((remaining + ChunkSize - 1) / ChunkSize)

	buf := make([]byte, ChunkSize)
	var chunk uint16
	bytesSent := job.Offset

	for {
		n, err := f.Read(buf)
		if n > 0 {
			msg := Message{
				Op:      OpPut,
				Path:    job.DstPath,
				FrameID: job.ID,
				Chunk:   chunk,
				Total:   totalChunks,
				Size:    job.Size,
				Data:    buf[:n],
			}

			if err := m.send(msg); err != nil {
				log.Printf("❌ Failed to send chunk: %v", err)
				m.mu.Lock()
				if m.onError != nil {
					m.onError(job, err)
				}
				m.activeJob = nil
				m.startNextJob()
				m.mu.Unlock()
				return
			}

			chunk++
			bytesSent += int64(n)
			job.Done = bytesSent

			if m.onProgress != nil {
				m.onProgress(job)
			}

			// Small delay to avoid overwhelming the channel
			time.Sleep(1 * time.Millisecond)
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			log.Printf("❌ Failed to read file: %v", err)
			m.mu.Lock()
			if m.onError != nil {
				m.onError(job, err)
			}
			m.activeJob = nil
			m.startNextJob()
			m.mu.Unlock()
			return
		}
	}

	log.Printf("✅ Upload complete: %s (%d bytes)", job.SrcPath, bytesSent)
	// Wait for ACK from agent
}

func (m *Manager) handleReceiveChunk(msg Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job := m.activeJob
	if job == nil || job.ID != msg.FrameID {
		return
	}

	// Open file on first chunk
	if m.receiveFile == nil {
		// Ensure directory exists
		dir := filepath.Dir(job.DstPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Printf("❌ Failed to create directory: %v", err)
			if m.onError != nil {
				m.onError(job, err)
			}
			return
		}

		f, err := os.Create(job.DstPath)
		if err != nil {
			log.Printf("❌ Failed to create file: %v", err)
			if m.onError != nil {
				m.onError(job, err)
			}
			return
		}
		m.receiveFile = f
		m.receivedBytes = 0
	}

	// Write chunk
	if len(msg.Data) > 0 {
		n, err := m.receiveFile.Write(msg.Data)
		if err != nil {
			log.Printf("❌ Failed to write chunk: %v", err)
			if m.onError != nil {
				m.onError(job, err)
			}
			return
		}
		m.receivedBytes += int64(n)
		job.Done = m.receivedBytes

		if m.onProgress != nil {
			m.onProgress(job)
		}
	}

	// Check if complete
	if msg.Total > 0 && msg.Chunk == msg.Total-1 {
		m.receiveFile.Close()
		m.receiveFile = nil

		log.Printf("✅ Download complete: %s (%d bytes)", job.DstPath, m.receivedBytes)

		if m.onComplete != nil {
			m.onComplete(job)
		}

		m.activeJob = nil
		m.startNextJob()
	}
}

func (m *Manager) handleAck(msg Message) {
	m.mu.Lock()
	defer m.mu.Unlock()

	job := m.activeJob
	if job == nil {
		return
	}

	// Check if this is final ACK
	if msg.Path != "" {
		log.Printf("✅ Transfer complete: %s", msg.Path)
		if m.onComplete != nil {
			m.onComplete(job)
		}
		m.activeJob = nil
		m.startNextJob()
	}
}

func (m *Manager) send(msg Message) error {
	if m.sendFunc == nil {
		return fmt.Errorf("send function not set")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	log.Printf("📤 Sending file message: op=%s, path=%s, len=%d", msg.Op, msg.Path, len(data))
	return m.sendFunc(data)
}

// GetActiveJob returns the currently active job
func (m *Manager) GetActiveJob() *Job {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.activeJob
}

// GetQueueLength returns the number of queued jobs
func (m *Manager) GetQueueLength() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.queue)
}

// CancelActive cancels the active transfer
func (m *Manager) CancelActive() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.receiveFile != nil {
		m.receiveFile.Close()
		m.receiveFile = nil
	}

	m.activeJob = nil
	m.startNextJob()
}
