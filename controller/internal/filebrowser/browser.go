package filebrowser

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// FileInfo represents a file or directory
type FileInfo struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	IsDir   bool   `json:"is_dir"`
	ModTime int64  `json:"mod_time"`
}

// FileBrowser is a Total Commander-style dual-pane file browser
type FileBrowser struct {
	window       fyne.Window
	parentWindow fyne.Window

	// Left pane (local)
	localPath        string
	localFiles       []FileInfo
	localList        *widget.List
	localPathLabel   *widget.Label
	localDriveSelect *widget.Select

	// Right pane (remote)
	remotePath        string
	remoteFiles       []FileInfo
	remoteList        *widget.List
	remotePathLabel   *widget.Label
	remoteDriveSelect *widget.Select

	// Selected items
	localSelected  int
	remoteSelected int

	// Callbacks
	onRequestRemoteDir    func(path string)
	onSendFile            func(localPath, remotePath string) error
	onReceiveFile         func(remotePath, localPath string) error
	onRequestRemoteDrives func()

	// Status
	statusLabel  *widget.Label
	transferring bool
}

// NewFileBrowser creates a new dual-pane file browser
func NewFileBrowser(parent fyne.Window) *FileBrowser {
	fb := &FileBrowser{
		parentWindow:   parent,
		localPath:      getDefaultPath(),
		remotePath:     "C:\\",
		localSelected:  -1,
		remoteSelected: -1,
	}
	return fb
}

// Show displays the file browser window
func (fb *FileBrowser) Show() {
	// Recover from any panics to prevent app crash
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ FileBrowser panic recovered: %v", r)
		}
	}()

	// Create new window
	fb.window = fyne.CurrentApp().NewWindow("📁 File Browser")
	fb.window.Resize(fyne.NewSize(1000, 600))
	fb.window.CenterOnScreen()

	// Build UI
	content := fb.buildUI()
	fb.window.SetContent(content)

	// Load initial directories with error handling
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("❌ FileBrowser loadLocalDir panic: %v", r)
			}
		}()
		fb.loadLocalDir(fb.localPath)
	}()
	
	fb.requestRemoteDir(fb.remotePath)

	fb.window.Show()
}

