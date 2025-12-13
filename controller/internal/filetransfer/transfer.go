package filetransfer

import (
	"encoding/base64"
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
	File         *os.File // File handle for downloads
	mu           sync.Mutex
	onProgress   func(progress int64, total int64)
	onComplete   func(success bool, err error)
}

// SetOnProgress sets the progress callback
func (t *Transfer) SetOnProgress(callback func(progress int64, total int64)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onProgress = callback
}

// SetOnComplete sets the completion callback
func (t *Transfer) SetOnComplete(callback func(success bool, err error)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onComplete = callback
}

// Manager manages file transfers
type Manager struct {
	transfers    map[string]*Transfer
	mu           sync.Mutex
	sendData     func(data []byte) error
	onTransfer   func(transfer *Transfer)
	dirCallbacks map[string]func(string, []FileInfo, error)
	onDirListing func(path string, files []FileInfo, err error)
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

		// Send chunk with base64 encoding
		chunk := map[string]interface{}{
			"type":   "file_chunk",
			"id":     transfer.ID,
			"offset": offset,
			"data":   base64.StdEncoding.EncodeToString(buffer[:n]),
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
		time.Sleep(5 * time.Millisecond)
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
	case "directory_listing":
		return m.handleDirectoryListing(message)
	case "operation_result":
		return m.handleOperationResult(message)
	}

	return nil
}

// handleTransferStart handles incoming file transfer initiation
func (m *Manager) handleTransferStart(message map[string]interface{}) error {
	id, _ := message["id"].(string)
	filename, _ := message["filename"].(string)
	size, _ := message["size"].(float64)

	// Create download directory if needed
	downloadDir := m.GetDownloadDir()
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		log.Printf("⚠️ Failed to create download dir: %v", err)
	}

	// Create file for writing
	filePath := filepath.Join(downloadDir, filename)
	file, err := os.Create(filePath)
	if err != nil {
		log.Printf("❌ Failed to create file: %v", err)
		return err
	}

	transfer := &Transfer{
		ID:        id,
		Filename:  filename,
		Size:      int64(size),
		Status:    "transferring",
		Direction: "download",
		StartTime: time.Now(),
		File:      file,
	}

	m.mu.Lock()
	m.transfers[id] = transfer
	m.mu.Unlock()

	if m.onTransfer != nil {
		m.onTransfer(transfer)
	}

	log.Printf("📥 Receiving file: %s (%d bytes) -> %s", filename, int64(size), filePath)
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

	// Decode base64 data
	data, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		return fmt.Errorf("failed to decode chunk: %w", err)
	}

	// Write to file if we have a file handle
	if transfer.File != nil {
		// Seek to offset position
		_, err := transfer.File.Seek(int64(offset), 0)
		if err != nil {
			return fmt.Errorf("failed to seek: %w", err)
		}
		
		_, err = transfer.File.Write(data)
		if err != nil {
			return fmt.Errorf("failed to write chunk: %w", err)
		}
	}

	// Update progress
	transfer.mu.Lock()
	transfer.Progress = int64(offset) + int64(len(data))
	transfer.mu.Unlock()

	if transfer.onProgress != nil {
		transfer.onProgress(transfer.Progress, transfer.Size)
	}

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

	// Close file if open
	if transfer.File != nil {
		transfer.File.Close()
		transfer.File = nil
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

// GetDownloadDir returns the download directory
func (m *Manager) GetDownloadDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, "Downloads", "RemoteDesktop")
}

// FileInfo represents a file or directory entry
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	ModTime int64  `json:"mod_time"`
}

// RequestDirectoryListing requests a directory listing from the agent
func (m *Manager) RequestDirectoryListing(path string, callback func(path string, files []FileInfo, err error)) error {
	requestID := fmt.Sprintf("dir_%d", time.Now().UnixNano())
	
	// Store callback
	m.mu.Lock()
	if m.dirCallbacks == nil {
		m.dirCallbacks = make(map[string]func(string, []FileInfo, error))
	}
	m.dirCallbacks[requestID] = callback
	m.mu.Unlock()

	request := map[string]interface{}{
		"type":       "list_directory",
		"path":       path,
		"request_id": requestID,
	}

	requestJSON, _ := json.Marshal(request)
	return m.sendData(requestJSON)
}

