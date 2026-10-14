//go:build linux

package winutil

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	portalName        = "org.freedesktop.portal.Desktop"
	portalPath        = dbus.ObjectPath("/org/freedesktop/portal/desktop")
	portalFileChooser = "org.freedesktop.portal.FileChooser"
	portalRequest     = "org.freedesktop.portal.Request"
	portalTimeout     = 2 * time.Minute
)

func pickFolderPortal(title string) (string, error) {
	return pickFolderPortalWithTimeout(title, nil, portalTimeout)
}

func pickFolderPortalWithTimeout(title string, afterOpen func(*dbus.Conn, dbus.ObjectPath), timeout time.Duration) (string, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return "", fmt.Errorf("cannot open folder picker: session D-Bus is unavailable: %w", err)
	}
	defer conn.Close()
	portal := conn.Object(portalName, portalPath)
	var versionVariant dbus.Variant
	if err := conn.Object(portalName, portalPath).Call("org.freedesktop.DBus.Properties.Get", 0, portalFileChooser, "version").Store(&versionVariant); err != nil {
		return "", fmt.Errorf("cannot open folder picker: the FileChooser portal is unavailable: %w", err)
	}
	version, ok := versionVariant.Value().(uint32)
	if !ok {
		return "", fmt.Errorf("cannot open folder picker: portal returned an invalid FileChooser version")
	}
	if err := requireFileChooserVersion(version); err != nil {
		return "", err
	}

	signals := make(chan *dbus.Signal, 8)
	conn.Signal(signals)
	match := []dbus.MatchOption{
		dbus.WithMatchInterface(portalRequest),
		dbus.WithMatchMember("Response"),
	}
	if err := conn.AddMatchSignal(match...); err != nil {
		conn.RemoveSignal(signals)
		return "", fmt.Errorf("cannot subscribe to the folder-picker response: %w", err)
	}
	defer func() { conn.RemoveSignal(signals); _ = conn.RemoveMatchSignal(match...) }()
	options := map[string]dbus.Variant{"directory": dbus.MakeVariant(true)}
	var requestPath dbus.ObjectPath
	if err := portal.Call(portalFileChooser+".OpenFile", 0, "", title, options).Store(&requestPath); err != nil {
		return "", fmt.Errorf("cannot open the folder picker: %w", err)
	}
	if requestPath == "" {
		return "", fmt.Errorf("folder picker returned an empty request path")
	}
	if afterOpen != nil {
		afterOpen(conn, requestPath)
	}
	selected, cancelled, err := awaitPortalResponse(signals, requestPath, timeout)
	if err != nil {
		if errors.Is(err, errPortalTimeout) {
			_ = conn.Object(portalName, requestPath).Call(portalRequest+".Close", 0).Err
		}
		return "", err
	}
	if cancelled {
		return "", nil
	}
	return selected, nil
}

func requireFileChooserVersion(version uint32) error {
	if version < 3 {
		return fmt.Errorf("cannot open folder picker: FileChooser portal version %d is too old; version 3 or later is required", version)
	}
	return nil
}

var errPortalTimeout = errors.New("folder picker response timed out")

func awaitPortalResponse(signals <-chan *dbus.Signal, requestPath dbus.ObjectPath, timeout time.Duration) (string, bool, error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	for {
		select {
		case signal := <-signals:
			if signal == nil || signal.Path != requestPath || len(signal.Body) < 2 {
				continue
			}
			code, ok := signal.Body[0].(uint32)
			if !ok {
				return "", false, fmt.Errorf("folder picker returned an invalid response code")
			}
			if code == 1 {
				return "", true, nil
			}
			if code != 0 {
				return "", false, fmt.Errorf("folder picker failed with response code %d", code)
			}
			results, ok := signal.Body[1].(map[string]dbus.Variant)
			if !ok {
				return "", false, fmt.Errorf("folder picker returned invalid response data")
			}
			uris, ok := results["uris"].Value().([]string)
			if !ok || len(uris) == 0 {
				return "", false, fmt.Errorf("folder picker did not return a selected folder")
			}
			path, err := localFileURIPath(uris[0])
			if err != nil {
				return "", false, fmt.Errorf("folder picker returned an unusable location: %w", err)
			}
			return path, false, nil
		case <-timer.C:
			return "", false, fmt.Errorf("folder picker timed out after %s: %w", timeout, errPortalTimeout)
		}
	}
}

func localFileURIPath(raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if !strings.EqualFold(u.Scheme, "file") {
		return "", fmt.Errorf("expected a file:// URI, got %q", u.Scheme)
	}
	if u.Host != "" && !strings.EqualFold(u.Host, "localhost") {
		return "", fmt.Errorf("URI refers to a remote host %q", u.Host)
	}
	path := filepath.Clean(filepath.FromSlash(u.Path))
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("URI path is not absolute")
	}
	return path, nil
}
