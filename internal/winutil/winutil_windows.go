//go:build windows

// Package winutil wraps the small set of Win32 APIs BuildFlow needs. Everything
// here is implemented with the Go standard library (syscall + lazy DLL
// loading) so the produced executable has no third party dependencies.
package winutil

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")
)

// MessageBox style flags.
const (
	MB_OK              = 0x00000000
	MB_ICONERROR       = 0x00000010
	MB_ICONQUESTION    = 0x00000020
	MB_ICONWARNING     = 0x00000030
	MB_ICONINFORMATION = 0x00000040
	MB_YESNO           = 0x00000004
	MB_SETFOREGROUND   = 0x00010000
	MB_TOPMOST         = 0x00040000
)

const (
	swShow   = 5
	swNormal = 1
)

// MessageBox shows a native modal message box. It is used for errors that must
// be visible even when no console is attached.
func MessageBox(title, text string, style uint) {
	proc := user32.NewProc("MessageBoxW")
	t, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	m, err := syscall.UTF16PtrFromString(text)
	if err != nil {
		return
	}
	_, _, _ = proc.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(m)), uintptr(style))
}

// ErrorBox is a convenience wrapper for an error style message box.
func ErrorBox(title, text string) {
	MessageBox(title, text, MB_ICONERROR|MB_OK|MB_SETFOREGROUND|MB_TOPMOST)
}

// InfoBox is a convenience wrapper for an information style message box.
func InfoBox(title, text string) {
	MessageBox(title, text, MB_ICONINFORMATION|MB_OK|MB_SETFOREGROUND|MB_TOPMOST)
}

// ConfirmBox asks a yes/no question and reports whether the user agreed.
func ConfirmBox(title, text string) bool {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(text)
	proc := user32.NewProc("MessageBoxW")
	r, _, _ := proc.Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(m)), uintptr(MB_YESNO|MB_ICONQUESTION|MB_SETFOREGROUND|MB_TOPMOST))
	return r == 6 // IDYES
}

// OpenURL hands a URL to the user's default browser.
func OpenURL(url string) error {
	proc := shell32.NewProc("ShellExecuteW")
	u, err := syscall.UTF16PtrFromString(url)
	if err != nil {
		return err
	}
	verb, _ := syscall.UTF16PtrFromString("open")
	r, _, _ := proc.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(u)), 0, 0, swNormal)
	if r <= 32 {
		return fmt.Errorf("ShellExecuteW failed for %q (code %d)", url, r)
	}
	return nil
}

// OpenFolder reveals a folder in Explorer.
func OpenFolder(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("open folder %q: %w", abs, err)
	}
	proc := shell32.NewProc("ShellExecuteW")
	p, err := syscall.UTF16PtrFromString(abs)
	if err != nil {
		return err
	}
	verb, _ := syscall.UTF16PtrFromString("open")
	r, _, _ := proc.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(p)), 0, 0, swShow)
	if r <= 32 {
		return fmt.Errorf("ShellExecuteW failed for %q (code %d)", abs, r)
	}
	return nil
}

// OpenTextFile opens a file with its default handler (Notepad, Obsidian...).
func OpenTextFile(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, err := os.Stat(abs); err != nil {
		return fmt.Errorf("open file %q: %w", abs, err)
	}
	proc := shell32.NewProc("ShellExecuteW")
	p, _ := syscall.UTF16PtrFromString(abs)
	verb, _ := syscall.UTF16PtrFromString("open")
	r, _, _ := proc.Call(0, uintptr(unsafe.Pointer(verb)), uintptr(unsafe.Pointer(p)), 0, 0, swShow)
	if r <= 32 {
		return fmt.Errorf("ShellExecuteW failed for %q (code %d)", abs, r)
	}
	return nil
}

// AttachConsole attaches the GUI executable to its parent console so that
// `BuildFlow.exe --console` shows log output and accepts Ctrl+C.
func AttachConsole() bool {
	proc := kernel32.NewProc("AttachConsole")
	r, _, _ := proc.Call(^uintptr(0)) // ATTACH_PARENT_PROCESS
	return r != 0
}

