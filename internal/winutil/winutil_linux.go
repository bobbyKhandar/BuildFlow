//go:build linux

// Package winutil keeps the historical package name while providing Linux
// desktop integrations through standard desktop interfaces and helper tools.
package winutil

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"buildflow/internal/appdata"
	"github.com/godbus/dbus/v5"
)

var (
	errUnsupported = errors.New("operation is unavailable on this Linux desktop")
	lastHandoff    atomic.Bool
)

func MessageBox(title, text string, style uint) { ErrorBox(title, text) }
func ErrorBox(title, text string)               { fmt.Fprintf(os.Stderr, "%s: %s\n", title, text) }
func InfoBox(title, text string)                { fmt.Fprintf(os.Stderr, "%s: %s\n", title, text) }
func ConfirmBox(title, text string) bool        { return false }
func AttachConsole() bool                       { return true }

func OpenURL(target string) error { return openWithXDG(target) }
func OpenFolder(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("open folder: path must be absolute: %q", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("open folder %q: %w", path, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("open folder %q: path is not a directory", path)
	}
	return openWithXDG(path)
}
func OpenTextFile(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("open file: path must be absolute: %q", path)
	}
	if _, err := os.Stat(path); err != nil {
		return fmt.Errorf("open file %q: %w", path, err)
	}
	return openWithXDG(path)
}

func openWithXDG(target string) error {
	tool, err := exec.LookPath("xdg-open")
	if err != nil {
		return fmt.Errorf("cannot open %q: xdg-open is not installed or not on PATH", target)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, tool, target)
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return fmt.Errorf("open output sink for xdg-open: %w", err)
	}
	cmd.Stdout, cmd.Stderr = devNull, devNull
	runErr := cmd.Run()
	_ = devNull.Close()
	if runErr != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return fmt.Errorf("xdg-open %q timed out after 15 seconds", target)
		}
		return fmt.Errorf("xdg-open %q failed: %w", target, runErr)
	}
	return nil
}

func CopyToClipboard(text string) error {
	wayland := strings.TrimSpace(os.Getenv("WAYLAND_DISPLAY")) != "" || strings.EqualFold(os.Getenv("XDG_SESSION_TYPE"), "wayland")
	var candidates [][]string
	if wayland {
		candidates = [][]string{{"wl-copy"}}
	} else if strings.TrimSpace(os.Getenv("DISPLAY")) != "" {
		candidates = [][]string{{"xclip", "-selection", "clipboard"}, {"xsel", "--clipboard", "--input"}}
	} else {
		return errors.New("clipboard is unavailable: neither a Wayland nor X11 graphical session was detected")
	}
	missing := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		tool, err := exec.LookPath(candidate[0])
		if err != nil {
			missing = append(missing, candidate[0])
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		cmd := exec.CommandContext(ctx, tool, candidate[1:]...)
		cmd.Stdin = strings.NewReader(text)
		// Clipboard helpers can daemonize. Direct them to /dev/null instead of
		// Go's output-copy pipes, which a daemon child could keep open.
		devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			cancel()
			return fmt.Errorf("open output sink for %s: %w", candidate[0], err)
		}
		cmd.Stdout, cmd.Stderr = devNull, devNull
		runErr := cmd.Run()
		ctxErr := ctx.Err()
		cancel()
		_ = devNull.Close()
		if runErr != nil {
			if errors.Is(ctxErr, context.DeadlineExceeded) {
				return fmt.Errorf("%s timed out while copying to the clipboard", candidate[0])
			}
			return fmt.Errorf("%s could not copy to the clipboard: %w", candidate[0], runErr)
		}
		return nil
	}
	return fmt.Errorf("clipboard helper unavailable; install %s", strings.Join(missing, " or "))
}

// Instance owns a process lock and its private Unix-domain handoff socket.
type Instance struct {
	lock       *os.File
	listener   *net.UnixListener
	socketPath string
	stop       chan struct{}
	mu         sync.Mutex
	handler    func()
	pending    int
	released   bool
}

