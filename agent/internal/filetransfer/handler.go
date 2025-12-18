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
	log.Printf("📥 Agent received file message: %d bytes", len(data))
	
	var message map[string]interface{}
	if err := json.Unmarshal(data, &message); err != nil {
		log.Printf("❌ Failed to parse file message: %v", err)
		return err
	}

	// Check for "op" field (new TotalCMD protocol)
	if op, ok := message["op"].(string); ok {
		log.Printf("📥 TotalCMD op: %s", op)
		return h.handleTotalCMDMessage(op, message, data)
	}

	// Legacy "type" field
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

// handleTotalCMDMessage handles the new TotalCMD-style protocol
func (h *Handler) handleTotalCMDMessage(op string, message map[string]interface{}, rawData []byte) error {
	log.Printf("📁 TotalCMD message: op=%s", op)
	switch op {
	case "list":
		path, _ := message["path"].(string)
		return h.handleListOp(path)
	case "drives":
		log.Println("📁 Handling drives request...")
		return h.handleDrivesOp()
	case "get":
		path, _ := message["path"].(string)
		fid, _ := message["fid"].(float64)
		offset, _ := message["off"].(float64)
		return h.handleGetOp(path, uint16(fid), int64(offset))
	case "put":
		return h.handlePutOp(message)
	case "mkdir":
		path, _ := message["path"].(string)
		return h.handleMkdirOp(path)
	case "rm":
		path, _ := message["path"].(string)
		return h.handleRmOp(path)
	case "mv":
		path, _ := message["path"].(string)
		target, _ := message["target"].(string)
		return h.handleMvOp(path, target)
	}
	return nil
}

