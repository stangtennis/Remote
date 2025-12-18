package filetransfer

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// FileBrowser is a TotalCMD-style dual-pane file browser
type FileBrowser struct {
	window   fyne.Window
	manager  *Manager
	
	// Local pane
	localPath      string
	localEntries   []Entry
	localList      *widget.List
	localPathLabel *widget.Label
	localSelected  int
	
	// Remote pane
	remotePath      string
	remoteEntries   []Entry
	remoteList      *widget.List
	remotePathLabel *widget.Label
	remoteSelected  int
	
	// Status
	statusLabel  *widget.Label
	progressBar  *widget.ProgressBar
	
	// State
	activePaneLocal bool
	mu              sync.Mutex
	
	// Double-click tracking
	lastLocalClick    time.Time
	lastLocalClickID  int
	lastRemoteClick   time.Time
	lastRemoteClickID int
	
	// Callbacks
	onClose func()
}

// NewFileBrowser creates a new file browser window
func NewFileBrowser(app fyne.App, manager *Manager, onClose func()) *FileBrowser {
	fb := &FileBrowser{
		manager:         manager,
		activePaneLocal: true,
		onClose:         onClose,
		localSelected:   -1,
		remoteSelected:  -1,
	}
	
	// Create window
	fb.window = app.NewWindow("📁 Filoverførsel")
	fb.window.Resize(fyne.NewSize(1200, 700))
	
	// Set up manager callbacks
	manager.SetOnListResult(fb.handleListResult)
	manager.SetOnDrives(fb.handleDrives)
	manager.SetOnProgress(fb.handleProgress)
	manager.SetOnComplete(fb.handleComplete)
	manager.SetOnError(fb.handleError)
	
	// Build UI
	fb.buildUI()
	
	// Initialize local path
	home, _ := os.UserHomeDir()
	fb.localPath = home
	fb.refreshLocal()
	
	// Request remote drives
	fb.remotePath = ""
	manager.ListDrives()
	
	fb.window.SetOnClosed(func() {
		if fb.onClose != nil {
			fb.onClose()
		}
	})
	
	return fb
}

