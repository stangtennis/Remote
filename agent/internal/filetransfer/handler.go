package filetransfer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Handler handles file transfers on the agent side
type Handler struct {
	activeTransfers map[string]*activeTransfer
	mu              sync.Mutex
	downloadDir     string
	sendData        func(data []byte) error
}

// activeTransfer represents an ongoing file transfer
type activeTransfer struct {
	ID       string
	Filename string
	Size     int64
	Received int64
	File     *os.File
}

// NewHandler creates a new file transfer handler
func NewHandler(downloadDir string) *Handler {
	// Create download directory if it doesn't exist
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		log.Printf("Warning: Failed to create download directory: %v", err)
	}

	return &Handler{
		activeTransfers: make(map[string]*activeTransfer),
		downloadDir:     downloadDir,
	}
}

// SetSendDataCallback sets the callback for sending data back to controller
func (h *Handler) SetSendDataCallback(callback func(data []byte) error) {
	h.sendData = callback
}

// HandleIncomingData processes incoming file transfer messages
func (h *Handler) HandleIncomingData(data []byte) error {
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
		return h.handleTransferStart(message)
	case "file_chunk":
		return h.handleFileChunk(message)
	case "file_transfer_complete":
		return h.handleTransferComplete(message)
	case "file_transfer_error":
		return h.handleTransferError(message)
	case "list_directory":
		return h.handleListDirectory(message)
	case "request_file":
		return h.handleRequestFile(message)
	case "create_directory":
		return h.handleCreateDirectory(message)
	case "delete_item":
		return h.handleDeleteItem(message)
	case "rename_item":
		return h.handleRenameItem(message)
	}

	return nil
}

// handleTransferStart initiates a new file transfer
func (h *Handler) handleTransferStart(message map[string]interface{}) error {
	id, _ := message["id"].(string)
	filename, _ := message["filename"].(string)
	size, _ := message["size"].(float64)

	log.Printf("📥 Receiving file: %s (%d bytes)", filename, int64(size))

	// Create file
	filePath := filepath.Join(h.downloadDir, filename)
	file, err := os.Create(filePath)
	if err != nil {
		h.sendError(id, fmt.Sprintf("Failed to create file: %v", err))
		return err
	}

	// Store active transfer
	h.mu.Lock()
	h.activeTransfers[id] = &activeTransfer{
		ID:       id,
		Filename: filename,
		Size:     int64(size),
		Received: 0,
		File:     file,
	}
	h.mu.Unlock()

	log.Printf("✅ File transfer started: %s", filename)
	return nil
}

// handleFileChunk processes an incoming file chunk
func (h *Handler) handleFileChunk(message map[string]interface{}) error {
	id, _ := message["id"].(string)
	dataStr, _ := message["data"].(string)

	h.mu.Lock()
	transfer, exists := h.activeTransfers[id]
	h.mu.Unlock()

	if !exists {
		return fmt.Errorf("transfer not found: %s", id)
	}

	// Decode base64 data
	data, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		h.sendError(id, fmt.Sprintf("Failed to decode chunk: %v", err))
		return err
	}

	// Write chunk to file
	n, err := transfer.File.Write(data)
	if err != nil {
		h.sendError(id, fmt.Sprintf("Failed to write chunk: %v", err))
		return err
	}

	transfer.Received += int64(n)

	// Log progress every 10%
	progress := (transfer.Received * 100) / transfer.Size
	if transfer.Size > 0 && progress%10 == 0 && transfer.Received > 0 {
		log.Printf("📊 Transfer progress: %d%% (%d/%d bytes)", progress, transfer.Received, transfer.Size)
	}

	return nil
}

// handleTransferComplete finalizes a file transfer
func (h *Handler) handleTransferComplete(message map[string]interface{}) error {
	id, _ := message["id"].(string)

	h.mu.Lock()
	transfer, exists := h.activeTransfers[id]
	if exists {
		delete(h.activeTransfers, id)
	}
	h.mu.Unlock()

	if !exists {
		return fmt.Errorf("transfer not found: %s", id)
	}

	// Close file
	if err := transfer.File.Close(); err != nil {
		log.Printf("Warning: Failed to close file: %v", err)
	}

	log.Printf("✅ File received successfully: %s", transfer.Filename)

	// Send confirmation
	confirmation := map[string]interface{}{
		"type": "file_transfer_complete",
		"id":   id,
	}

	confirmJSON, _ := json.Marshal(confirmation)
	if h.sendData != nil {
		return h.sendData(confirmJSON)
	}

	return nil
}