// buildUI creates the dual-pane interface
func (fb *FileBrowser) buildUI() fyne.CanvasObject {
	// === LEFT PANE (Local) ===
	fb.localPathLabel = widget.NewLabel(fb.localPath)
	fb.localPathLabel.Wrapping = fyne.TextTruncate

	// Drive selector for local
	localDrives := getLocalDrives()
	fb.localDriveSelect = widget.NewSelect(localDrives, func(drive string) {
		fb.loadLocalDir(drive)
	})
	if len(localDrives) > 0 {
		fb.localDriveSelect.SetSelected(localDrives[0])
	}

	localUpBtn := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		parent := filepath.Dir(fb.localPath)
		if parent != fb.localPath {
			fb.loadLocalDir(parent)
		}
	})

	localHeader := container.NewBorder(
		nil, nil,
		container.NewHBox(fb.localDriveSelect, localUpBtn),
		nil,
		fb.localPathLabel,
	)

	fb.localList = widget.NewList(
		func() int { return len(fb.localFiles) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewIcon(theme.FileIcon()),
				widget.NewLabel("filename.txt"),
				widget.NewLabel("1.2 MB"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(fb.localFiles) {
				return
			}
			file := fb.localFiles[id]
			box := obj.(*fyne.Container)
			icon := box.Objects[0].(*widget.Icon)
			name := box.Objects[1].(*widget.Label)
			size := box.Objects[2].(*widget.Label)

			if file.IsDir {
				icon.SetResource(theme.FolderIcon())
				size.SetText("<DIR>")
			} else {
				icon.SetResource(theme.FileIcon())
				size.SetText(formatSize(file.Size))
			}
			name.SetText(file.Name)
		},
	)

	fb.localList.OnSelected = func(id widget.ListItemID) {
		fb.localSelected = id
	}

	fb.localList.OnUnselected = func(id widget.ListItemID) {
		fb.localSelected = -1
	}

	// Double-click to enter directory
	// Note: Fyne doesn't have native double-click, we'll use a button
	localOpenBtn := widget.NewButton("Open", func() {
		if fb.localSelected >= 0 && fb.localSelected < len(fb.localFiles) {
			file := fb.localFiles[fb.localSelected]
			if file.IsDir {
				fb.loadLocalDir(file.Path)
			}
		}
	})

	localPane := container.NewBorder(
		container.NewVBox(
			widget.NewLabel("💻 Local Computer"),
			localHeader,
		),
		localOpenBtn,
		nil, nil,
		fb.localList,
	)

	// === RIGHT PANE (Remote) ===
	fb.remotePathLabel = widget.NewLabel(fb.remotePath)
	fb.remotePathLabel.Wrapping = fyne.TextTruncate

	// Drive selector for remote
	fb.remoteDriveSelect = widget.NewSelect([]string{"C:\\"}, func(drive string) {
		fb.requestRemoteDir(drive)
	})
	fb.remoteDriveSelect.SetSelected("C:\\")

	remoteUpBtn := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		parent := filepath.Dir(fb.remotePath)
		if parent != fb.remotePath && parent != "." {
			fb.requestRemoteDir(parent)
		}
	})

	remoteHeader := container.NewBorder(
		nil, nil,
		container.NewHBox(fb.remoteDriveSelect, remoteUpBtn),
		nil,
		fb.remotePathLabel,
	)

	fb.remoteList = widget.NewList(
		func() int { return len(fb.remoteFiles) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewIcon(theme.FileIcon()),
				widget.NewLabel("filename.txt"),
				widget.NewLabel("1.2 MB"),
			)
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			if id >= len(fb.remoteFiles) {
				return
			}
			file := fb.remoteFiles[id]
			box := obj.(*fyne.Container)
			icon := box.Objects[0].(*widget.Icon)
			name := box.Objects[1].(*widget.Label)
			size := box.Objects[2].(*widget.Label)

			if file.IsDir {
				icon.SetResource(theme.FolderIcon())
				size.SetText("<DIR>")
			} else {
				icon.SetResource(theme.FileIcon())
				size.SetText(formatSize(file.Size))
			}
			name.SetText(file.Name)
		},
	)

	fb.remoteList.OnSelected = func(id widget.ListItemID) {
		fb.remoteSelected = id
	}

	fb.remoteList.OnUnselected = func(id widget.ListItemID) {
		fb.remoteSelected = -1
	}

	remoteOpenBtn := widget.NewButton("Open", func() {
		if fb.remoteSelected >= 0 && fb.remoteSelected < len(fb.remoteFiles) {
			file := fb.remoteFiles[fb.remoteSelected]
			if file.IsDir {
				fb.requestRemoteDir(file.Path)
			}
		}
	})

	remotePane := container.NewBorder(
		container.NewVBox(
			widget.NewLabel("🌐 Remote Computer"),
			remoteHeader,
		),
		remoteOpenBtn,
		nil, nil,
		fb.remoteList,
	)

	// === MIDDLE BUTTONS ===
	sendBtn := widget.NewButtonWithIcon("→ Send", theme.MailSendIcon(), func() {
		fb.sendSelectedFile()
	})
	sendBtn.Importance = widget.HighImportance

	receiveBtn := widget.NewButtonWithIcon("← Receive", theme.DownloadIcon(), func() {
		fb.receiveSelectedFile()
	})

	refreshBtn := widget.NewButtonWithIcon("Refresh", theme.ViewRefreshIcon(), func() {
		fb.loadLocalDir(fb.localPath)
		fb.requestRemoteDir(fb.remotePath)
	})

	middleButtons := container.NewVBox(
		widget.NewLabel(""),
		widget.NewLabel(""),
		sendBtn,
		receiveBtn,
		widget.NewSeparator(),
		refreshBtn,
	)

	// === STATUS BAR ===
	fb.statusLabel = widget.NewLabel("Ready")

	// === MAIN LAYOUT ===
	panes := container.NewHSplit(
		localPane,
		container.NewBorder(nil, nil, middleButtons, nil, remotePane),
	)
	panes.SetOffset(0.5)

	return container.NewBorder(
		nil,
		fb.statusLabel,
		nil, nil,
		panes,
	)
}