func runtimeDirectory() (string, error) {
	if base := strings.TrimSpace(os.Getenv("XDG_RUNTIME_DIR")); base != "" && filepath.IsAbs(base) {
		if info, err := os.Lstat(base); err == nil && info.IsDir() {
			if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) == os.Getuid() && info.Mode().Perm()&0o077 == 0 {
				path := filepath.Join(base, "buildflow")
				if err := privateDirectory(path); err == nil {
					return path, nil
				}
			}
		}
	}
	state, err := appdata.StateDir()
	if err != nil {
		return "", err
	}
	path := filepath.Join(state, "runtime")
	if err := privateDirectory(path); err != nil {
		return "", fmt.Errorf("create private runtime directory %q: %w", path, err)
	}
	return path, nil
}

func privateDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%q is not a real directory", path)
	}
	if st, ok := info.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != os.Getuid() {
		return fmt.Errorf("%q is not owned by the current user", path)
	}
	return nil
}

func AcquireInstance(identity string) (*Instance, bool, error) {
	lastHandoff.Store(false)
	dir, err := runtimeDirectory()
	if err != nil {
		return nil, false, err
	}
	if identity == "" {
		identity = "default"
	}
	lockPath := filepath.Join(dir, identity+".lock")
	socketPath := filepath.Join(dir, identity+".sock")
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, fmt.Errorf("open instance lock %q: %w", lockPath, err)
	}
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = lock.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			if sendHandoff(socketPath, lockPath, 4*time.Second) {
				lastHandoff.Store(true)
			}
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("lock %q: %w", lockPath, err)
	}
	// The lock is held before stale socket cleanup, so no healthy owner can be
	// unlinked by a concurrent launch. Flock releases automatically on crash.
	if info, e := os.Lstat(socketPath); e == nil {
		if info.Mode()&os.ModeSocket != 0 {
			_ = os.Remove(socketPath)
		} else {
			_ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
			_ = lock.Close()
			return nil, false, fmt.Errorf("refusing to replace non-socket instance path %q", socketPath)
		}
	}
	if err := lock.Truncate(0); err != nil {
		_ = lock.Close()
		return nil, false, err
	}
	if _, err := lock.Seek(0, 0); err != nil {
		_ = lock.Close()
		return nil, false, err
	}
	if _, err := fmt.Fprintf(lock, "%d\n", os.Getpid()); err != nil {
		_ = lock.Close()
		return nil, false, err
	}
	if err := lock.Sync(); err != nil {
		_ = lock.Close()
		return nil, false, err
	}
	addr, err := net.ResolveUnixAddr("unix", socketPath)
	if err != nil {
		_ = lock.Close()
		return nil, false, err
	}
	listener, err := net.ListenUnix("unix", addr)
	if err != nil {
		_ = lock.Close()
		return nil, false, fmt.Errorf("create instance handoff socket: %w", err)
	}
	if err := os.Chmod(socketPath, 0o600); err != nil {
		_ = listener.Close()
		_ = lock.Close()
		_ = os.Remove(socketPath)
		return nil, false, err
	}
	i := &Instance{lock: lock, listener: listener, socketPath: socketPath, stop: make(chan struct{})}
	go i.acceptHandoffs()
	return i, false, nil
}

func sendHandoff(socketPath, lockPath string, within time.Duration) bool {
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		pid := lockPID(lockPath)
		if pid > 0 {
			conn, err := net.DialTimeout("unix", socketPath, 150*time.Millisecond)
			if err == nil {
				_ = conn.SetDeadline(time.Now().Add(500 * time.Millisecond))
				_, writeErr := fmt.Fprintf(conn, "open %d\n", pid)
				_ = conn.Close()
				if writeErr == nil {
					return true
				}
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

func lockPID(path string) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(raw)))
	return pid
}

func (i *Instance) acceptHandoffs() {
	for {
		conn, err := i.listener.AcceptUnix()
		if err != nil {
			select {
			case <-i.stop:
				return
			default:
			}
			continue
		}
		_ = conn.SetDeadline(time.Now().Add(1 * time.Second))
		line, _ := bufio.NewReader(conn).ReadString('\n')
		_ = conn.Close()
		fields := strings.Fields(line)
		if len(fields) != 2 || fields[0] != "open" {
			continue
		}
		pid, err := strconv.Atoi(fields[1])
		if err != nil || pid != os.Getpid() || lockPID(i.lock.Name()) != os.Getpid() {
			continue
		}
		i.mu.Lock()
		if i.handler != nil {
			handler := i.handler
			i.mu.Unlock()
			go handler()
		} else {
			i.pending++
			i.mu.Unlock()
		}
	}
}

