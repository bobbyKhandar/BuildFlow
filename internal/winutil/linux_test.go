//go:build linux

package winutil

import (
	"errors"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

func TestLocalFileURIPathDecodesSpacesAndUnicode(t *testing.T) {
	got, err := localFileURIPath("file:///tmp/space%20and%20%E9%9B%AA")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/tmp/space and 雪" {
		t.Fatalf("got %q", got)
	}
	for _, raw := range []string{"https://example.com/tmp", "file://other-host/tmp/a", "file:relative"} {
		if _, err := localFileURIPath(raw); err == nil {
			t.Errorf("expected %q to be rejected", raw)
		}
	}
}

func TestPortalResponseCancelFailureAndTimeout(t *testing.T) {
	path := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/1")
	cancel := make(chan *dbus.Signal, 1)
	cancel <- &dbus.Signal{Path: path, Body: []any{uint32(1), map[string]dbus.Variant{}}}
	if _, cancelled, err := awaitPortalResponse(cancel, path, time.Second); err != nil || !cancelled {
		t.Fatalf("cancel: cancelled=%v err=%v", cancelled, err)
	}

	failure := make(chan *dbus.Signal, 1)
	failure <- &dbus.Signal{Path: path, Body: []any{uint32(2), map[string]dbus.Variant{}}}
	if _, _, err := awaitPortalResponse(failure, path, time.Second); err == nil {
		t.Fatal("portal error was accepted")
	}

	if _, _, err := awaitPortalResponse(make(chan *dbus.Signal), path, time.Millisecond); err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("timeout error = %v", err)
	}
}

func TestPortalResponseSelectedURI(t *testing.T) {
	path := dbus.ObjectPath("/org/freedesktop/portal/desktop/request/2")
	signals := make(chan *dbus.Signal, 1)
	signals <- &dbus.Signal{Path: path, Body: []any{uint32(0), map[string]dbus.Variant{"uris": dbus.MakeVariant([]string{"file:///tmp/a%20path/%E9%9B%AA"})}}}
	got, cancelled, err := awaitPortalResponse(signals, path, time.Second)
	if err != nil || cancelled || got != "/tmp/a path/雪" {
		t.Fatalf("selection = %q, cancelled=%v, err=%v", got, cancelled, err)
	}
}

func TestFileChooserVersionRequirement(t *testing.T) {
	if err := requireFileChooserVersion(2); err == nil {
		t.Fatal("portal v2 should be rejected")
	}
	if err := requireFileChooserVersion(3); err != nil {
		t.Fatalf("portal v3: %v", err)
	}
	if err := requireFileChooserVersion(4); err != nil {
		t.Fatalf("portal v4: %v", err)
	}
}

func TestLivePortalCancellation(t *testing.T) {
	if os.Getenv("BUILDFLOW_TEST_PORTAL") != "1" {
		t.Skip("set BUILDFLOW_TEST_PORTAL=1 to exercise the active desktop portal")
	}
	var closeErr error
	selected, err := pickFolderPortalWithTimeout("BuildFlow portal integration test", func(conn *dbus.Conn, path dbus.ObjectPath) {
		closeErr = conn.Object(portalName, path).Call(portalRequest+".Close", 0).Err
	}, 8*time.Second)
	if errors.Is(err, errPortalTimeout) {
		t.Skipf("the active portal kept the request pending; interactive selection/cancellation requires user input: close error: %v", closeErr)
	}
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if selected != "" {
		t.Fatalf("cancelled portal returned a path: %q", selected)
	}
}

func TestDesktopQuoteEscapesSpecialCharacters(t *testing.T) {
	got, err := desktopQuote(`/home/user/Build Flow $beta`)
	if err != nil {
		t.Fatal(err)
	}
	if got != `"/home/user/Build Flow \$beta"` {
		t.Fatalf("desktop quoted path = %s", got)
	}
	if _, err := desktopQuote("/tmp/has\nnewline"); err == nil {
		t.Fatal("newline was accepted")
	}
}

func TestIsolatedAutostartDoesNotTouchNormalEntry(t *testing.T) {
	root := t.TempDir()
	normalConfig := filepath.Join(root, "normal-config")
	isolated := filepath.Join(root, "isolated root")
	t.Setenv("XDG_CONFIG_HOME", normalConfig)
	t.Setenv("HOME", root)
	t.Setenv("BUILDFLOW_HOME", isolated)
	if err := SetAutostart(true); err != nil {
		t.Fatal(err)
	}
	entry := AutostartLocation()
	if !strings.HasPrefix(entry, isolated+string(filepath.Separator)) {
		t.Fatalf("entry escaped isolated root: %s", entry)
	}
	raw, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "Exec=\"") || !strings.Contains(string(raw), "--minimized") {
		t.Fatalf("unexpected desktop entry: %s", raw)
	}
	if _, err := os.Stat(filepath.Join(normalConfig, "autostart")); !os.IsNotExist(err) {
		t.Fatalf("normal XDG autostart directory was touched: %v", err)
	}
	if !AutostartEnabled() {
		t.Fatal("autostart entry not detected")
	}
	if err := SetAutostart(false); err != nil {
		t.Fatal(err)
	}
	if AutostartEnabled() {
		t.Fatal("autostart entry still exists after removal")
	}
}

