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

// Transfer represents a file transfer operation
type Transfer struct {
	ID           string
	Filename     string
	Size         int64
	Progress     int64
	Status       string // "pending", "transferring", "completed", "failed"
	Direction    string // "upload" or "download"
	StartTime    time.Time
	EndTime      time.Time
	Error        string
	mu           sync.Mutex
	onProgress   func(progress int64, total int64)
	onComplete   func(success bool, err error)
}

// Manager manages file transfers
type Manager struct {
	transfers    map[string]*Transfer
	mu           sync.Mutex
	sendData     func(data []byte) error
	onTransfer   func(transfer *Transfer)
}

// NewManager creates a new file transfer manager
func NewManager() *Manager {
	return &Manager{
		transfers: make(map[string]*Transfer),
	}
}

// SetSendDataCallback sets the callback for sending data via WebRTC
func (m *Manager) SetSendDataCallback(callback func(data []byte) error) {
	m.sendData = callback
}

// SetOnTransferCallback sets the callback for new transfers
func (m *Manager) SetOnTransferCallback(callback func(transfer *Transfer)) {
	m.onTransfer = callback
}

// SendFile initiates a file upload to the remote device
func (m *Manager) SendFile(filePath string) (*Transfer, error) {
	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("failed to get file info: %w", err)
	}

	// Create transfer
	transfer := &Transfer{
		ID:        fmt.Sprintf("upload_%d", time.Now().UnixNano()),
		Filename:  filepath.Base(filePath),
		Size:      fileInfo.Size(),
		Status:    "pending",
		Direction: "upload",
		StartTime: time.Now(),
	}

	m.mu.Lock()
	m.transfers[transfer.ID] = transfer
	m.mu.Unlock()

	// Send file metadata
	metadata := map[string]interface{}{
		"type":     "file_transfer_start",
		"id":       transfer.ID,
		"filename": transfer.Filename,
		"size":     transfer.Size,
	}

	metadataJSON, _ := json.Marshal(metadata)
	if err := m.sendData(metadataJSON); err != nil {
		transfer.Status = "failed"
		transfer.Error = err.Error()
		return transfer, err
	}

	// Start transfer in background
	go m.uploadFile(transfer, filePath)

	return transfer, nil
}

// uploadFile uploads a file in chunks
func (m *Manager) uploadFile(transfer *Transfer, filePath string) {
	transfer.mu.Lock()
	transfer.Status = "transferring"
	transfer.mu.Unlock()

	file, err := os.Open(filePath)
	if err != nil {
		m.failTransfer(transfer, err)
		return
	}
	defer file.Close()

	// Send file in 64KB chunks
	chunkSize := 64 * 1024
	buffer := make([]byte, chunkSize)
	offset := int64(0)

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			m.failTransfer(transfer, err)
			return
		}

		if n == 0 {
			break
		}

		// Send chunk
		chunk := map[string]interface{}{
			"type":   "file_chunk",
			"id":     transfer.ID,
			"offset": offset,
			"data":   buffer[:n],
		}

		chunkJSON, _ := json.Marshal(chunk)
		if err := m.sendData(chunkJSON); err != nil {
			m.failTransfer(transfer, err)
			return
		}

		offset += int64(n)
		transfer.mu.Lock()
		transfer.Progress = offset
		transfer.mu.Unlock()

		// Call progress callback
		if transfer.onProgress != nil {
			transfer.onProgress(offset, transfer.Size)
		}

		// Small delay to avoid overwhelming the connection
		time.Sleep(10 * time.Millisecond)
	}

	// Send completion message
	complete := map[string]interface{}{
		"type": "file_transfer_complete",
		"id":   transfer.ID,
	}

	completeJSON, _ := json.Marshal(complete)
	if err := m.sendData(completeJSON); err != nil {
		m.failTransfer(transfer, err)
		return
	}

	transfer.mu.Lock()
	transfer.Status = "completed"
	transfer.EndTime = time.Now()
	transfer.mu.Unlock()

	if transfer.onComplete != nil {
		transfer.onComplete(true, nil)
	}

	log.Printf("✅ File transfer completed: %s (%d bytes)", transfer.Filename, transfer.Size)
}

// HandleIncomingData processes incoming file transfer data
func (m *Manager) HandleIncomingData(data []byte) error {
	var message map[string]interface{}
	if err := json.Unmarshal(data, &message); err != nil {
		return err
	}

	msgType, ok := message["type"].(string)
	if !ok {
		return fmt.Errorf("invalid message type")
	}

	switch msgType {
	case "file_transfer_start":
		return m.handleTransferStart(message)
	case "file_chunk":
		return m.handleFileChunk(message)
	case "file_transfer_complete":
		return m.handleTransferComplete(message)
	case "file_transfer_error":
		return m.handleTransferError(message)
	}

	return nil
}

