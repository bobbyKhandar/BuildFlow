//go:build !windows && !linux

// Package winutil has no non-Windows implementation. This file exists so the
// module can be type-checked and unit tested on other platforms; every entry
// point reports that it is unsupported.
package winutil

import "errors"

var errUnsupported = errors.New("winutil: only supported on Windows")

// MessageBox is unsupported outside Windows.
func MessageBox(title, text string, style uint) {}

// ErrorBox is unsupported outside Windows.
func ErrorBox(title, text string) {}

// InfoBox is unsupported outside Windows.
func InfoBox(title, text string) {}

// ConfirmBox always answers no outside Windows.
func ConfirmBox(title, text string) bool { return false }

// OpenURL is unsupported outside Windows.
func OpenURL(url string) error { return errUnsupported }

// OpenFolder is unsupported outside Windows.
func OpenFolder(path string) error { return errUnsupported }

// OpenTextFile is unsupported outside Windows.
func OpenTextFile(path string) error { return errUnsupported }

// AttachConsole always reports false outside Windows.
func AttachConsole() bool { return false }

// Instance is a no-op outside Windows.
type Instance struct{}

// AcquireInstance always succeeds outside Windows.
func AcquireInstance(name string) (*Instance, bool, error) { return &Instance{}, false, nil }

// Release is a no-op outside Windows.
func (i *Instance) Release() {}

func (i *Instance) SetHandoffHandler(handler func()) {}
func HandoffOnAcquire() bool                         { return false }

// AutostartCommand returns an empty string outside Windows.
func AutostartCommand() string { return "" }

// AutostartLocation is unavailable outside supported desktop platforms.
func AutostartLocation() string { return "" }

// AutostartEnabled always reports false outside Windows.
func AutostartEnabled() bool { return false }

// SetAutostart is unsupported outside Windows.
func SetAutostart(enabled bool) error { return errUnsupported }

// PickFolder is unsupported outside Windows.
func PickFolder(title string) (string, error) { return "", errUnsupported }

// CopyToClipboard is unsupported outside Windows.
func CopyToClipboard(text string) error { return errUnsupported }

// TrayItem is a single tray context-menu entry.
type TrayItem struct {
	ID      string
	Label   string
	Checked bool
	Enabled bool
}

// Tray is inert outside Windows.
type Tray struct{ items []TrayItem }

// NewTray returns an inert tray.
func NewTray() *Tray { return &Tray{} }

// On is a no-op outside Windows.
func (t *Tray) On(id string, fn func()) {}

// SetErrorHandler is a no-op on unsupported platforms.
func (t *Tray) SetErrorHandler(fn func(error)) {}

// SetItems is a no-op outside Windows.
func (t *Tray) SetItems(items []TrayItem) { t.items = items }

// Start immediately fails outside Windows.
func (t *Tray) Start() error { return errUnsupported }

// Stop is a no-op outside Windows.
func (t *Tray) Stop() {}

// IsPortReachable always reports false outside Windows.
func IsPortReachable(port int) bool { return false }

// ExeName returns a generic name outside Windows.
func ExeName() string { return "BuildFlow" }