// ---------------------------------------------------------------------------
// Single instance
// ---------------------------------------------------------------------------

// Instance is a named mutex that guarantees a single running copy.
type Instance struct {
	handle syscall.Handle
	name   string
}

var (
	globalProcessMutex sync.Mutex
	globalInstance     *Instance
)

// WaitForSingleObject return values used to probe the instance mutex.
const (
	waitAbandoned = 0x00000080
	waitObject0   = 0x00000000
	waitTimeout   = 0x00000102
	waitFailed    = 0xFFFFFFFF
)

// AcquireInstance creates a per-user named mutex and takes ownership of it.
// When another copy already owns it, alreadyRunning is true and the caller
// should hand over to that copy and exit.
//
// Ownership is probed with WaitForSingleObject rather than by reading
// GetLastError after CreateMutexW. Go's syscall trampoline clears the thread's
// last-error before returning, so ERROR_ALREADY_EXISTS is never observable that
// way: the check silently always reported "nobody else is running" and a second
// copy went on to fight for the listening port.
func AcquireInstance(name string) (*Instance, bool, error) {
	globalProcessMutex.Lock()
	defer globalProcessMutex.Unlock()
	if globalInstance != nil {
		return globalInstance, false, nil
	}
	utf16, err := syscall.UTF16PtrFromString("Local\\" + name)
	if err != nil {
		return nil, false, err
	}
	proc := kernel32.NewProc("CreateMutexW")
	h, _, callErr := proc.Call(0, 1, uintptr(unsafe.Pointer(utf16)))
	if h == 0 {
		return nil, false, fmt.Errorf("CreateMutexW failed: %w", callErr)
	}
	wait, _, _ := kernel32.NewProc("WaitForSingleObject").Call(h, 0)
	switch wait {
	case waitObject0, waitAbandoned:
		// Nobody held the lock, so this copy created the mutex and now owns
		// it. WAIT_ABANDONED means the previous owner died without closing it
		// and the lock has been handed to us, which is the same situation.
		inst := &Instance{handle: syscall.Handle(h), name: name}
		globalInstance = inst
		return inst, false, nil
	case waitTimeout:
		// CreateMutexW ignored the ownership request because the mutex already
		// existed and another copy is holding it.
		kernel32.NewProc("CloseHandle").Call(h)
		return nil, true, nil
	default:
		kernel32.NewProc("CloseHandle").Call(h)
		return nil, false, fmt.Errorf("WaitForSingleObject gave %d for the single-instance lock %q", wait, name)
	}
}

// Release drops the mutex.
func (i *Instance) Release() {
	if i == nil || i.handle == 0 {
		return
	}
	globalProcessMutex.Lock()
	defer globalProcessMutex.Unlock()
	kernel32.NewProc("CloseHandle").Call(uintptr(i.handle))
	i.handle = 0
	globalInstance = nil
}

// ---------------------------------------------------------------------------
// Start with Windows (HKCU Run key, no administrator rights needed)
// ---------------------------------------------------------------------------

const (
	runKeyPath = `Software\Microsoft\Windows\CurrentVersion\Run`
	runValue   = "BuildFlow"

	hkeyCurrentUser = 0x80000001
	keySetValue     = 0x0002
	keyQueryValue   = 0x0001
	regSz           = 1
)

// AutostartCommand returns the command line registered for sign-in start.
func AutostartCommand() string {
	exe, err := os.Executable()
	if err != nil {
		exe = os.Args[0]
	}
	// --minimized starts silently in the tray; the browser is opened from the
	// tray menu instead of automatically at sign-in.
	return fmt.Sprintf("\"%s\" --minimized", exe)
}