// handleTransferError handles transfer errors
func (h *Handler) handleTransferError(message map[string]interface{}) error {
	id, _ := message["id"].(string)
	errorMsg, _ := message["error"].(string)

	h.mu.Lock()
	transfer, exists := h.activeTransfers[id]
	if exists {
		delete(h.activeTransfers, id)
	}
	h.mu.Unlock()

	if exists {
		transfer.File.Close()
		log.Printf("❌ File transfer failed: %s - %s", transfer.Filename, errorMsg)
	}

	return nil
}

// sendError sends an error message to the controller
func (h *Handler) sendError(id, errorMsg string) {
	if h.sendData == nil {
		return
	}

	errorMessage := map[string]interface{}{
		"type":  "file_transfer_error",
		"id":    id,
		"error": errorMsg,
	}

	errorJSON, _ := json.Marshal(errorMessage)
	h.sendData(errorJSON)
}

// SendFile sends a file to the controller
func (h *Handler) SendFile(filePath string) error {
	// Open file
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Get file info
	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Generate transfer ID
	id := fmt.Sprintf("upload_%d", fileInfo.ModTime().UnixNano())
	filename := filepath.Base(filePath)

	log.Printf("📤 Sending file: %s (%d bytes)", filename, fileInfo.Size())

	// Send metadata
	metadata := map[string]interface{}{
		"type":     "file_transfer_start",
		"id":       id,
		"filename": filename,
		"size":     fileInfo.Size(),
	}

	metadataJSON, _ := json.Marshal(metadata)
	if err := h.sendData(metadataJSON); err != nil {
		return err
	}

	// Send file in chunks
	chunkSize := 64 * 1024
	buffer := make([]byte, chunkSize)
	offset := int64(0)

	for {
		n, err := file.Read(buffer)
		if err != nil && err != io.EOF {
			h.sendError(id, fmt.Sprintf("Failed to read file: %v", err))
			return err
		}

		if n == 0 {
			break
		}

		// Send chunk with base64 encoding
		chunk := map[string]interface{}{
			"type":   "file_chunk",
			"id":     id,
			"offset": offset,
			"data":   base64.StdEncoding.EncodeToString(buffer[:n]),
		}

		chunkJSON, _ := json.Marshal(chunk)
		if err := h.sendData(chunkJSON); err != nil {
			h.sendError(id, fmt.Sprintf("Failed to send chunk: %v", err))
			return err
		}

		offset += int64(n)
		
		// Log progress
		progress := (offset * 100) / fileInfo.Size()
		if progress%10 == 0 {
			log.Printf("📤 Upload progress: %d%%", progress)
		}
	}

	// Send completion
	complete := map[string]interface{}{
		"type": "file_transfer_complete",
		"id":   id,
	}

	completeJSON, _ := json.Marshal(complete)
	if err := h.sendData(completeJSON); err != nil {
		return err
	}

	log.Printf("✅ File sent successfully: %s", filename)
	return nil
}

// Cleanup closes all active transfers
func (h *Handler) Cleanup() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, transfer := range h.activeTransfers {
		if transfer.File != nil {
			transfer.File.Close()
		}
	}

	h.activeTransfers = make(map[string]*activeTransfer)
}

// FileInfo represents a file or directory entry
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	ModTime int64  `json:"mod_time"` // Unix timestamp
}

// handleListDirectory lists contents of a directory
func (h *Handler) handleListDirectory(message map[string]interface{}) error {
	path, _ := message["path"].(string)
	requestID, _ := message["request_id"].(string)

	// Default to user's home directory
	if path == "" || path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			path = "C:\\"
		} else {
			path = home
		}
	}

	// Handle drive letters for Windows
	if len(path) == 1 && strings.ToUpper(path) >= "A" && strings.ToUpper(path) <= "Z" {
		path = path + ":\\"
	}

	log.Printf("📂 Listing directory: %s", path)

	entries, err := os.ReadDir(path)
	if err != nil {
		h.sendDirectoryError(requestID, path, err.Error())
		return err
	}

	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, FileInfo{
			Name:    entry.Name(),
			Path:    filepath.Join(path, entry.Name()),
			Size:    info.Size(),
			IsDir:   entry.IsDir(),
			ModTime: info.ModTime().Unix(),
		})
	}

	// Sort: directories first, then by name
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	response := map[string]interface{}{
		"type":       "directory_listing",
		"request_id": requestID,
		"path":       path,
		"files":      files,
	}

	responseJSON, _ := json.Marshal(response)
	return h.sendData(responseJSON)
}