// loadLocalDir loads the local directory listing
func (fb *FileBrowser) loadLocalDir(path string) {
	// Recover from panics
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ loadLocalDir panic: %v", r)
		}
	}()

	entries, err := os.ReadDir(path)
	if err != nil {
		log.Printf("Failed to read local dir: %v", err)
		fyne.Do(func() {
			fb.setStatus("Error: " + err.Error())
		})
		return
	}

	fb.localPath = path
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

	fb.localFiles = files
	fb.localSelected = -1

	// Update UI on main thread
	fyne.Do(func() {
		if fb.localPathLabel != nil {
			fb.localPathLabel.SetText(path)
		}
		if fb.localList != nil {
			fb.localList.Refresh()
		}
		fb.setStatus(fmt.Sprintf("Local: %d items", len(fb.localFiles)))
	})
}

// requestRemoteDir requests directory listing from remote
func (fb *FileBrowser) requestRemoteDir(path string) {
	fb.remotePath = path
	fb.remotePathLabel.SetText(path)
	fb.setStatus("Loading remote directory...")

	if fb.onRequestRemoteDir != nil {
		fb.onRequestRemoteDir(path)
	}
}

// SetRemoteFiles updates the remote file list (called when data received)
func (fb *FileBrowser) SetRemoteFiles(files []FileInfo) {
	// Sort: directories first, then by name
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	fb.remoteFiles = files
	fb.remoteSelected = -1

	// Update UI on main thread
	fyne.Do(func() {
		if fb.remoteList != nil {
			fb.remoteList.Refresh()
		}
		fb.setStatus(fmt.Sprintf("Remote: %d items", len(fb.remoteFiles)))
	})
}

// SetRemoteDrives updates the remote drive list
func (fb *FileBrowser) SetRemoteDrives(drives []string) {
	fyne.Do(func() {
		if fb.remoteDriveSelect != nil {
			fb.remoteDriveSelect.Options = drives
			if len(drives) > 0 {
				fb.remoteDriveSelect.SetSelected(drives[0])
			}
			fb.remoteDriveSelect.Refresh()
		}
	})
}

// sendSelectedFile sends the selected local file to remote
func (fb *FileBrowser) sendSelectedFile() {
	if fb.localSelected < 0 || fb.localSelected >= len(fb.localFiles) {
		dialog.ShowInformation("No Selection", "Please select a file to send", fb.window)
		return
	}

	file := fb.localFiles[fb.localSelected]
	if file.IsDir {
		dialog.ShowInformation("Cannot Send", "Cannot send directories yet. Please select a file.", fb.window)
		return
	}

	remoteDest := filepath.Join(fb.remotePath, file.Name)

	dialog.ShowConfirm("Send File",
		fmt.Sprintf("Send '%s' to remote?\n\nDestination: %s", file.Name, remoteDest),
		func(ok bool) {
			if !ok {
				return
			}

			fb.setStatus("Sending: " + file.Name)
			fb.transferring = true

			if fb.onSendFile != nil {
				go func() {
					err := fb.onSendFile(file.Path, remoteDest)
					fb.transferring = false
					if err != nil {
						fb.setStatus("Error: " + err.Error())
						dialog.ShowError(err, fb.window)
					} else {
						fb.setStatus("Sent: " + file.Name)
						dialog.ShowInformation("Success", "File sent successfully!", fb.window)
						// Refresh remote dir
						fb.requestRemoteDir(fb.remotePath)
					}
				}()
			}
		}, fb.window)
}