func TestInstanceHandoffAndCleanQuit(t *testing.T) {
	runtime := filepath.Join(t.TempDir(), "runtime")
	if err := os.MkdirAll(runtime, 0o700); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(runtime, 0o700)
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
	t.Setenv("HOME", t.TempDir())
	first, already, err := AcquireInstance("same-data-root")
	if err != nil || already {
		t.Fatalf("first acquire: already=%v err=%v", already, err)
	}
	defer first.Release()
	handed := make(chan struct{}, 1)
	first.SetHandoffHandler(func() { handed <- struct{}{} })
	second, already, err := AcquireInstance("same-data-root")
	if err != nil || !already || second != nil {
		t.Fatalf("second acquire: already=%v err=%v", already, err)
	}
	if !HandoffOnAcquire() {
		t.Fatal("second launch did not deliver handoff")
	}
	select {
	case <-handed:
	case <-time.After(2 * time.Second):
		t.Fatal("running instance did not receive handoff")
	}
	first.Release()
	restarted, already, err := AcquireInstance("same-data-root")
	if err != nil || already {
		t.Fatalf("reacquire after clean quit: already=%v err=%v", already, err)
	}
	restarted.Release()
}

func TestInstanceRecoversAfterOwnerCrash(t *testing.T) {
	runtime := filepath.Join(t.TempDir(), "runtime")
	if err := os.MkdirAll(runtime, 0o700); err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(runtime, 0o700)
	t.Setenv("XDG_RUNTIME_DIR", runtime)
	t.Setenv("XDG_STATE_HOME", filepath.Join(t.TempDir(), "state"))
	t.Setenv("HOME", t.TempDir())
	first, _, err := AcquireInstance("crash-root")
	if err != nil {
		t.Fatal(err)
	}
	close(first.stop)
	_ = first.listener.Close()
	_ = syscall.Flock(int(first.lock.Fd()), syscall.LOCK_UN)
	_ = first.lock.Close() // emulate process exit; deliberately leave stale socket
	second, already, err := AcquireInstance("crash-root")
	if err != nil || already {
		t.Fatalf("recovery: already=%v err=%v", already, err)
	}
	second.Release()
}

func TestPortReachableDetectsConflict(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	if !IsPortReachable(port) {
		t.Fatal("listening loopback port was not detected")
	}
	if IsPortReachable(0) {
		t.Fatal("port zero unexpectedly reachable")
	}
}

func TestTrayMenuExportsActionsAndIcon(t *testing.T) {
	tray := NewTray()
	items := []TrayItem{{ID: "open", Label: "Open BuildFlow", Enabled: true}, {ID: "autostart", Label: "Start at login", Enabled: true, Checked: true}, {ID: "quit", Label: "Quit BuildFlow", Enabled: true}}
	tray.SetItems(items)
	menu := &trayMenu{tray: tray}
	_, layout, dbusErr := menu.GetLayout(0, -1, nil)
	if dbusErr != nil {
		t.Fatal(dbusErr)
	}
	if len(layout.Children) != len(items) {
		t.Fatalf("menu contains %d items", len(layout.Children))
	}
	var called string
	calledCh := make(chan string, 1)
	tray.On("open", func() { calledCh <- "open" })
	if dbusErr := menu.Event(1, "clicked", dbus.MakeVariant(0), 0); dbusErr != nil {
		t.Fatal(dbusErr)
	}
	select {
	case called = <-calledCh:
	case <-time.After(time.Second):
		t.Fatal("menu action was not dispatched")
	}
	if called != "open" {
		t.Fatalf("dispatched %q", called)
	}
	if got := len(buildFlowIcon()); got != 16*16*4 {
		t.Fatalf("icon byte count = %d", got)
	}
}

func TestFileURIUsesURLDecoder(t *testing.T) {
	u, err := url.Parse("file:///tmp/a%20b")
	if err != nil {
		t.Fatal(err)
	}
	if u.Path != "/tmp/a b" {
		t.Fatalf("URL path = %q", u.Path)
	}
}

func TestDesktopHelpersPreserveArgumentsAndClipboardFallback(t *testing.T) {
	dir := t.TempDir()
	openOut := filepath.Join(dir, "opened.txt")
	xselOut := filepath.Join(dir, "clipboard.txt")
	openScript := "#!/bin/sh\nprintf '%s' \"$1\" > '" + openOut + "'\n"
	xselScript := "#!/bin/sh\n/usr/bin/cat > '" + xselOut + "'\n"
	for name, body := range map[string]string{"xdg-open": openScript, "xsel": xselScript} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
	t.Setenv("WAYLAND_DISPLAY", "wayland-test")
	t.Setenv("XDG_SESSION_TYPE", "wayland")
	if err := OpenURL("https://example.test/a path/雪"); err != nil {
		t.Fatal(err)
	}
	opened, err := os.ReadFile(openOut)
	if err != nil {
		t.Fatal(err)
	}
	if string(opened) != "https://example.test/a path/雪" {
		t.Fatalf("xdg-open argument changed: %q", opened)
	}
	if err := CopyToClipboard("clipboard 雪"); err == nil || !strings.Contains(err.Error(), "wl-copy") {
		t.Fatalf("missing Wayland helper error = %v", err)
	}
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("XDG_SESSION_TYPE", "x11")
	t.Setenv("DISPLAY", ":test")
	if err := CopyToClipboard("clipboard 雪"); err != nil {
		t.Fatal(err)
	}
	clipboard, err := os.ReadFile(xselOut)
	if err != nil {
		t.Fatal(err)
	}
	if string(clipboard) != "clipboard 雪" {
		t.Fatalf("clipboard bytes = %q", clipboard)
	}
}
