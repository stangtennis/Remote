//go:build windows

package clipboard

import (
	"fmt"
	"log"
	"sync"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Raw Win32 clipboard helpers — golang.design/x/clipboard's internal
// OpenClipboard retry loop hangs forever when another process holds the
// clipboard briefly, which happens routinely in cross-session contexts.
// We do our own bounded-retry OpenClipboard so a busy clipboard skips
// the poll instead of locking up the goroutine.

const (
	cfUnicodeText = 13
	cfBitmap      = 2
	cfDIB         = 8

	gMemMoveable = 0x0002
)

var (
	user32                   = windows.NewLazySystemDLL("user32.dll")
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	procOpenClipboard        = user32.NewProc("OpenClipboard")
	procCloseClipboard       = user32.NewProc("CloseClipboard")
	procEmptyClipboard       = user32.NewProc("EmptyClipboard")
	procGetClipboardData     = user32.NewProc("GetClipboardData")
	procSetClipboardData     = user32.NewProc("SetClipboardData")
	procIsClipboardFormatAvailable = user32.NewProc("IsClipboardFormatAvailable")
	procGetClipboardSequenceNumber = user32.NewProc("GetClipboardSequenceNumber")
	procGlobalAlloc          = kernel32.NewProc("GlobalAlloc")
	procGlobalLock           = kernel32.NewProc("GlobalLock")
	procGlobalUnlock         = kernel32.NewProc("GlobalUnlock")
	procGlobalSize           = kernel32.NewProc("GlobalSize")

	clipboardWriteMu sync.Mutex
)

// rawSequence returns the current clipboard sequence number. Any time the
// clipboard contents change, this number increases. No OpenClipboard
// required — safe to call frequently.
func rawSequence() uint32 {
	ret, _, _ := procGetClipboardSequenceNumber.Call()
	return uint32(ret)
}

// rawOpen tries to open the clipboard up to maxAttempts times with sleep
// between. Returns true on success.
func rawOpen(maxAttempts int) bool {
	for i := 0; i < maxAttempts; i++ {
		ret, _, errno := procOpenClipboard.Call(0)
		log.Printf("[raw] OpenClipboard attempt %d: ret=%d errno=%v", i+1, ret, errno)
		if ret != 0 {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func rawClose() {
	procCloseClipboard.Call()
}

// rawReadText reads CF_UNICODETEXT. Returns ("", false) if no text or
// clipboard busy.
func rawReadText() (string, bool) {
	if !rawOpen(10) { // ~200ms total wait
		return "", false
	}
	defer rawClose()

	ret, _, _ := procIsClipboardFormatAvailable.Call(cfUnicodeText)
	if ret == 0 {
		return "", false
	}

	hData, _, _ := procGetClipboardData.Call(cfUnicodeText)
	if hData == 0 {
		return "", false
	}
	ptr, _, _ := procGlobalLock.Call(hData)
	if ptr == 0 {
		return "", false
	}
	defer procGlobalUnlock.Call(hData)

	// Read NUL-terminated wchar
	var u16 []uint16
	for i := 0; ; i++ {
		c := *(*uint16)(unsafe.Pointer(ptr + uintptr(i*2)))
		if c == 0 {
			break
		}
		u16 = append(u16, c)
		if i > 10*1024*1024 {
			break // 10M chars cap
		}
	}
	return string(utf16.Decode(u16)), true
}

// rawWriteText writes a UTF-16 string to clipboard as CF_UNICODETEXT.
func rawWriteText(text string) error {
	clipboardWriteMu.Lock()
	defer clipboardWriteMu.Unlock()

	if !rawOpen(20) {
		return fmt.Errorf("OpenClipboard failed after retries")
	}
	defer rawClose()

	procEmptyClipboard.Call()

	// Allocate global memory for the UTF-16 string
	utf16Text := utf16.Encode([]rune(text + "\x00")) // append NUL
	bytes := len(utf16Text) * 2
	hMem, _, _ := procGlobalAlloc.Call(gMemMoveable, uintptr(bytes))
	if hMem == 0 {
		return fmt.Errorf("GlobalAlloc failed")
	}
	ptr, _, _ := procGlobalLock.Call(hMem)
	if ptr == 0 {
		return fmt.Errorf("GlobalLock failed")
	}
	for i, c := range utf16Text {
		*(*uint16)(unsafe.Pointer(ptr + uintptr(i*2))) = c
	}
	procGlobalUnlock.Call(hMem)

	if r, _, _ := procSetClipboardData.Call(cfUnicodeText, hMem); r == 0 {
		return fmt.Errorf("SetClipboardData failed")
	}
	return nil
}