// handleTransferStart handles incoming file transfer initiation
func (m *Manager) handleTransferStart(message map[string]interface{}) error {
	id, _ := message["id"].(string)
	filename, _ := message["filename"].(string)
	size, _ := message["size"].(float64)

	transfer := &Transfer{
		ID:        id,
		Filename:  filename,
		Size:      int64(size),
		Status:    "transferring",
		Direction: "download",
		StartTime: time.Now(),
	}

	m.mu.Lock()
	m.transfers[id] = transfer
	m.mu.Unlock()

	if m.onTransfer != nil {
		m.onTransfer(transfer)
	}

	log.Printf("📥 Receiving file: %s (%d bytes)", filename, int64(size))
	return nil
}

// handleFileChunk handles incoming file chunks
func (m *Manager) handleFileChunk(message map[string]interface{}) error {
	id, _ := message["id"].(string)
	offset, _ := message["offset"].(float64)
	dataStr, _ := message["data"].(string)

	m.mu.Lock()
	transfer, exists := m.transfers[id]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("transfer not found: %s", id)
	}

	// Update progress
	transfer.mu.Lock()
	transfer.Progress = int64(offset) + int64(len(dataStr))
	transfer.mu.Unlock()

	if transfer.onProgress != nil {
		transfer.onProgress(transfer.Progress, transfer.Size)
	}

	// TODO: Write data to file
	// For now, just track progress

	return nil
}

// handleTransferComplete handles transfer completion
func (m *Manager) handleTransferComplete(message map[string]interface{}) error {
	id, _ := message["id"].(string)

	m.mu.Lock()
	transfer, exists := m.transfers[id]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("transfer not found: %s", id)
	}

	transfer.mu.Lock()
	transfer.Status = "completed"
	transfer.EndTime = time.Now()
	transfer.mu.Unlock()

	if transfer.onComplete != nil {
		transfer.onComplete(true, nil)
	}

	log.Printf("✅ File received: %s", transfer.Filename)
	return nil
}

// handleTransferError handles transfer errors
func (m *Manager) handleTransferError(message map[string]interface{}) error {
	id, _ := message["id"].(string)
	errorMsg, _ := message["error"].(string)

	m.mu.Lock()
	transfer, exists := m.transfers[id]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("transfer not found: %s", id)
	}

	transfer.mu.Lock()
	transfer.Status = "failed"
	transfer.Error = errorMsg
	transfer.EndTime = time.Now()
	transfer.mu.Unlock()

	if transfer.onComplete != nil {
		transfer.onComplete(false, fmt.Errorf(errorMsg))
	}

	log.Printf("❌ File transfer failed: %s - %s", transfer.Filename, errorMsg)
	return nil
}

// failTransfer marks a transfer as failed
func (m *Manager) failTransfer(transfer *Transfer, err error) {
	transfer.mu.Lock()
	transfer.Status = "failed"
	transfer.Error = err.Error()
	transfer.EndTime = time.Now()
	transfer.mu.Unlock()

	if transfer.onComplete != nil {
		transfer.onComplete(false, err)
	}

	// Send error message to remote
	errorMsg := map[string]interface{}{
		"type":  "file_transfer_error",
		"id":    transfer.ID,
		"error": err.Error(),
	}

	errorJSON, _ := json.Marshal(errorMsg)
	m.sendData(errorJSON)

	log.Printf("❌ File transfer failed: %s - %v", transfer.Filename, err)
}

// GetTransfer returns a transfer by ID
func (m *Manager) GetTransfer(id string) (*Transfer, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	transfer, exists := m.transfers[id]
	return transfer, exists
}

// GetAllTransfers returns all transfers
func (m *Manager) GetAllTransfers() []*Transfer {
	m.mu.Lock()
	defer m.mu.Unlock()

	transfers := make([]*Transfer, 0, len(m.transfers))
	for _, t := range m.transfers {
		transfers = append(transfers, t)
	}
	return transfers
}

// GetProgress returns the progress of a transfer (0-100)
func (t *Transfer) GetProgress() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.Size == 0 {
		return 0
	}
	return int((t.Progress * 100) / t.Size)
}

// GetSpeed returns the transfer speed in bytes per second
func (t *Transfer) GetSpeed() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.Status != "transferring" {
		return 0
	}

	elapsed := time.Since(t.StartTime).Seconds()
	if elapsed == 0 {
		return 0
	}

	return int64(float64(t.Progress) / elapsed)
}
