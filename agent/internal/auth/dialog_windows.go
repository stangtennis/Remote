//go:build windows
// +build windows

package auth

import (
	"syscall"
	"unsafe"
)

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	comdlg32         = syscall.NewLazyDLL("comdlg32.dll")
	createWindowExW  = user32.NewProc("CreateWindowExW")
	defWindowProcW   = user32.NewProc("DefWindowProcW")
	destroyWindow    = user32.NewProc("DestroyWindow")
	dispatchMessageW = user32.NewProc("DispatchMessageW")
	getMessageW      = user32.NewProc("GetMessageW")
	getWindowTextW   = user32.NewProc("GetWindowTextW")
	loadCursorW      = user32.NewProc("LoadCursorW")
	loadIconW        = user32.NewProc("LoadIconW")
	postQuitMessage  = user32.NewProc("PostQuitMessage")
	registerClassExW = user32.NewProc("RegisterClassExW")
	sendMessageW     = user32.NewProc("SendMessageW")
	setFocus         = user32.NewProc("SetFocus")
	showWindow       = user32.NewProc("ShowWindow")
	translateMessage = user32.NewProc("TranslateMessage")
	updateWindow     = user32.NewProc("UpdateWindow")
	messageBoxW      = user32.NewProc("MessageBoxW")
	getModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	setWindowTextW   = user32.NewProc("SetWindowTextW")
	enableWindow     = user32.NewProc("EnableWindow")
	getDlgItem       = user32.NewProc("GetDlgItem")
	setWindowLongW   = user32.NewProc("SetWindowLongW")
	getWindowLongW   = user32.NewProc("GetWindowLongW")
)

const (
	WS_OVERLAPPED      = 0x00000000
	WS_CAPTION         = 0x00C00000
	WS_SYSMENU         = 0x00080000
	WS_VISIBLE         = 0x10000000
	WS_CHILD           = 0x40000000
	WS_TABSTOP         = 0x00010000
	WS_BORDER          = 0x00800000
	WS_EX_CLIENTEDGE   = 0x00000200
	ES_PASSWORD        = 0x0020
	ES_AUTOHSCROLL     = 0x0080
	BS_DEFPUSHBUTTON   = 0x0001
	SS_CENTER          = 0x0001
	SS_LEFT            = 0x0000
	WM_CREATE          = 0x0001
	WM_DESTROY         = 0x0002
	WM_CLOSE           = 0x0010
	WM_COMMAND         = 0x0111
	WM_SETFONT         = 0x0030
	WM_GETTEXT         = 0x000D
	WM_GETTEXTLENGTH   = 0x000E
	BN_CLICKED         = 0
	SW_SHOW            = 5
	IDC_ARROW          = 32512
	IDI_APPLICATION    = 32512
	MB_OK              = 0x00000000
	MB_ICONERROR       = 0x00000010
	MB_ICONWARNING     = 0x00000030
	MB_ICONINFORMATION = 0x00000040
	COLOR_BTNFACE      = 15
	GWL_STYLE          = -16
)

const (
	ID_EMAIL_EDIT    = 101
	ID_PASSWORD_EDIT = 102
	ID_LOGIN_BTN     = 103
	ID_STATUS_LABEL  = 104
)

type WNDCLASSEXW struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     syscall.Handle
	HIcon         syscall.Handle
	HCursor       syscall.Handle
	HbrBackground syscall.Handle
	LpszMenuName  *uint16
	LpszClassName *uint16
	HIconSm       syscall.Handle
}