func (i *Instance) SetHandoffHandler(handler func()) {
	if i == nil {
		return
	}
	i.mu.Lock()
	i.handler = handler
	pending := i.pending
	i.pending = 0
	i.mu.Unlock()
	for n := 0; n < pending; n++ {
		go handler()
	}
}

func (i *Instance) Release() {
	if i == nil {
		return
	}
	i.mu.Lock()
	if i.released {
		i.mu.Unlock()
		return
	}
	i.released = true
	close(i.stop)
	i.mu.Unlock()
	_ = i.listener.Close()
	_ = os.Remove(i.socketPath)
	_ = syscall.Flock(int(i.lock.Fd()), syscall.LOCK_UN)
	_ = i.lock.Close()
}

func HandoffOnAcquire() bool { return lastHandoff.Load() }
func IsPortReachable(port int) bool {
	c, err := net.DialTimeout("tcp4", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 250*time.Millisecond)
	if err != nil {
		return false
	}
	_ = c.Close()
	return true
}
func ExeName() string { return "BuildFlow" }

func AutostartLocation() string { path, _ := autostartPath(); return path }
func AutostartCommand() string {
	exe, err := executablePath()
	if err != nil {
		return ""
	}
	quoted, err := desktopQuote(exe)
	if err != nil {
		return ""
	}
	return quoted + " --minimized"
}
func AutostartEnabled() bool {
	path, err := autostartPath()
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}
func SetAutostart(enabled bool) error {
	path, err := autostartPath()
	if err != nil {
		return err
	}
	if !enabled {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove login startup entry %q: %w", path, err)
		}
		return nil
	}
	exe, err := executablePath()
	if err != nil {
		return err
	}
	command, err := desktopQuote(exe)
	if err != nil {
		return err
	}
	if strings.TrimSpace(os.Getenv(appdata.EnvOverride)) != "" {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("create isolated autostart directory: %w", err)
		}
		if err := os.Chmod(filepath.Dir(path), 0o700); err != nil {
			return fmt.Errorf("secure isolated autostart directory: %w", err)
		}
	} else if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create autostart directory: %w", err)
	}
	body := "[Desktop Entry]\nType=Application\nName=BuildFlow\nComment=Start BuildFlow at login\nExec=" + command + " --minimized\nTerminal=false\nX-GNOME-Autostart-enabled=true\n"
	tmp, err := os.CreateTemp(filepath.Dir(path), ".buildflow-*.desktop")
	if err != nil {
		return fmt.Errorf("create login startup entry: %w", err)
	}
	tmpName := tmp.Name()
	_, writeErr := tmp.WriteString(body)
	if writeErr == nil {
		writeErr = tmp.Sync()
	}
	closeErr := tmp.Close()
	if writeErr != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("write login startup entry: %w", writeErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmpName)
		return closeErr
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		_ = os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("install login startup entry: %w", err)
	}
	return nil
}

func autostartPath() (string, error) {
	if override := strings.TrimSpace(os.Getenv(appdata.EnvOverride)); override != "" {
		root, err := filepath.Abs(override)
		if err != nil {
			return "", fmt.Errorf("resolve %s: %w", appdata.EnvOverride, err)
		}
		return filepath.Join(root, "autostart", "buildflow.desktop"), nil
	}
	config, err := appdata.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(config), "autostart", "buildflow.desktop"), nil
}

func executablePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate BuildFlow executable: %w", err)
	}
	abs, err := filepath.Abs(exe)
	if err != nil {
		return "", fmt.Errorf("resolve BuildFlow executable: %w", err)
	}
	return filepath.Clean(abs), nil
}

func desktopQuote(s string) (string, error) {
	if strings.ContainsAny(s, "\r\n\x00") {
		return "", fmt.Errorf("executable path contains a character unsupported by desktop entries")
	}
	s = strings.ReplaceAll(s, "%", "%%")
	backtickEscape := "\\" + "`"
	s = strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "$", "\\$", "`", backtickEscape).Replace(s)
	return `"` + s + `"`, nil
}

func PickFolder(title string) (string, error) { return pickFolderPortal(title) }

type TrayItem struct {
	ID      string
	Label   string
	Checked bool
	Enabled bool
}
type Tray struct {
	mu       sync.Mutex
	items    []TrayItem
	handlers map[string]func()
	onError  func(error)
	conn     *dbus.Conn
	stop     chan struct{}
	started  bool
}

