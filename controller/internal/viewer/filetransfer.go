package viewer

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

// FileTransfer handles file transfer operations
type FileTransfer struct {
	viewer        *Viewer
	transferring  bool
	progress      float64
	
	// Callbacks
	onSendFile    func(filePath string) error
	onReceiveFile func(fileName string, data []byte) error
}

// NewFileTransfer creates a new file transfer handler
func NewFileTransfer(viewer *Viewer) *FileTransfer {
	return &FileTransfer{
		viewer:       viewer,
		transferring: false,
	}
}

// ShowSendDialog shows file selection dialog
func (ft *FileTransfer) ShowSendDialog() {
	dialog.ShowFileOpen(func(file fyne.URIReadCloser, err error) {
		if err != nil {
			dialog.ShowError(err, ft.viewer.window)
			return
		}
		if file == nil {
			return // User cancelled
		}
		defer file.Close()
		
		// Get file info
		filePath := file.URI().Path()
		fileName := filepath.Base(filePath)
		
		// Confirm send
		dialog.ShowConfirm(
			"Send File",
			fmt.Sprintf("Send file '%s' to remote computer?", fileName),
			func(confirmed bool) {
				if confirmed {
					ft.sendFile(filePath)
				}
			},
			ft.viewer.window,
		)
	}, ft.viewer.window)
}

// sendFile initiates file transfer
func (ft *FileTransfer) sendFile(filePath string) {
	if ft.transferring {
		dialog.ShowInformation("Transfer in Progress", "Please wait for current transfer to complete", ft.viewer.window)
		return
	}
	
	// Read file
	data, err := os.ReadFile(filePath)
	if err != nil {
		dialog.ShowError(fmt.Errorf("failed to read file: %w", err), ft.viewer.window)
		return
	}
	
	fileName := filepath.Base(filePath)
	fileSize := len(data)
	
	log.Printf("Sending file: %s (%d bytes)", fileName, fileSize)
	
	// Show progress dialog
	ft.showProgressDialog(fileName, fileSize)
	
	// Send file via callback
	if ft.onSendFile != nil {
		go func() {
			ft.transferring = true
			defer func() {
				ft.transferring = false
			}()
			
			err := ft.onSendFile(filePath)
			if err != nil {
				log.Printf("File transfer failed: %v", err)
				dialog.ShowError(fmt.Errorf("file transfer failed: %w", err), ft.viewer.window)
			} else {
				log.Printf("File transfer completed: %s", fileName)
				dialog.ShowInformation("Transfer Complete", fmt.Sprintf("File '%s' sent successfully!", fileName), ft.viewer.window)
			}
		}()
	} else {
		dialog.ShowInformation("Not Implemented", "File transfer will be implemented with WebRTC data channel", ft.viewer.window)
	}
}

// showProgressDialog shows transfer progress
func (ft *FileTransfer) showProgressDialog(fileName string, fileSize int) {
	// TODO: Show actual progress dialog with progress bar
	// This will be implemented with WebRTC data channel progress callbacks
	
	log.Printf("Transfer progress dialog shown for: %s (%s)", fileName, formatBytes(fileSize))
	
	// Placeholder for future progress dialog implementation
	// Will include:
	// - Progress bar (0-100%)
	// - File size label
	// - Transfer speed
	// - Cancel button
}

// ReceiveFile handles incoming file
func (ft *FileTransfer) ReceiveFile(fileName string, data []byte) error {
	log.Printf("Receiving file: %s (%d bytes)", fileName, len(data))
	
	// Show save dialog
	dialog.ShowFileSave(func(file fyne.URIWriteCloser, err error) {
		if err != nil {
			dialog.ShowError(err, ft.viewer.window)
			return
		}
		if file == nil {
			return // User cancelled
		}
		defer file.Close()
		
		// Write file
		_, err = file.Write(data)
		if err != nil {
			dialog.ShowError(fmt.Errorf("failed to save file: %w", err), ft.viewer.window)
			return
		}
		
		dialog.ShowInformation("File Received", fmt.Sprintf("File '%s' saved successfully!", fileName), ft.viewer.window)
	}, ft.viewer.window)
	
	return nil
}

// UpdateProgress updates transfer progress
func (ft *FileTransfer) UpdateProgress(progress float64) {
	ft.progress = progress
	log.Printf("Transfer progress: %.1f%%", progress)
}

// SetOnSendFile sets callback for sending files
func (ft *FileTransfer) SetOnSendFile(callback func(filePath string) error) {
	ft.onSendFile = callback
}

// SetOnReceiveFile sets callback for receiving files
func (ft *FileTransfer) SetOnReceiveFile(callback func(fileName string, data []byte) error) {
	ft.onReceiveFile = callback
}

// Helper functions
func formatBytes(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