type MSG struct {
	HWnd    syscall.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

// LoginDialog holds dialog state
type LoginDialog struct {
	hwnd         syscall.Handle
	emailEdit    syscall.Handle
	passwordEdit syscall.Handle
	loginBtn     syscall.Handle
	statusLabel  syscall.Handle
	config       AuthConfig
	result       *AuthResult
	email        string
	password     string
}

var currentDialog *LoginDialog

// ShowLoginDialog displays a login dialog and returns the result
func ShowLoginDialog(config AuthConfig) *AuthResult {
	dialog := &LoginDialog{
		config: config,
	}
	currentDialog = dialog

	className := syscall.StringToUTF16Ptr("RemoteAgentLoginClass")

	hInstance, _, _ := getModuleHandleW.Call(0)
	hCursor, _, _ := loadCursorW.Call(0, uintptr(IDC_ARROW))
	hIcon, _, _ := loadIconW.Call(0, uintptr(IDI_APPLICATION))

	wc := WNDCLASSEXW{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEXW{})),
		Style:         0,
		LpfnWndProc:   syscall.NewCallback(loginWndProc),
		HInstance:     syscall.Handle(hInstance),
		HCursor:       syscall.Handle(hCursor),
		HIcon:         syscall.Handle(hIcon),
		HbrBackground: syscall.Handle(COLOR_BTNFACE + 1),
		LpszClassName: className,
	}

	registerClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	// Create main window
	hwnd, _, _ := createWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("🔐 Remote Desktop Agent - Login"))),
		uintptr(WS_OVERLAPPED|WS_CAPTION|WS_SYSMENU),
		100, 100, 400, 280,
		0, 0,
		hInstance,
		0,
	)

	dialog.hwnd = syscall.Handle(hwnd)

	// Create controls
	createLoginControls(dialog, syscall.Handle(hInstance))

	showWindow.Call(hwnd, SW_SHOW)
	updateWindow.Call(hwnd)

	// Message loop
	var msg MSG
	for {
		ret, _, _ := getMessageW.Call(
			uintptr(unsafe.Pointer(&msg)),
			0, 0, 0,
		)
		if ret == 0 {
			break
		}
		translateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		dispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	return dialog.result
}

func createLoginControls(dialog *LoginDialog, hInstance syscall.Handle) {
	staticClass := syscall.StringToUTF16Ptr("STATIC")
	editClass := syscall.StringToUTF16Ptr("EDIT")
	buttonClass := syscall.StringToUTF16Ptr("BUTTON")

	// Title label
	createWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(staticClass)),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Sign in to Remote Desktop"))),
		uintptr(WS_VISIBLE|WS_CHILD|SS_CENTER),
		20, 15, 340, 25,
		uintptr(dialog.hwnd), 0,
		uintptr(hInstance), 0,
	)

	// Email label
	createWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(staticClass)),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Email:"))),
		uintptr(WS_VISIBLE|WS_CHILD|SS_LEFT),
		20, 55, 340, 20,
		uintptr(dialog.hwnd), 0,
		uintptr(hInstance), 0,
	)

	// Email edit
	emailHwnd, _, _ := createWindowExW.Call(
		WS_EX_CLIENTEDGE,
		uintptr(unsafe.Pointer(editClass)),
		0,
		uintptr(WS_VISIBLE|WS_CHILD|WS_TABSTOP|WS_BORDER|ES_AUTOHSCROLL),
		20, 75, 340, 25,
		uintptr(dialog.hwnd),
		ID_EMAIL_EDIT,
		uintptr(hInstance), 0,
	)
	dialog.emailEdit = syscall.Handle(emailHwnd)

	// Password label
	createWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(staticClass)),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Password:"))),
		uintptr(WS_VISIBLE|WS_CHILD|SS_LEFT),
		20, 110, 340, 20,
		uintptr(dialog.hwnd), 0,
		uintptr(hInstance), 0,
	)

	// Password edit
	passwordHwnd, _, _ := createWindowExW.Call(
		WS_EX_CLIENTEDGE,
		uintptr(unsafe.Pointer(editClass)),
		0,
		uintptr(WS_VISIBLE|WS_CHILD|WS_TABSTOP|WS_BORDER|ES_PASSWORD|ES_AUTOHSCROLL),
		20, 130, 340, 25,
		uintptr(dialog.hwnd),
		ID_PASSWORD_EDIT,
		uintptr(hInstance), 0,
	)
	dialog.passwordEdit = syscall.Handle(passwordHwnd)

	// Login button
	loginBtnHwnd, _, _ := createWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(buttonClass)),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Sign In"))),
		uintptr(WS_VISIBLE|WS_CHILD|WS_TABSTOP|BS_DEFPUSHBUTTON),
		120, 170, 140, 35,
		uintptr(dialog.hwnd),
		ID_LOGIN_BTN,
		uintptr(hInstance), 0,
	)
	dialog.loginBtn = syscall.Handle(loginBtnHwnd)

	// Status label
	statusHwnd, _, _ := createWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(staticClass)),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(""))),
		uintptr(WS_VISIBLE|WS_CHILD|SS_CENTER),
		20, 215, 340, 20,
		uintptr(dialog.hwnd),
		ID_STATUS_LABEL,
		uintptr(hInstance), 0,
	)
	dialog.statusLabel = syscall.Handle(statusHwnd)

	// Set focus to email field
	setFocus.Call(emailHwnd)
}

func loginWndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_COMMAND:
		cmdID := int(wParam & 0xFFFF)
		notifyCode := int((wParam >> 16) & 0xFFFF)

		if cmdID == ID_LOGIN_BTN && notifyCode == BN_CLICKED {
			handleLogin()
		}
		return 0

	case WM_CLOSE:
		destroyWindow.Call(uintptr(hwnd))
		return 0

	case WM_DESTROY:
		postQuitMessage.Call(0)
		return 0
	}

	ret, _, _ := defWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

func handleLogin() {
	if currentDialog == nil {
		return
	}

	// Get email
	emailLen, _, _ := sendMessageW.Call(
		uintptr(currentDialog.emailEdit),
		WM_GETTEXTLENGTH, 0, 0,
	)
	emailBuf := make([]uint16, emailLen+1)
	getWindowTextW.Call(
		uintptr(currentDialog.emailEdit),
		uintptr(unsafe.Pointer(&emailBuf[0])),
		uintptr(emailLen+1),
	)
	email := syscall.UTF16ToString(emailBuf)

	// Get password
	passwordLen, _, _ := sendMessageW.Call(
		uintptr(currentDialog.passwordEdit),
		WM_GETTEXTLENGTH, 0, 0,
	)
	passwordBuf := make([]uint16, passwordLen+1)
	getWindowTextW.Call(
		uintptr(currentDialog.passwordEdit),
		uintptr(unsafe.Pointer(&passwordBuf[0])),
		uintptr(passwordLen+1),
	)
	password := syscall.UTF16ToString(passwordBuf)

	if email == "" || password == "" {
		showMessage("Please enter email and password", MB_ICONWARNING)
		return
	}

	// Update status
	setStatus("Signing in...")

	// Disable button during login
	enableWindow.Call(uintptr(currentDialog.loginBtn), 0)

	// Perform login
	result, err := Login(currentDialog.config, email, password)

	// Re-enable button
	enableWindow.Call(uintptr(currentDialog.loginBtn), 1)

	if err != nil {
		setStatus("Connection error")
		showMessage("Could not connect to server:\n"+err.Error(), MB_ICONERROR)
		return
	}

	if !result.Success {
		setStatus(result.Message)
		showMessage(result.Message, MB_ICONWARNING)
		return
	}

	// Success!
	currentDialog.result = result
	setStatus("Login successful!")

	// Close dialog
	destroyWindow.Call(uintptr(currentDialog.hwnd))
}

func setStatus(text string) {
	if currentDialog != nil && currentDialog.statusLabel != 0 {
		setWindowTextW.Call(
			uintptr(currentDialog.statusLabel),
			uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))),
		)
	}
}

func showMessage(text string, flags uint32) {
	messageBoxW.Call(
		uintptr(currentDialog.hwnd),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr("Remote Desktop Agent"))),
		uintptr(flags|MB_OK),
	)
}