// AutostartEnabled reports whether the sign-in entry currently exists.
func AutostartEnabled() bool {
	proc := advapi32.NewProc("RegOpenKeyExW")
	keyPath, _ := syscall.UTF16PtrFromString(runKeyPath)
	var h syscall.Handle
	r, _, _ := proc.Call(uintptr(hkeyCurrentUser), uintptr(unsafe.Pointer(keyPath)), 0, keyQueryValue, uintptr(unsafe.Pointer(&h)))
	if r != 0 {
		return false
	}
	valueName, _ := syscall.UTF16PtrFromString(runValue)
	query := advapi32.NewProc("RegQueryValueExW")
	var typ, size uint32
	rr, _, _ := query.Call(uintptr(h), uintptr(unsafe.Pointer(valueName)), 0, uintptr(unsafe.Pointer(&typ)), 0, uintptr(unsafe.Pointer(&size)))
	advapi32.NewProc("RegCloseKey").Call(uintptr(h))
	return rr == 0 && typ == regSz
}

// SetAutostart adds or removes the per-user sign-in entry.
func SetAutostart(enabled bool) error {
	keyPath, _ := syscall.UTF16PtrFromString(runKeyPath)
	create := advapi32.NewProc("RegCreateKeyExW")
	var h syscall.Handle
	r, _, _ := create.Call(
		uintptr(hkeyCurrentUser), uintptr(unsafe.Pointer(keyPath)), 0, 0,
		0, keySetValue, 0, uintptr(unsafe.Pointer(&h)),
	)
	if r != 0 {
		return fmt.Errorf("open %s for writing (error %d): %v", runKeyPath, r, os.ErrPermission)
	}
	defer advapi32.NewProc("RegCloseKey").Call(uintptr(h))

	if !enabled {
		valueName, _ := syscall.UTF16PtrFromString(runValue)
		del := advapi32.NewProc("RegDeleteValueW")
		rr, _, _ := del.Call(uintptr(h), uintptr(unsafe.Pointer(valueName)))
		// 2 == ERROR_FILE_NOT_FOUND, which simply means "already off".
		if rr != 0 && rr != 2 {
			return fmt.Errorf("remove sign-in entry (error %d)", rr)
		}
		return nil
	}

	cmd, err := syscall.UTF16FromString(AutostartCommand())
	if err != nil {
		return err
	}
	valueName, _ := syscall.UTF16PtrFromString(runValue)
	set := advapi32.NewProc("RegSetValueExW")
	byteLen := uint32(len(cmd) * 2)
	rr, _, _ := set.Call(uintptr(h), uintptr(unsafe.Pointer(valueName)), 0, regSz,
		uintptr(unsafe.Pointer(&cmd[0])), uintptr(byteLen))
	if rr != 0 {
		return fmt.Errorf("write sign-in entry (error %d)", rr)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Native folder picker (SHBrowseForFolderW)
// ---------------------------------------------------------------------------

const (
	bifReturnOnlyFSDirs = 0x00000001
	bifNewDialogStyle   = 0x00000040
	bifEditBox          = 0x00000002
)

type browseInfoW struct {
	hwndOwner      syscall.Handle
	pidlRoot       uintptr
	pszDisplayName *uint16
	lpszTitle      *uint16
	ulFlags        uint32
	lpfn           uintptr
	lParam         uintptr
	iImage         int32
}

// PickFolder opens the native Windows folder browser and returns the selected
// absolute path. It returns an empty string when the user cancels.
func PickFolder(title string) (string, error) {
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return "", err
	}
	// SHBrowseForFolder shows a modal dialog, so give it a dedicated OS thread.
	type result struct {
		path string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- result{err: fmt.Errorf("folder picker failed: %v", r)}
			}
		}()
		bi := browseInfoW{
			lpszTitle: titlePtr,
			ulFlags:   bifReturnOnlyFSDirs | bifNewDialogStyle | bifEditBox,
		}
		browse := shell32.NewProc("SHBrowseForFolderW")
		pidl, _, _ := browse.Call(uintptr(unsafe.Pointer(&bi)))
		if pidl == 0 {
			ch <- result{} // cancelled
			return
		}
		defer shell32.NewProc("ILFree").Call(pidl)
		buf := make([]uint16, 32768)
		get := shell32.NewProc("SHGetPathFromIDListW")
		r, _, _ := get.Call(pidl, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		if r == 0 {
			ch <- result{err: errors.New("could not resolve the selected folder")}
			return
		}
		ch <- result{path: syscall.UTF16ToString(buf)}
	}()
	res := <-ch
	return res.path, res.err
}