func (fb *FileBrowser) buildUI() {
	// Local pane
	fb.localPathLabel = widget.NewLabel(fb.localPath)
	fb.localPathLabel.TextStyle = fyne.TextStyle{Bold: true}
	
	localUpBtn := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		fb.localPath = filepath.Dir(fb.localPath)
		fb.refreshLocal()
	})
	
	localRefreshBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		fb.refreshLocal()
	})
	
	localHeader := container.NewBorder(nil, nil, 
		container.NewHBox(localUpBtn, localRefreshBtn),
		nil,
		fb.localPathLabel,
	)
	
	fb.localList = widget.NewList(
		func() int { return len(fb.localEntries) },
		func() fyne.CanvasObject {
			return fb.createListItem()
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			fb.updateListItem(obj, fb.localEntries[id], true)
		},
	)
	fb.localList.OnSelected = func(id widget.ListItemID) {
		fb.activePaneLocal = true
		fb.localSelected = int(id)
		
		// Double-click detection via rapid selection
		now := time.Now()
		if fb.lastLocalClick.Add(400*time.Millisecond).After(now) && fb.lastLocalClickID == int(id) {
			// Double-click - navigate or upload
			if id < len(fb.localEntries) {
				fb.handleLocalDoubleClick(fb.localEntries[id])
			}
		}
		fb.lastLocalClick = now
		fb.lastLocalClickID = int(id)
	}
	
	localPane := container.NewBorder(localHeader, nil, nil, nil, fb.localList)
	localCard := widget.NewCard("💻 Lokal", "", localPane)
	
	// Remote pane
	fb.remotePathLabel = widget.NewLabel("Venter på forbindelse...")
	fb.remotePathLabel.TextStyle = fyne.TextStyle{Bold: true}
	
	remoteUpBtn := widget.NewButtonWithIcon("", theme.NavigateBackIcon(), func() {
		if fb.remotePath == "" {
			return
		}
		parent := filepath.Dir(fb.remotePath)
		if parent == fb.remotePath || parent == "." {
			// Go to drives list
			fb.remotePath = ""
			fb.manager.ListDrives()
		} else {
			fb.remotePath = parent
			fb.manager.ListDirectory(fb.remotePath)
		}
	})
	
	remoteRefreshBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		if fb.remotePath == "" {
			fb.manager.ListDrives()
		} else {
			fb.manager.ListDirectory(fb.remotePath)
		}
	})
	
	remoteHeader := container.NewBorder(nil, nil,
		container.NewHBox(remoteUpBtn, remoteRefreshBtn),
		nil,
		fb.remotePathLabel,
	)
	
	fb.remoteList = widget.NewList(
		func() int { return len(fb.remoteEntries) },
		func() fyne.CanvasObject {
			return fb.createListItem()
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			fb.updateListItem(obj, fb.remoteEntries[id], false)
		},
	)
	fb.remoteList.OnSelected = func(id widget.ListItemID) {
		fb.activePaneLocal = false
		fb.remoteSelected = int(id)
		
		// Double-click detection via rapid selection
		now := time.Now()
		if fb.lastRemoteClick.Add(400*time.Millisecond).After(now) && fb.lastRemoteClickID == int(id) {
			// Double-click - navigate or download
			if id < len(fb.remoteEntries) {
				fb.handleRemoteDoubleClick(fb.remoteEntries[id])
			}
		}
		fb.lastRemoteClick = now
		fb.lastRemoteClickID = int(id)
	}
	
	remotePane := container.NewBorder(remoteHeader, nil, nil, nil, fb.remoteList)
	remoteCard := widget.NewCard("🌐 Fjerncomputer", "", remotePane)
	
	// Split view
	split := container.NewHSplit(localCard, remoteCard)
	split.SetOffset(0.5)
	
	// Toolbar
	downloadBtn := widget.NewButtonWithIcon("⬇️ Download", theme.DownloadIcon(), func() {
		fb.doDownload()
	})
	downloadBtn.Importance = widget.HighImportance
	
	uploadBtn := widget.NewButtonWithIcon("⬆️ Upload", theme.UploadIcon(), func() {
		fb.doUpload()
	})
	uploadBtn.Importance = widget.HighImportance
	
	newFolderBtn := widget.NewButtonWithIcon("📁 Ny mappe", theme.FolderNewIcon(), func() {
		fb.doNewFolder()
	})
	
	deleteBtn := widget.NewButtonWithIcon("🗑️ Slet", theme.DeleteIcon(), func() {
		fb.doDelete()
	})
	
	toolbar := container.NewHBox(
		downloadBtn,
		uploadBtn,
		widget.NewSeparator(),
		newFolderBtn,
		deleteBtn,
		layout.NewSpacer(),
	)
	
	// Status bar
	fb.statusLabel = widget.NewLabel("Klar")
	fb.progressBar = widget.NewProgressBar()
	fb.progressBar.Hide()
	
	statusBar := container.NewBorder(nil, nil, fb.statusLabel, nil, fb.progressBar)
	
	// Main layout
	content := container.NewBorder(toolbar, statusBar, nil, nil, split)
	fb.window.SetContent(content)
	
	// Keyboard shortcuts
	fb.window.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
		switch key.Name {
		case fyne.KeyF5:
			if fb.activePaneLocal {
				fb.doUpload()
			} else {
				fb.doDownload()
			}
		case fyne.KeyF7:
			fb.doNewFolder()
		case fyne.KeyDelete:
			fb.doDelete()
		case fyne.KeyF6:
			fb.doRename()
		}
	})
}

func (fb *FileBrowser) createListItem() fyne.CanvasObject {
	icon := widget.NewIcon(theme.FileIcon())
	name := widget.NewLabel("Filename")
	size := widget.NewLabel("0 KB")
	size.Alignment = fyne.TextAlignTrailing
	
	// Use Border layout: icon on left, size on right, name fills center
	return container.NewBorder(nil, nil, icon, size, name)
}

func (fb *FileBrowser) updateListItem(obj fyne.CanvasObject, entry Entry, isLocal bool) {
	c := obj.(*fyne.Container)
	// Border layout order: [center, top, bottom, left, right] -> [name, nil, nil, icon, size]
	// Objects[0] = center (name), Objects[1] = left (icon), Objects[2] = right (size)
	name := c.Objects[0].(*widget.Label)
	icon := c.Objects[1].(*widget.Icon)
	size := c.Objects[2].(*widget.Label)
	
	name.SetText(entry.Name)
	
	if entry.IsDir {
		icon.SetResource(theme.FolderIcon())
		size.SetText("")
	} else {
		icon.SetResource(theme.FileIcon())
		size.SetText(formatSize(entry.Size))
	}
}

func (fb *FileBrowser) handleLocalDoubleClick(entry Entry) {
	if entry.IsDir {
		fb.localPath = entry.Path
		fb.refreshLocal()
	} else {
		// Upload file
		fb.doUploadFile(entry.Path)
	}
}

func (fb *FileBrowser) handleRemoteDoubleClick(entry Entry) {
	if entry.IsDir {
		fb.remotePath = entry.Path
		fb.manager.ListDirectory(fb.remotePath)
	} else {
		// Download file
		localDst := filepath.Join(fb.localPath, entry.Name)
		fb.manager.Download(entry.Path, localDst, entry.Size)
		fb.statusLabel.SetText(fmt.Sprintf("Downloader: %s", entry.Name))
		fb.progressBar.Show()
		fb.progressBar.SetValue(0)
	}
}