func NewTray() *Tray { return &Tray{handlers: map[string]func(){}, stop: make(chan struct{})} }
func (t *Tray) On(id string, fn func()) {
	t.mu.Lock()
	if t.handlers == nil {
		t.handlers = map[string]func(){}
	}
	t.handlers[id] = fn
	t.mu.Unlock()
}
func (t *Tray) SetItems(items []TrayItem) {
	t.mu.Lock()
	t.items = append([]TrayItem(nil), items...)
	c := t.conn
	t.mu.Unlock()
	if c != nil {
		_ = c.Emit(menuObjectPath, menuInterface+".LayoutUpdated", uint32(1), int32(0))
	}
}
func (t *Tray) SetErrorHandler(fn func(error)) { t.mu.Lock(); t.onError = fn; t.mu.Unlock() }
func (t *Tray) Start() error {
	t.mu.Lock()
	if t.started {
		t.mu.Unlock()
		return errors.New("tray already started")
	}
	t.started = true
	t.mu.Unlock()
	go t.run()
	return nil
}
func (t *Tray) Stop() {
	t.mu.Lock()
	if !t.started {
		t.mu.Unlock()
		return
	}
	close(t.stop)
	c := t.conn
	t.started = false
	t.mu.Unlock()
	if c != nil {
		_ = c.Close()
	}
}
func (t *Tray) report(err error) {
	t.mu.Lock()
	fn := t.onError
	t.mu.Unlock()
	if fn != nil {
		fn(err)
	}
}
func (t *Tray) dispatch(id string) {
	t.mu.Lock()
	fn := t.handlers[id]
	t.mu.Unlock()
	if fn != nil {
		go fn()
	}
}

func (t *Tray) run() {
	lastError := ""
	for {
		conn, err := dbus.ConnectSessionBus()
		if err != nil {
			if err.Error() != lastError {
				t.report(fmt.Errorf("connect to the session D-Bus: %w", err))
				lastError = err.Error()
			}
			if !sleepOrStop(t.stop, 2*time.Second) {
				return
			}
			continue
		}
		lastError = ""
		t.mu.Lock()
		t.conn = conn
		t.mu.Unlock()
		if err := t.export(conn); err != nil {
			t.report(fmt.Errorf("export StatusNotifierItem: %w", err))
			_ = conn.Close()
			t.mu.Lock()
			t.conn = nil
			t.mu.Unlock()
			if !sleepOrStop(t.stop, 2*time.Second) {
				return
			}
			continue
		}
		ownerChanges := make(chan *dbus.Signal, 8)
		conn.Signal(ownerChanges)
		if matchErr := conn.AddMatchSignal(dbus.WithMatchSender("org.freedesktop.DBus"), dbus.WithMatchInterface("org.freedesktop.DBus"), dbus.WithMatchMember("NameOwnerChanged"), dbus.WithMatchArg(0, watcherBusName)); matchErr != nil {
			t.report(fmt.Errorf("watch StatusNotifierWatcher changes: %w", matchErr))
		}
		ticker := time.NewTicker(2 * time.Second)
		registered := false
	connectionAttempt:
		for {
			if !registered {
				ok, regErr := t.registerWithWatcher(conn)
				if regErr != nil && regErr.Error() != lastError {
					t.report(regErr)
					lastError = regErr.Error()
				}
				if ok {
					registered = true
					lastError = ""
				}
			}
			select {
			case <-t.stop:
				ticker.Stop()
				conn.RemoveSignal(ownerChanges)
				_ = conn.Close()
				t.mu.Lock()
				if t.conn == conn {
					t.conn = nil
				}
				t.mu.Unlock()
				return
			case sig := <-ownerChanges:
				if len(sig.Body) >= 3 {
					if next, ok := sig.Body[2].(string); ok && next != "" {
						registered = false
					}
					if prev, ok := sig.Body[1].(string); ok && prev != "" {
						registered = false
					}
				}
			case <-ticker.C:
				// Also probe periodically in case a watcher started before signal
				// subscription or its owner-change notification was lost.
				if !registered {
					continue
				}
				has, probeErr := watcherHasOwner(conn)
				if probeErr != nil {
					if probeErr.Error() != lastError {
						t.report(fmt.Errorf("session D-Bus connection stopped: %w", probeErr))
						lastError = probeErr.Error()
					}
					ticker.Stop()
					conn.RemoveSignal(ownerChanges)
					_ = conn.Close()
					t.mu.Lock()
					if t.conn == conn {
						t.conn = nil
					}
					t.mu.Unlock()
					if !sleepOrStop(t.stop, 2*time.Second) {
						return
					}
					break connectionAttempt
				}
				if !has {
					registered = false
				}
			}
		}
	}
}

