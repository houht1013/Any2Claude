//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// ---------------------------------------------------------------------------
//  Win32 API
// ---------------------------------------------------------------------------

var (
	shell32   = syscall.NewLazyDLL("shell32.dll")
	user32    = syscall.NewLazyDLL("user32.dll")
	kernel32  = syscall.NewLazyDLL("kernel32.dll")

	pShellNotifyIcon  = shell32.NewProc("Shell_NotifyIconW")
	pCreateWindowExW  = user32.NewProc("CreateWindowExW")
	pDefWindowProcW   = user32.NewProc("DefWindowProcW")
	pRegisterClassExW = user32.NewProc("RegisterClassExW")
	pGetMessageW      = user32.NewProc("GetMessageW")
	pTranslateMessage = user32.NewProc("TranslateMessage")
	pDispatchMessageW = user32.NewProc("DispatchMessageW")
	pPostQuitMessage  = user32.NewProc("PostQuitMessage")
	pCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	pAppendMenuW      = user32.NewProc("AppendMenuW")
	pTrackPopupMenu   = user32.NewProc("TrackPopupMenu")
	pDestroyMenu      = user32.NewProc("DestroyMenu")
	pSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	pGetCursorPos     = user32.NewProc("GetCursorPos")
	pLoadImage        = user32.NewProc("LoadImageW")
	pDestroyIcon      = user32.NewProc("DestroyIcon")
	pGetModuleHandle  = kernel32.NewProc("GetModuleHandleW")
)

const (
	NIM_ADD    = 0x00000000
	NIM_DELETE = 0x00000002
	NIF_ICON   = 0x00000002
	NIF_TIP    = 0x00000004
	NIF_MESSAGE = 0x00000001

	WM_USER       = 0x0400
	WM_TRAYICON   = WM_USER + 1
	WM_COMMAND    = 0x0111
	WM_RBUTTONUP  = 0x0205
	WM_LBUTTONDBLCLK = 0x0203

	MF_STRING    = 0x00000000
	MF_SEPARATOR = 0x00000800
	MF_GRAYED    = 0x00000001

	TPM_BOTTOMALIGN = 0x0020
	TPM_LEFTALIGN   = 0x0000

	IMAGE_ICON        = 1
	LR_LOADFROMFILE   = 0x00000010
	LR_DEFAULTSIZE    = 0x00000040

	IDM_DASHBOARD = 1001
	IDM_QUIT      = 1002
)

type WNDCLASSEX struct {
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

type NOTIFYICONDATA struct {
	CbSize           uint32
	HWnd             syscall.Handle
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            syscall.Handle
	SzTip            [128]uint16
}

type MSG struct {
	HWnd    syscall.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      POINT
}

type POINT struct {
	X, Y int32
}

var (
	trayHWND    syscall.Handle
	trayNID     NOTIFYICONDATA
	trayDashURL string
)

func wndProc(hwnd syscall.Handle, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_TRAYICON:
		switch lParam {
		case WM_RBUTTONUP:
			showContextMenu(hwnd)
		case WM_LBUTTONDBLCLK:
			openBrowser(trayDashURL)
		}
		return 0
	case WM_COMMAND:
		switch int(wParam & 0xFFFF) {
		case IDM_DASHBOARD:
			openBrowser(trayDashURL)
		case IDM_QUIT:
			removeTrayIcon()
			pPostQuitMessage.Call(0)
			os.Exit(0)
		}
		return 0
	}
	ret, _, _ := pDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

func showContextMenu(hwnd syscall.Handle) {
	hMenu, _, _ := pCreatePopupMenu.Call()

	// Title (grayed)
	title := syscall.StringToUTF16Ptr(fmt.Sprintf("%s v%s", appName, version))
	pAppendMenuW.Call(hMenu, MF_STRING|MF_GRAYED, 0, uintptr(unsafe.Pointer(title)))

	// Separator
	pAppendMenuW.Call(hMenu, MF_SEPARATOR, 0, 0)

	// Open Dashboard
	dashLabel := syscall.StringToUTF16Ptr("Open Dashboard")
	pAppendMenuW.Call(hMenu, MF_STRING, IDM_DASHBOARD, uintptr(unsafe.Pointer(dashLabel)))

	// Separator
	pAppendMenuW.Call(hMenu, MF_SEPARATOR, 0, 0)

	// Quit
	quitLabel := syscall.StringToUTF16Ptr("Quit")
	pAppendMenuW.Call(hMenu, MF_STRING, IDM_QUIT, uintptr(unsafe.Pointer(quitLabel)))

	// Get cursor pos and show menu
	var pt POINT
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForegroundWindow.Call(uintptr(hwnd))
	pTrackPopupMenu.Call(hMenu, TPM_BOTTOMALIGN|TPM_LEFTALIGN, uintptr(pt.X), uintptr(pt.Y), 0, uintptr(hwnd), 0)
	pDestroyMenu.Call(hMenu)
}

func createTrayIcon(hwnd syscall.Handle) syscall.Handle {
	// Write icon to temp file and load it
	icoData := buildIcon()
	tmpDir := os.TempDir()
	icoPath := filepath.Join(tmpDir, "any2claude_tray.ico")
	os.WriteFile(icoPath, icoData, 0644)

	icoPathPtr := syscall.StringToUTF16Ptr(icoPath)
	hIcon, _, _ := pLoadImage.Call(0, uintptr(unsafe.Pointer(icoPathPtr)), IMAGE_ICON, 16, 16, LR_LOADFROMFILE)
	return syscall.Handle(hIcon)
}

func addTrayIcon(hwnd syscall.Handle) {
	hIcon := createTrayIcon(hwnd)

	trayNID = NOTIFYICONDATA{
		CbSize:           uint32(unsafe.Sizeof(trayNID)),
		HWnd:             hwnd,
		UID:              1,
		UFlags:           NIF_ICON | NIF_TIP | NIF_MESSAGE,
		UCallbackMessage: WM_TRAYICON,
		HIcon:            hIcon,
	}
	tip := syscall.StringToUTF16(appName)
	copy(trayNID.SzTip[:], tip)

	pShellNotifyIcon.Call(NIM_ADD, uintptr(unsafe.Pointer(&trayNID)))
}

func removeTrayIcon() {
	pShellNotifyIcon.Call(NIM_DELETE, uintptr(unsafe.Pointer(&trayNID)))
}

func runSystray(proxyPort, dashPort int) {
	trayDashURL = fmt.Sprintf("http://127.0.0.1:%d", dashPort)

	hInstance, _, _ := pGetModuleHandle.Call(0)
	className := syscall.StringToUTF16Ptr("Any2ClaudeTray")

	wc := WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(WNDCLASSEX{})),
		LpfnWndProc:   syscall.NewCallback(wndProc),
		HInstance:     syscall.Handle(hInstance),
		LpszClassName: className,
	}

	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))

	hwnd, _, _ := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(appName))),
		0, 0, 0, 0, 0, 0, 0, hInstance, 0,
	)
	trayHWND = syscall.Handle(hwnd)

	addTrayIcon(trayHWND)

	// Message loop
	var msg MSG
	for {
		ret, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}