func (fb *FileBrowser) refreshLocal() {
	fb.localPathLabel.SetText(fb.localPath)
	
	entries, err := os.ReadDir(fb.localPath)
	if err != nil {
		log.Printf("❌ Failed to read local directory: %v", err)
		return
	}
	
	fb.localEntries = make([]Entry, 0, len(entries))
	for _, e := range entries {
		info, err := e.Info()
		if err != nil {
			continue
		}
		
		// Skip hidden files on Windows
		if runtime.GOOS == "windows" && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		
		fb.localEntries = append(fb.localEntries, Entry{
			Name:  e.Name(),
			Path:  filepath.Join(fb.localPath, e.Name()),
			IsDir: e.IsDir(),
			Size:  info.Size(),
			Mod:   info.ModTime().Unix(),
		})
	}
	
	// Sort: directories first, then by name
	sort.Slice(fb.localEntries, func(i, j int) bool {
		if fb.localEntries[i].IsDir != fb.localEntries[j].IsDir {
			return fb.localEntries[i].IsDir
		}
		return strings.ToLower(fb.localEntries[i].Name) < strings.ToLower(fb.localEntries[j].Name)
	})
	
	fb.localList.Refresh()
}

func (fb *FileBrowser) handleListResult(path string, entries []Entry) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	
	fb.remotePath = path
	fb.remotePathLabel.SetText(path)
	fb.remoteEntries = entries
	
	// Sort: directories first, then by name
	sort.Slice(fb.remoteEntries, func(i, j int) bool {
		if fb.remoteEntries[i].IsDir != fb.remoteEntries[j].IsDir {
			return fb.remoteEntries[i].IsDir
		}
		return strings.ToLower(fb.remoteEntries[i].Name) < strings.ToLower(fb.remoteEntries[j].Name)
	})
	
	fb.remoteList.Refresh()
	fb.statusLabel.SetText(fmt.Sprintf("%d elementer", len(entries)))
}

func (fb *FileBrowser) handleDrives(entries []Entry) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	
	fb.remotePath = ""
	fb.remotePathLabel.SetText("Drev")
	fb.remoteEntries = entries
	fb.remoteList.Refresh()
	fb.statusLabel.SetText(fmt.Sprintf("%d drev", len(entries)))
}

func (fb *FileBrowser) handleProgress(job *Job) {
	if job.Size > 0 {
		progress := float64(job.Done) / float64(job.Size)
		fb.progressBar.SetValue(progress)
		
		pct := int(progress * 100)
		speed := formatSize(job.Done) // Simplified - should calculate actual speed
		fb.statusLabel.SetText(fmt.Sprintf("%s: %d%% (%s)", filepath.Base(job.SrcPath), pct, speed))
	}
}

func (fb *FileBrowser) handleComplete(job *Job) {
	fb.progressBar.Hide()
	fb.statusLabel.SetText(fmt.Sprintf("✅ Færdig: %s", filepath.Base(job.SrcPath)))
	
	// Refresh appropriate pane
	if job.Op == "download" {
		fb.refreshLocal()
	} else {
		fb.manager.ListDirectory(fb.remotePath)
	}
	
	// Show notification
	dialog.ShowInformation("Overførsel færdig", 
		fmt.Sprintf("%s er overført", filepath.Base(job.SrcPath)), fb.window)
}

func (fb *FileBrowser) handleError(job *Job, err error) {
	fb.progressBar.Hide()
	fb.statusLabel.SetText(fmt.Sprintf("❌ Fejl: %v", err))
	
	dialog.ShowError(err, fb.window)
}

func (fb *FileBrowser) doDownload() {
	if fb.remoteSelected < 0 || fb.remoteSelected >= len(fb.remoteEntries) {
		dialog.ShowInformation("Vælg fil", "Vælg en fil fra fjerncomputeren at downloade", fb.window)
		return
	}
	
	entry := fb.remoteEntries[fb.remoteSelected]
	if entry.IsDir {
		// Navigate into directory
		fb.remotePath = entry.Path
		fb.manager.ListDirectory(fb.remotePath)
		return
	}
	
	localDst := filepath.Join(fb.localPath, entry.Name)
	fb.manager.Download(entry.Path, localDst, entry.Size)
	fb.statusLabel.SetText(fmt.Sprintf("Downloader: %s", entry.Name))
	fb.progressBar.Show()
	fb.progressBar.SetValue(0)
}

func (fb *FileBrowser) doUpload() {
	if fb.localSelected < 0 || fb.localSelected >= len(fb.localEntries) {
		dialog.ShowInformation("Vælg fil", "Vælg en fil at uploade", fb.window)
		return
	}
	
	entry := fb.localEntries[fb.localSelected]
	if entry.IsDir {
		// Navigate into directory
		fb.localPath = entry.Path
		fb.refreshLocal()
		return
	}
	
	fb.doUploadFile(entry.Path)
}