func sleepOrStop(stop <-chan struct{}, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-stop:
		return false
	case <-timer.C:
		return true
	}
}
func (t *Tray) registerWithWatcher(conn *dbus.Conn) (bool, error) {
	has, err := watcherHasOwner(conn)
	if err != nil {
		return false, fmt.Errorf("check StatusNotifierWatcher: %w", err)
	}
	if !has {
		return false, nil
	}
	names := conn.Names()
	if len(names) == 0 {
		return false, errors.New("session D-Bus connection has no registered name")
	}
	obj := conn.Object(watcherBusName, dbus.ObjectPath(watcherObjectPath))
	if err := obj.Call(watcherInterface+".RegisterStatusNotifierItem", 0, names[0]).Err; err != nil {
		return false, fmt.Errorf("register tray item with StatusNotifierWatcher: %w", err)
	}
	return true, nil
}
func watcherHasOwner(conn *dbus.Conn) (bool, error) {
	var has bool
	err := conn.BusObject().Call("org.freedesktop.DBus.NameHasOwner", 0, watcherBusName).Store(&has)
	return has, err
}

const (
	itemObjectPath    = "/StatusNotifierItem"
	menuObjectPath    = "/StatusNotifierMenu"
	itemInterface     = "org.kde.StatusNotifierItem"
	menuInterface     = "com.canonical.dbusmenu"
	watcherBusName    = "org.kde.StatusNotifierWatcher"
	watcherObjectPath = "/StatusNotifierWatcher"
	watcherInterface  = "org.kde.StatusNotifierWatcher"
)

type iconPixmap struct {
	Width  int32
	Height int32
	Data   []byte
}
type statusToolTip struct {
	Title            string
	Icon             []iconPixmap
	DescriptionTitle string
	Description      string
}
type itemProperties struct{ tray *Tray }

func (p *itemProperties) Get(iface, name string) (dbus.Variant, *dbus.Error) {
	all := p.GetAll(iface)
	if v, ok := all[name]; ok {
		return v, nil
	}
	return dbus.Variant{}, dbus.NewError("org.freedesktop.DBus.Error.UnknownProperty", []any{name})
}
func (p *itemProperties) GetAll(iface string) map[string]dbus.Variant {
	if iface != itemInterface {
		return map[string]dbus.Variant{}
	}
	return map[string]dbus.Variant{
		"Category": dbus.MakeVariant("ApplicationStatus"), "Id": dbus.MakeVariant("buildflow"), "Title": dbus.MakeVariant("BuildFlow"), "Status": dbus.MakeVariant("Active"),
		"WindowId": dbus.MakeVariant(uint32(0)), "IconName": dbus.MakeVariant(""), "IconPixmap": dbus.MakeVariant([]iconPixmap{{Width: 16, Height: 16, Data: buildFlowIcon()}}),
		"OverlayIconName": dbus.MakeVariant(""), "OverlayIconPixmap": dbus.MakeVariant([]iconPixmap{}), "AttentionIconName": dbus.MakeVariant(""), "AttentionIconPixmap": dbus.MakeVariant([]iconPixmap{}),
		"AttentionMovieName": dbus.MakeVariant(""), "ToolTip": dbus.MakeVariant(statusToolTip{Title: "BuildFlow", Icon: []iconPixmap{}, DescriptionTitle: "BuildFlow is running", Description: "Open the local workflow interface"}), "ItemIsMenu": dbus.MakeVariant(false), "Menu": dbus.MakeVariant(dbus.ObjectPath(menuObjectPath)),
	}
}
func (p *itemProperties) Set(iface, name string, value dbus.Variant) *dbus.Error {
	return dbus.NewError("org.freedesktop.DBus.Error.PropertyReadOnly", []any{name})
}

