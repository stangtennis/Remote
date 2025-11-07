package filetransfer

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
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

	// Write chunk to file
	n, err := transfer.File.Write([]byte(dataStr))
	if err != nil {
		h.sendError(id, fmt.Sprintf("Failed to write chunk: %v", err))
		return err
	}

	transfer.Received += int64(n)

	// Log progress
	progress := (transfer.Received * 100) / transfer.Size
	if progress%10 == 0 {
		log.Printf("📊 Transfer progress: %d%%", progress)
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

		// Send chunk
		chunk := map[string]interface{}{
			"type":   "file_chunk",
			"id":     id,
			"offset": offset,
			"data":   string(buffer[:n]),
		}

		chunkJSON, _ := json.Marshal(chunk)
		if err := h.sendData(chunkJSON); err != nil {
			h.sendError(id, fmt.Sprintf("Failed to send chunk: %v", err))
			return err
		}

		offset += int64(n)
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