// ---------------------------------------------------------------------------
// Clipboard
// ---------------------------------------------------------------------------

// CopyToClipboard puts UTF-16 text on the Windows clipboard.
func CopyToClipboard(text string) error {
	open := user32.NewProc("OpenClipboard")
	open.Call(0)
	defer user32.NewProc("CloseClipboard").Call()

	empty := user32.NewProc("EmptyClipboard")
	empty.Call()

	data, err := syscall.UTF16FromString(text)
	if err != nil {
		return err
	}
	alloc := kernel32.NewProc("GlobalAlloc")
	h, _, _ := alloc.Call(0x0002, uintptr(len(data)*2)) // GMEM_MOVEABLE
	if h == 0 {
		return errors.New("GlobalAlloc failed")
	}
	lock := kernel32.NewProc("GlobalLock")
	lp, _, _ := lock.Call(h)
	if lp == 0 {
		return errors.New("GlobalLock failed")
	}
	// GlobalLock returns a raw address. Holding it in a syscall.Handle variable
	// and taking that variable's address keeps the conversion a genuine pointer
	// conversion, which is what the Windows API contract expects here: the
	// returned address is the locked block.
	hBlock := syscall.Handle(lp)
	defer runtime.KeepAlive(hBlock)
	dst := unsafe.Slice((*uint16)(unsafe.Pointer(&hBlock)), len(data))
	copy(dst, data)
	kernel32.NewProc("GlobalUnlock").Call(h)

	setData := user32.NewProc("SetClipboardData")
	const cfUnicodeText = 13
	if r, _, _ := setData.Call(cfUnicodeText, h); r == 0 {
		kernel32.NewProc("GlobalFree").Call(h)
		return errors.New("SetClipboardData failed")
	}
	return nil
}

// ---------------------------------------------------------------------------
// System tray
// ---------------------------------------------------------------------------

const (
	wmDestroy          = 0x0002
	wmClose            = 0x0010
	wmQueryEndSession  = 0x0011
	wmEndSession       = 0x0016
	wmCommand          = 0x0111
	wmContextMenu      = 0x007B
	wmLButtonUp        = 0x0202
	wmRButtonUp        = 0x0205
	wmLButtonDblClk    = 0x0203
	wmApp              = 0x8000
	wmTrayCallback     = wmApp + 1
	wsOverlappedWindow = 0x00CF0000
	csHRedraw          = 0x0003
	csVRedraw          = 0x0001

	nimAdd     = 0
	nimDelete  = 2
	nifMessage = 0x00000001
	nifIcon    = 0x00000002
	nifTip     = 0x00000004

	imageIcon      = 1
	lrLoadFromFile = 0x00000010
	lrDefaultSize  = 0x00000040
	tpmRightButton = 0x0002
	tpmReturnCmd   = 0x0100
	mfString       = 0x00000000
	mfSeparator    = 0x00000800
	mfChecked      = 0x00000008
	idiApplication = 32512
)

type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     syscall.Handle
	hIcon         syscall.Handle
	hCursor       syscall.Handle
	hbrBackground syscall.Handle
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       syscall.Handle
}

type notifyIconDataW struct {
	cbSize           uint32
	hWnd             syscall.Handle
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            syscall.Handle
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         syscall.GUID
	hBalloonIcon     syscall.Handle
}

type msg struct {
	hwnd    syscall.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      struct{ x, y int32 }
}

type point struct{ x, y int32 }

// TrayItem is a single tray context-menu entry.
type TrayItem struct {
	ID      string
	Label   string
	Checked bool
	Enabled bool
}

// Tray is a hidden window plus a notification-area icon. It is what makes
// "Open BuildFlow" and "Quit BuildFlow" available after the browser tab is
// closed.
type Tray struct {
	mu        sync.Mutex
	hwnd      syscall.Handle
	iconID    uint32
	items     []TrayItem
	handlers  map[string]func()
	hidden    bool
	started   bool
	className string
}