// handleRequestFile sends a file to the controller
func (h *Handler) handleRequestFile(message map[string]interface{}) error {
	path, _ := message["path"].(string)
	
	if path == "" {
		return fmt.Errorf("no path specified")
	}

	log.Printf("📤 File requested: %s", path)
	return h.SendFile(path)
}

// handleCreateDirectory creates a new directory
func (h *Handler) handleCreateDirectory(message map[string]interface{}) error {
	path, _ := message["path"].(string)
	requestID, _ := message["request_id"].(string)

	if path == "" {
		return fmt.Errorf("no path specified")
	}

	log.Printf("📁 Creating directory: %s", path)

	err := os.MkdirAll(path, 0755)
	
	response := map[string]interface{}{
		"type":       "operation_result",
		"request_id": requestID,
		"operation":  "create_directory",
		"path":       path,
		"success":    err == nil,
	}
	if err != nil {
		response["error"] = err.Error()
	}

	responseJSON, _ := json.Marshal(response)
	return h.sendData(responseJSON)
}

// handleDeleteItem deletes a file or directory
func (h *Handler) handleDeleteItem(message map[string]interface{}) error {
	path, _ := message["path"].(string)
	requestID, _ := message["request_id"].(string)

	if path == "" {
		return fmt.Errorf("no path specified")
	}

	log.Printf("🗑️ Deleting: %s", path)

	// Check if it's a directory
	info, err := os.Stat(path)
	if err != nil {
		h.sendOperationResult(requestID, "delete", path, false, err.Error())
		return err
	}

	if info.IsDir() {
		err = os.RemoveAll(path)
	} else {
		err = os.Remove(path)
	}

	success := err == nil
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	h.sendOperationResult(requestID, "delete", path, success, errMsg)
	return err
}

// handleRenameItem renames a file or directory
func (h *Handler) handleRenameItem(message map[string]interface{}) error {
	oldPath, _ := message["old_path"].(string)
	newPath, _ := message["new_path"].(string)
	requestID, _ := message["request_id"].(string)

	if oldPath == "" || newPath == "" {
		return fmt.Errorf("paths not specified")
	}

	log.Printf("✏️ Renaming: %s -> %s", oldPath, newPath)

	err := os.Rename(oldPath, newPath)

	success := err == nil
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	h.sendOperationResult(requestID, "rename", newPath, success, errMsg)
	return err
}

// sendDirectoryError sends a directory listing error
func (h *Handler) sendDirectoryError(requestID, path, errMsg string) {
	if h.sendData == nil {
		return
	}

	response := map[string]interface{}{
		"type":       "directory_listing",
		"request_id": requestID,
		"path":       path,
		"error":      errMsg,
		"files":      []FileInfo{},
	}

	responseJSON, _ := json.Marshal(response)
	h.sendData(responseJSON)
}

// sendOperationResult sends operation result
func (h *Handler) sendOperationResult(requestID, operation, path string, success bool, errMsg string) {
	if h.sendData == nil {
		return
	}

	response := map[string]interface{}{
		"type":       "operation_result",
		"request_id": requestID,
		"operation":  operation,
		"path":       path,
		"success":    success,
	}
	if errMsg != "" {
		response["error"] = errMsg
	}

	responseJSON, _ := json.Marshal(response)
	h.sendData(responseJSON)
}

// GetDrives returns available drives (Windows)
func (h *Handler) GetDrives() []FileInfo {
	drives := []FileInfo{}
	
	// Check common drive letters
	for _, letter := range "CDEFGHIJKLMNOPQRSTUVWXYZ" {
		path := string(letter) + ":\\"
		if _, err := os.Stat(path); err == nil {
			drives = append(drives, FileInfo{
				Name:  string(letter) + ":",
				Path:  path,
				IsDir: true,
			})
		}
	}
	
	return drives
}

// handleListDrives lists available drives
func (h *Handler) handleListDrives(requestID string) error {
	drives := h.GetDrives()
	
	response := map[string]interface{}{
		"type":       "drives_listing",
		"request_id": requestID,
		"drives":     drives,
	}

	responseJSON, _ := json.Marshal(response)
	return h.sendData(responseJSON)
}