func (t *Tray) export(conn *dbus.Conn) error {
	if err := conn.Export(t, dbus.ObjectPath(itemObjectPath), itemInterface); err != nil {
		return err
	}
	if err := conn.Export(&itemProperties{tray: t}, dbus.ObjectPath(itemObjectPath), "org.freedesktop.DBus.Properties"); err != nil {
		return err
	}
	if err := conn.Export(&trayMenu{tray: t}, dbus.ObjectPath(menuObjectPath), menuInterface); err != nil {
		return err
	}
	return nil
}
func (t *Tray) Activate(x, y int32) *dbus.Error                    { t.dispatch("open"); return nil }
func (t *Tray) SecondaryActivate(x, y int32) *dbus.Error           { t.dispatch("open"); return nil }
func (t *Tray) ContextMenu(x, y int32) *dbus.Error                 { return nil }
func (t *Tray) Scroll(delta int32, orientation string) *dbus.Error { return nil }

type menuLayout struct {
	ID         int32
	Properties map[string]dbus.Variant
	Children   []dbus.Variant
}
type trayMenu struct{ tray *Tray }

func (m *trayMenu) GetLayout(parentID uint32, depth int32, propertyNames []string) (uint32, menuLayout, *dbus.Error) {
	m.tray.mu.Lock()
	items := append([]TrayItem(nil), m.tray.items...)
	m.tray.mu.Unlock()
	children := make([]dbus.Variant, 0, len(items))
	for n, item := range items {
		id := int32(n + 1)
		props := map[string]dbus.Variant{"enabled": dbus.MakeVariant(item.Enabled), "visible": dbus.MakeVariant(true)}
		if item.ID == "-" {
			props["type"] = dbus.MakeVariant("separator")
		} else {
			props["label"] = dbus.MakeVariant(item.Label)
			if item.ID == "autostart" {
				props["toggle-type"] = dbus.MakeVariant("checkmark")
				checked := int32(0)
				if item.Checked {
					checked = 1
				}
				props["toggle-state"] = dbus.MakeVariant(checked)
			}
		}
		children = append(children, dbus.MakeVariant(menuLayout{ID: id, Properties: props, Children: []dbus.Variant{}}))
	}
	return 1, menuLayout{ID: 0, Properties: map[string]dbus.Variant{}, Children: children}, nil
}
func (m *trayMenu) GetGroupProperties(ids []int32, propertyNames []string) ([]struct {
	ID         int32
	Properties map[string]dbus.Variant
}, *dbus.Error) {
	return nil, nil
}
func (m *trayMenu) GetProperty(id int32, name string) (dbus.Variant, *dbus.Error) {
	return dbus.Variant{}, dbus.NewError("com.canonical.dbusmenu.Error.NotFound", []any{id, name})
}
func (m *trayMenu) Event(id int32, event string, data dbus.Variant, timestamp uint32) *dbus.Error {
	if event != "clicked" {
		return nil
	}
	m.tray.mu.Lock()
	items := append([]TrayItem(nil), m.tray.items...)
	m.tray.mu.Unlock()
	index := int(id - 1)
	if index >= 0 && index < len(items) && items[index].Enabled && items[index].ID != "-" {
		m.tray.dispatch(items[index].ID)
	}
	return nil
}
func (m *trayMenu) EventGroup(events []struct {
	ID        int32
	Event     string
	Data      dbus.Variant
	Timestamp uint32
}) ([]int32, *dbus.Error) {
	for _, event := range events {
		_ = m.Event(event.ID, event.Event, event.Data, event.Timestamp)
	}
	return []int32{}, nil
}
func (m *trayMenu) AboutToShow(id int32) (bool, *dbus.Error) { return false, nil }
func (m *trayMenu) AboutToShowGroup(ids []int32) ([]int32, []int32, *dbus.Error) {
	return []int32{}, []int32{}, nil
}

func buildFlowIcon() []byte {
	// Inline 16x16 ARGB icon: a blue tile with a white pixel-font B. This
	// pixmap is carried over D-Bus, so no icon theme or installed asset is needed.
	const glyph = "11110/10001/10001/11110/10001/10001/11110"
	rows := strings.Split(glyph, "/")
	data := make([]byte, 16*16*4)
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			i := (y*16 + x) * 4
			data[i], data[i+1], data[i+2], data[i+3] = 255, 38, 86, 170
			if x < 5 && y/2 < len(rows) && rows[y/2][x] == '1' {
				data[i], data[i+1], data[i+2], data[i+3] = 255, 255, 255, 255
			}
		}
	}
	return data
}