// Entry for TotalCMD protocol
type Entry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"dir"`
	Size  int64  `json:"size"`
	Mod   int64  `json:"mod"`
}

// handleListOp lists directory contents
func (h *Handler) handleListOp(path string) error {
	if path == "" {
		home, _ := os.UserHomeDir()
		path = home
	}

	log.Printf("📂 List: %s", path)

	entries, err := os.ReadDir(path)
	if err != nil {
		return h.sendTotalCMDError(err.Error())
	}

	result := make([]Entry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		result = append(result, Entry{
			Name:  e.Name(),
			Path:  filepath.Join(path, e.Name()),
			IsDir: e.IsDir(),
			Size:  info.Size(),
			Mod:   info.ModTime().Unix(),
		})
	}

	// Sort: directories first, then by name
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsDir != result[j].IsDir {
			return result[i].IsDir
		}
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})

	response := map[string]interface{}{
		"op":      "list",
		"path":    path,
		"entries": result,
	}
	return h.sendJSON(response)
}

// handleDrivesOp lists available drives
func (h *Handler) handleDrivesOp() error {
	log.Println("📂 handleDrivesOp called - listing drives...")
	
	drives := make([]Entry, 0)
	for _, letter := range "CDEFGHIJKLMNOPQRSTUVWXYZ" {
		path := string(letter) + ":\\"
		if _, err := os.Stat(path); err == nil {
			drives = append(drives, Entry{
				Name:  string(letter) + ":",
				Path:  path,
				IsDir: true,
			})
		}
	}

	response := map[string]interface{}{
		"op":      "drives",
		"entries": drives,
	}
	return h.sendJSON(response)
}

// handleGetOp sends a file to the controller
func (h *Handler) handleGetOp(path string, fid uint16, offset int64) error {
	log.Printf("📤 Get: %s (fid=%d, offset=%d)", path, fid, offset)

	f, err := os.Open(path)
	if err != nil {
		return h.sendTotalCMDError(err.Error())
	}
	defer f.Close()

	info, _ := f.Stat()
	fileSize := info.Size()

	if offset > 0 {
		f.Seek(offset, 0)
	}

	remaining := fileSize - offset
	totalChunks := uint16((remaining + 59999) / 60000) // 60KB chunks
	
	buf := make([]byte, 60000)
	var chunk uint16

	for {
		n, err := f.Read(buf)
		if n > 0 {
			// Send chunk with binary data as base64
			msg := map[string]interface{}{
				"op":   "put",
				"path": path,
				"fid":  fid,
				"c":    chunk,
				"t":    totalChunks,
				"size": fileSize,
				"data": buf[:n],
			}
			if err := h.sendJSON(msg); err != nil {
				return err
			}
			chunk++

			// Progress every 64 chunks
			if chunk%64 == 0 {
				log.Printf("📤 Progress: %d/%d chunks", chunk, totalChunks)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return h.sendTotalCMDError(err.Error())
		}
	}

	log.Printf("✅ File sent: %s (%d bytes, %d chunks)", path, fileSize, chunk)
	
	// Send ACK
	ack := map[string]interface{}{
		"op":   "ack",
		"fid":  fid,
		"path": path,
	}
	return h.sendJSON(ack)
}

// handlePutOp receives a file chunk from the controller
func (h *Handler) handlePutOp(message map[string]interface{}) error {
	path, _ := message["path"].(string)
	fid, _ := message["fid"].(float64)
	chunk, _ := message["c"].(float64)
	total, _ := message["t"].(float64)
	
	// Get data - could be []byte or base64 string
	var data []byte
	if d, ok := message["data"].([]byte); ok {
		data = d
	} else if s, ok := message["data"].(string); ok {
		var err error
		data, err = base64.StdEncoding.DecodeString(s)
		if err != nil {
			return h.sendTotalCMDError("invalid data encoding")
		}
	}

	// Create/open file
	h.mu.Lock()
	transfer, exists := h.activeTransfers[fmt.Sprintf("%d", int(fid))]
	if !exists {
		// First chunk - create file
		dir := filepath.Dir(path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			h.mu.Unlock()
			return h.sendTotalCMDError(err.Error())
		}
		
		f, err := os.Create(path)
		if err != nil {
			h.mu.Unlock()
			return h.sendTotalCMDError(err.Error())
		}
		
		transfer = &activeTransfer{
			ID:   fmt.Sprintf("%d", int(fid)),
			File: f,
		}
		h.activeTransfers[transfer.ID] = transfer
		log.Printf("📥 Receiving: %s", path)
	}
	h.mu.Unlock()

	// Write chunk
	if len(data) > 0 {
		transfer.File.Write(data)
		transfer.Received += int64(len(data))
	}

	// Check if complete
	if total > 0 && uint16(chunk) == uint16(total)-1 {
		transfer.File.Close()
		h.mu.Lock()
		delete(h.activeTransfers, transfer.ID)
		h.mu.Unlock()
		
		log.Printf("✅ File received: %s (%d bytes)", path, transfer.Received)
		
		// Send ACK
		ack := map[string]interface{}{
			"op":   "ack",
			"fid":  int(fid),
			"path": path,
		}
		return h.sendJSON(ack)
	}

	// Periodic ACK every 64 chunks
	if int(chunk)%64 == 0 && chunk > 0 {
		ack := map[string]interface{}{
			"op":  "ack",
			"fid": int(fid),
			"c":   int(chunk),
		}
		return h.sendJSON(ack)
	}

	return nil
}

// handleMkdirOp creates a directory
func (h *Handler) handleMkdirOp(path string) error {
	log.Printf("📁 Mkdir: %s", path)
	
	if err := os.MkdirAll(path, 0755); err != nil {
		return h.sendTotalCMDError(err.Error())
	}
	
	ack := map[string]interface{}{
		"op":   "ack",
		"path": path,
	}
	return h.sendJSON(ack)
}

// handleRmOp removes a file or directory
func (h *Handler) handleRmOp(path string) error {
	log.Printf("🗑️ Rm: %s", path)
	
	if err := os.RemoveAll(path); err != nil {
		return h.sendTotalCMDError(err.Error())
	}
	
	ack := map[string]interface{}{
		"op":   "ack",
		"path": path,
	}
	return h.sendJSON(ack)
}

// handleMvOp moves/renames a file or directory
func (h *Handler) handleMvOp(oldPath, newPath string) error {
	log.Printf("✏️ Mv: %s -> %s", oldPath, newPath)
	
	if err := os.Rename(oldPath, newPath); err != nil {
		return h.sendTotalCMDError(err.Error())
	}
	
	ack := map[string]interface{}{
		"op":     "ack",
		"path":   oldPath,
		"target": newPath,
	}
	return h.sendJSON(ack)
}

// sendTotalCMDError sends an error in TotalCMD protocol format
func (h *Handler) sendTotalCMDError(errMsg string) error {
	msg := map[string]interface{}{
		"op":    "err",
		"error": errMsg,
	}
	return h.sendJSON(msg)
}

// sendJSON marshals and sends a JSON message
func (h *Handler) sendJSON(msg map[string]interface{}) error {
	if h.sendData == nil {
		log.Println("❌ sendJSON: sendData callback not set!")
		return fmt.Errorf("sendData not set")
	}
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	log.Printf("📤 Sending file response: %d bytes, op=%v", len(data), msg["op"])
	return h.sendData(data)
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