var (
	defaultTray *Tray
	quitOnce    sync.Once
)

// On registers a callback for a tray item id.
func (t *Tray) On(id string, fn func()) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.handlers == nil {
		t.handlers = map[string]func(){}
	}
	t.handlers[id] = fn
}

// SetItems replaces the context menu contents.
func (t *Tray) SetItems(items []TrayItem) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.items = items
}

// Start creates the hidden window, adds the tray icon and runs the message
// loop. It blocks until the loop ends; call it on a dedicated goroutine.
func (t *Tray) Start() error {
	t.mu.Lock()
	if t.started {
		t.mu.Unlock()
		return errors.New("tray already started")
	}
	t.started = true
	t.iconID = 1
	className := "BuildFlowTrayWindow"
	t.className = className
	t.mu.Unlock()

	classNamePtr, err := syscall.UTF16PtrFromString(className)
	if err != nil {
		return err
	}
	instance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	icon, _, _ := user32.NewProc("LoadIconW").Call(0, idiApplication)

	wc := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		style:         csHRedraw | csVRedraw,
		lpfnWndProc:   syscall.NewCallback(t.wndProc),
		hInstance:     syscall.Handle(instance),
		hIcon:         syscall.Handle(icon),
		lpszClassName: classNamePtr,
	}
	register := user32.NewProc("RegisterClassExW")
	if r, _, _ := register.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return errors.New("RegisterClassExW failed (is BuildFlow already running?)")
	}

	title, _ := syscall.UTF16PtrFromString("BuildFlow")
	create := user32.NewProc("CreateWindowExW")
	h, _, _ := create.Call(
		0, uintptr(unsafe.Pointer(classNamePtr)), uintptr(unsafe.Pointer(title)),
		wsOverlappedWindow, 0, 0, 0, 0, 0, 0, instance, 0,
	)
	if h == 0 {
		return errors.New("CreateWindowExW failed")
	}
	hwnd := syscall.Handle(h)
	t.mu.Lock()
	t.hwnd = hwnd
	t.mu.Unlock()

	tip, _ := syscall.UTF16FromString("BuildFlow - local build workflow workbench")
	data := notifyIconDataW{
		cbSize:           uint32(unsafe.Sizeof(notifyIconDataW{})),
		hWnd:             hwnd,
		uID:              t.iconID,
		uFlags:           nifMessage | nifIcon | nifTip,
		uCallbackMessage: wmTrayCallback,
		hIcon:            syscall.Handle(icon),
	}
	copy(data.szTip[:], tip)
	notify := shell32.NewProc("Shell_NotifyIconW")
	if r, _, _ := notify.Call(nimAdd, uintptr(unsafe.Pointer(&data))); r == 0 {
		return errors.New("Shell_NotifyIconW(NIM_ADD) failed")
	}

	t.loop()
	return nil
}

func (t *Tray) loop() {
	var m msg
	get := user32.NewProc("GetMessageW")
	trans := user32.NewProc("TranslateMessage")
	disp := user32.NewProc("DispatchMessageW")
	for {
		r, _, _ := get.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		trans.Call(uintptr(unsafe.Pointer(&m)))
		disp.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func (t *Tray) wndProc(hwnd syscall.Handle, message uint32, wParam, lParam uintptr) uintptr {
	t.mu.Lock()
	cur := t.hwnd
	t.mu.Unlock()
	switch message {
	case wmTrayCallback:
		switch lParam {
		case wmLButtonDblClk:
			t.dispatch("open")
		case wmLButtonUp:
			t.dispatch("open")
		case wmRButtonUp, wmContextMenu:
			if lParam == wmContextMenu {
				// Fall through to the menu using the cursor position below.
			}
			var pt point
			user32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&pt)))
			t.showMenu(pt)
		}
		return 0
	case wmQueryEndSession:
		// Give the app a chance to persist and shut down cleanly on sign-out.
		t.dispatch("session-end")
		return 1
	case wmEndSession, wmClose, wmDestroy:
		quitOnce.Do(func() {
			user32.NewProc("PostQuitMessage").Call(0)
		})
		return 0
	}
	def := user32.NewProc("DefWindowProcW")
	r, _, _ := def.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
	_ = cur
	return r
}