// RequestFile requests a file download from the agent
func (m *Manager) RequestFile(path string) error {
	request := map[string]interface{}{
		"type": "request_file",
		"path": path,
	}

	requestJSON, _ := json.Marshal(request)
	log.Printf("📥 Requesting file: %s", path)
	return m.sendData(requestJSON)
}

// CreateRemoteDirectory creates a directory on the agent
func (m *Manager) CreateRemoteDirectory(path string) error {
	request := map[string]interface{}{
		"type":       "create_directory",
		"path":       path,
		"request_id": fmt.Sprintf("mkdir_%d", time.Now().UnixNano()),
	}

	requestJSON, _ := json.Marshal(request)
	return m.sendData(requestJSON)
}

// DeleteRemoteItem deletes a file or directory on the agent
func (m *Manager) DeleteRemoteItem(path string) error {
	request := map[string]interface{}{
		"type":       "delete_item",
		"path":       path,
		"request_id": fmt.Sprintf("del_%d", time.Now().UnixNano()),
	}

	requestJSON, _ := json.Marshal(request)
	return m.sendData(requestJSON)
}

// RenameRemoteItem renames a file or directory on the agent
func (m *Manager) RenameRemoteItem(oldPath, newPath string) error {
	request := map[string]interface{}{
		"type":       "rename_item",
		"old_path":   oldPath,
		"new_path":   newPath,
		"request_id": fmt.Sprintf("ren_%d", time.Now().UnixNano()),
	}

	requestJSON, _ := json.Marshal(request)
	return m.sendData(requestJSON)
}

// handleDirectoryListing handles directory listing response from agent
func (m *Manager) handleDirectoryListing(message map[string]interface{}) error {
	requestID, _ := message["request_id"].(string)
	path, _ := message["path"].(string)
	errorMsg, _ := message["error"].(string)
	
	// Parse files array
	var files []FileInfo
	if filesRaw, ok := message["files"].([]interface{}); ok {
		for _, f := range filesRaw {
			if fileMap, ok := f.(map[string]interface{}); ok {
				fi := FileInfo{
					Name:  getString(fileMap, "name"),
					Path:  getString(fileMap, "path"),
					Size:  getInt64(fileMap, "size"),
					IsDir: getBool(fileMap, "is_dir"),
					ModTime: getInt64(fileMap, "mod_time"),
				}
				files = append(files, fi)
			}
		}
	}

	// Call callback if exists
	m.mu.Lock()
	callback, exists := m.dirCallbacks[requestID]
	if exists {
		delete(m.dirCallbacks, requestID)
	}
	m.mu.Unlock()

	var err error
	if errorMsg != "" {
		err = fmt.Errorf(errorMsg)
	}

	if callback != nil {
		callback(path, files, err)
	} else if m.onDirListing != nil {
		m.onDirListing(path, files, err)
	}

	log.Printf("📂 Directory listing: %s (%d items)", path, len(files))
	return nil
}

// handleOperationResult handles operation result from agent
func (m *Manager) handleOperationResult(message map[string]interface{}) error {
	operation, _ := message["operation"].(string)
	path, _ := message["path"].(string)
	success, _ := message["success"].(bool)
	errorMsg, _ := message["error"].(string)

	if success {
		log.Printf("✅ Operation %s succeeded: %s", operation, path)
	} else {
		log.Printf("❌ Operation %s failed: %s - %s", operation, path, errorMsg)
	}

	return nil
}

// SetOnDirListing sets callback for directory listings
func (m *Manager) SetOnDirListing(callback func(path string, files []FileInfo, err error)) {
	m.onDirListing = callback
}

// Helper functions for parsing JSON
func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getInt64(m map[string]interface{}, key string) int64 {
	if v, ok := m[key].(float64); ok {
		return int64(v)
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}