func (fb *FileBrowser) doUploadFile(localPath string) {
	if fb.remotePath == "" {
		dialog.ShowInformation("Vælg destination", "Naviger til en mappe på fjerncomputeren først", fb.window)
		return
	}
	
	remoteDst := fb.remotePath + "\\" + filepath.Base(localPath)
	fb.manager.Upload(localPath, remoteDst)
	fb.statusLabel.SetText(fmt.Sprintf("Uploader: %s", filepath.Base(localPath)))
	fb.progressBar.Show()
	fb.progressBar.SetValue(0)
}

func (fb *FileBrowser) doNewFolder() {
	entry := widget.NewEntry()
	entry.SetPlaceHolder("Mappenavn")
	
	dialog.ShowForm("Ny mappe", "Opret", "Annuller", []*widget.FormItem{
		widget.NewFormItem("Navn", entry),
	}, func(ok bool) {
		if !ok || entry.Text == "" {
			return
		}
		
		if fb.activePaneLocal {
			newPath := filepath.Join(fb.localPath, entry.Text)
			if err := os.MkdirAll(newPath, 0755); err != nil {
				dialog.ShowError(err, fb.window)
				return
			}
			fb.refreshLocal()
		} else {
			if fb.remotePath == "" {
				dialog.ShowInformation("Fejl", "Naviger til en mappe først", fb.window)
				return
			}
			newPath := fb.remotePath + "\\" + entry.Text
			fb.manager.CreateDirectory(newPath)
			// Refresh will happen when we get ACK
			time.AfterFunc(500*time.Millisecond, func() {
				fb.manager.ListDirectory(fb.remotePath)
			})
		}
	}, fb.window)
}

func (fb *FileBrowser) doDelete() {
	var entry Entry
	var isLocal bool
	
	if fb.activePaneLocal {
		if fb.localSelected < 0 || fb.localSelected >= len(fb.localEntries) {
			return
		}
		entry = fb.localEntries[fb.localSelected]
		isLocal = true
	} else {
		if fb.remoteSelected < 0 || fb.remoteSelected >= len(fb.remoteEntries) {
			return
		}
		entry = fb.remoteEntries[fb.remoteSelected]
		isLocal = false
	}
	
	dialog.ShowConfirm("Slet", 
		fmt.Sprintf("Er du sikker på at du vil slette '%s'?", entry.Name),
		func(ok bool) {
			if !ok {
				return
			}
			
			if isLocal {
				if err := os.RemoveAll(entry.Path); err != nil {
					dialog.ShowError(err, fb.window)
					return
				}
				fb.refreshLocal()
			} else {
				fb.manager.Delete(entry.Path)
				time.AfterFunc(500*time.Millisecond, func() {
					fb.manager.ListDirectory(fb.remotePath)
				})
			}
		}, fb.window)
}

func (fb *FileBrowser) doRename() {
	var entry Entry
	var isLocal bool
	
	if fb.activePaneLocal {
		if fb.localSelected < 0 || fb.localSelected >= len(fb.localEntries) {
			return
		}
		entry = fb.localEntries[fb.localSelected]
		isLocal = true
	} else {
		if fb.remoteSelected < 0 || fb.remoteSelected >= len(fb.remoteEntries) {
			return
		}
		entry = fb.remoteEntries[fb.remoteSelected]
		isLocal = false
	}
	
	nameEntry := widget.NewEntry()
	nameEntry.SetText(entry.Name)
	
	dialog.ShowForm("Omdøb", "Omdøb", "Annuller", []*widget.FormItem{
		widget.NewFormItem("Nyt navn", nameEntry),
	}, func(ok bool) {
		if !ok || nameEntry.Text == "" || nameEntry.Text == entry.Name {
			return
		}
		
		if isLocal {
			dir := filepath.Dir(entry.Path)
			newPath := filepath.Join(dir, nameEntry.Text)
			if err := os.Rename(entry.Path, newPath); err != nil {
				dialog.ShowError(err, fb.window)
				return
			}
			fb.refreshLocal()
		} else {
			dir := filepath.Dir(entry.Path)
			newPath := dir + "\\" + nameEntry.Text
			fb.manager.Rename(entry.Path, newPath)
			time.AfterFunc(500*time.Millisecond, func() {
				fb.manager.ListDirectory(fb.remotePath)
			})
		}
	}, fb.window)
}

// Show displays the file browser window
func (fb *FileBrowser) Show() {
	fb.window.Show()
}

// Close closes the file browser window
func (fb *FileBrowser) Close() {
	fb.window.Close()
}

// Helper functions
func formatSize(bytes int64) string {
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

func ref[T any](v T) *T {
	return &v
}