// receiveSelectedFile receives the selected remote file
func (fb *FileBrowser) receiveSelectedFile() {
	if fb.remoteSelected < 0 || fb.remoteSelected >= len(fb.remoteFiles) {
		dialog.ShowInformation("No Selection", "Please select a file to receive", fb.window)
		return
	}

	file := fb.remoteFiles[fb.remoteSelected]
	if file.IsDir {
		dialog.ShowInformation("Cannot Receive", "Cannot receive directories yet. Please select a file.", fb.window)
		return
	}

	localDest := filepath.Join(fb.localPath, file.Name)

	dialog.ShowConfirm("Receive File",
		fmt.Sprintf("Receive '%s' from remote?\n\nDestination: %s", file.Name, localDest),
		func(ok bool) {
			if !ok {
				return
			}

			fb.setStatus("Receiving: " + file.Name)
			fb.transferring = true

			if fb.onReceiveFile != nil {
				go func() {
					err := fb.onReceiveFile(file.Path, localDest)
					fb.transferring = false
					if err != nil {
						fb.setStatus("Error: " + err.Error())
						dialog.ShowError(err, fb.window)
					} else {
						fb.setStatus("Received: " + file.Name)
						dialog.ShowInformation("Success", "File received successfully!", fb.window)
						// Refresh local dir
						fb.loadLocalDir(fb.localPath)
					}
				}()
			}
		}, fb.window)
}

func (fb *FileBrowser) setStatus(msg string) {
	if fb.statusLabel != nil {
		fb.statusLabel.SetText(msg)
	}
	log.Println("FileBrowser:", msg)
}

// SetOnRequestRemoteDir sets callback for requesting remote directory
func (fb *FileBrowser) SetOnRequestRemoteDir(callback func(path string)) {
	fb.onRequestRemoteDir = callback
}

// SetOnSendFile sets callback for sending files
func (fb *FileBrowser) SetOnSendFile(callback func(localPath, remotePath string) error) {
	fb.onSendFile = callback
}

// SetOnReceiveFile sets callback for receiving files
func (fb *FileBrowser) SetOnReceiveFile(callback func(remotePath, localPath string) error) {
	fb.onReceiveFile = callback
}

// SetOnRequestRemoteDrives sets callback for requesting remote drives
func (fb *FileBrowser) SetOnRequestRemoteDrives(callback func()) {
	fb.onRequestRemoteDrives = callback
}

// Close closes the file browser window
func (fb *FileBrowser) Close() {
	if fb.window != nil {
		fb.window.Close()
	}
}

// === Helper functions ===

func getDefaultPath() string {
	if runtime.GOOS == "windows" {
		return "C:\\"
	}
	home, _ := os.UserHomeDir()
	return home
}

func getLocalDrives() []string {
	if runtime.GOOS != "windows" {
		return []string{"/"}
	}

	drives := []string{}
	
	// Use a safer method to detect drives on Windows
	// Only check common drive letters to avoid hanging on CD-ROM/network drives
	commonDrives := []rune{'C', 'D', 'E', 'F', 'G', 'H'}
	
	for _, letter := range commonDrives {
		drive := string(letter) + ":\\"
		// Use a quick check - just see if the path exists
		if info, err := os.Stat(drive); err == nil && info.IsDir() {
			drives = append(drives, drive)
		}
	}
	
	// Fallback to C:\ if nothing found
	if len(drives) == 0 {
		drives = []string{"C:\\"}
	}
	
	return drives
}

func formatSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.1f GB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.1f MB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.1f KB", float64(size)/KB)
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// ParseDirListingResponse parses JSON directory listing from remote
func ParseDirListingResponse(data []byte) ([]FileInfo, error) {
	var response struct {
		Type  string     `json:"type"`
		Path  string     `json:"path"`
		Files []FileInfo `json:"files"`
		Error string     `json:"error"`
	}

	if err := json.Unmarshal(data, &response); err != nil {
		return nil, err
	}

	if response.Error != "" {
		return nil, fmt.Errorf(response.Error)
	}

	return response.Files, nil
}

// CreateDirListingRequest creates a JSON request for directory listing
func CreateDirListingRequest(path string) []byte {
	request := map[string]string{
		"type": "dir_list",
		"path": path,
	}
	data, _ := json.Marshal(request)
	return data
}

// CreateDrivesRequest creates a JSON request for drive listing
func CreateDrivesRequest() []byte {
	request := map[string]string{
		"type": "drives_list",
	}
	data, _ := json.Marshal(request)
	return data
}

// Unused but needed for compilation
var _ = time.Now