func (t *Tray) dispatch(id string) {
	t.mu.Lock()
	fn := t.handlers[id]
	t.mu.Unlock()
	if fn != nil {
		fn()
	}
}

func (t *Tray) showMenu(pt point) {
	t.mu.Lock()
	hwnd := t.hwnd
	items := append([]TrayItem(nil), t.items...)
	t.mu.Unlock()
	if hwnd == 0 || len(items) == 0 {
		return
	}

	menu, _, _ := user32.NewProc("CreatePopupMenu").Call()
	defer user32.NewProc("DestroyMenu").Call(menu)
	appendMenu := user32.NewProc("AppendMenuW")
	commandByID := map[uintptr]string{}
	var next uintptr = 1000
	for _, it := range items {
		label := it.Label
		if !it.Enabled {
			label = label + "  (unavailable)"
		}
		lp, err := syscall.UTF16PtrFromString(label)
		if err != nil {
			continue
		}
		flags := uintptr(mfString)
		if it.Checked {
			flags |= mfChecked
		}
		if !it.Enabled {
			flags |= 0x00000001 // MF_GRAYED
		}
		appendMenu.Call(menu, flags, next, uintptr(unsafe.Pointer(lp)))
		commandByID[next] = it.ID
		next++
	}

	user32.NewProc("SetForegroundWindow").Call(uintptr(hwnd))
	track := user32.NewProc("TrackPopupMenuEx")
	cmd, _, _ := track.Call(menu, tpmRightButton|tpmReturnCmd, uintptr(pt.x), uintptr(pt.y), uintptr(hwnd), 0)
	user32.NewProc("PostMessageW").Call(uintptr(hwnd), 0, 0, 0)
	if id, ok := commandByID[cmd]; ok && id != "" {
		go t.dispatch(id)
	}
}

// NewTray creates the process-wide tray icon.
func NewTray() *Tray {
	defaultTray = &Tray{handlers: map[string]func(){}, hidden: true}
	return defaultTray
}

// Stop removes the tray icon.
func (t *Tray) Stop() {
	t.mu.Lock()
	hwnd := t.hwnd
	t.mu.Unlock()
	if hwnd == 0 {
		return
	}
	data := notifyIconDataW{
		cbSize: uint32(unsafe.Sizeof(notifyIconDataW{})),
		hWnd:   hwnd,
		uID:    t.iconID,
	}
	shell32.NewProc("Shell_NotifyIconW").Call(nimDelete, uintptr(unsafe.Pointer(&data)))
	user32.NewProc("DestroyWindow").Call(uintptr(hwnd))
}

// IsPortReachable reports whether something accepts TCP connections on the
// loopback port. Used to tell "BuildFlow is already running" apart from
// "something else owns the port".
func IsPortReachable(port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// ExeName returns the base name of the running executable.
func ExeName() string {
	exe, err := os.Executable()
	if err != nil {
		return "BuildFlow.exe"
	}
	return strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
}

type rtlOSVersionInfo struct {
	OSVersionInfoSize uint32
	MajorVersion      uint32
	MinorVersion      uint32
	BuildNumber       uint32
	PlatformID        uint32
	CSDVersion        [128]uint16
}

// WindowsVersion reports the true OS version using RtlGetVersion, which is not
// affected by the application compatibility shim that lies about Windows 10.
func WindowsVersion() (major, minor, build uint32, err error) {
	ntdll := syscall.NewLazyDLL("ntdll.dll")
	proc := ntdll.NewProc("RtlGetVersion")
	var info rtlOSVersionInfo
	info.OSVersionInfoSize = uint32(unsafe.Sizeof(info))
	ret, _, callErr := proc.Call(uintptr(unsafe.Pointer(&info)))
	if ret != 0 {
		return 0, 0, 0, fmt.Errorf("RtlGetVersion failed: %w", callErr)
	}
	return info.MajorVersion, info.MinorVersion, info.BuildNumber, nil
}
