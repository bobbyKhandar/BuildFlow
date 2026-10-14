// Package logging provides BuildFlow's small leveled file logger with size
// based rotation. Log files live in the per-user application-data folder so a
// crashed or misbehaving run can always be inspected afterwards.
package logging

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	maxFileBytes = 2 << 20 // 2 MiB before rotation
	maxFiles     = 3       // buildflow.log, .1, .2
	fileName     = "buildflow.log"
)

// Level is a log severity.
type Level int

// Supported levels.
const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// ParseLevel converts a case-insensitive name into a Level.
func ParseLevel(name string) Level {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "debug":
		return LevelDebug
	case "warn", "warning":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "DEBUG"
	case LevelWarn:
		return "WARN"
	case LevelError:
		return "ERROR"
	default:
		return "INFO"
	}
}

// Logger writes timestamped lines to a rotating file and, when one is
// attached, to the console as well.
type Logger struct {
	mu    sync.Mutex
	path  string
	file  *os.File
	level Level
	echo  bool
}

// New opens (creating if needed) the log file inside dir.
func New(dir string, level Level, echo bool) (*Logger, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create log folder %q: %w", dir, err)
	}
	l := &Logger{path: filepath.Join(dir, fileName), level: level, echo: echo}
	if err := l.open(); err != nil {
		return nil, err
	}
	return l, nil
}

// Path returns the active log file path.
func (l *Logger) Path() string { return l.path }

func (l *Logger) open() error {
	l.rotateIfNeeded()
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return fmt.Errorf("open log file %q: %w", l.path, err)
	}
	l.file = f
	return nil
}

func (l *Logger) rotateIfNeeded() {
	info, err := os.Stat(l.path)
	if err != nil || info.Size() < maxFileBytes {
		return
	}
	// Shift buildflow.1 -> .2, buildflow -> .1
	_ = os.Remove(fmt.Sprintf("%s.%d", l.path, maxFiles-1))
	for i := maxFiles - 2; i >= 1; i-- {
		_ = os.Rename(fmt.Sprintf("%s.%d", l.path, i), fmt.Sprintf("%s.%d", l.path, i+1))
	}
	_ = os.Rename(l.path, l.path+".1")
}

func (l *Logger) write(level Level, format string, args ...any) {
	if l == nil || level < l.level {
		return
	}
	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("%s [%s] %s\n", time.Now().Format("2006-01-02 15:04:05.000"), level, msg)

	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		if info, err := l.file.Stat(); err == nil && info.Size() >= maxFileBytes {
			_ = l.file.Close()
			l.file = nil
			if err := l.open(); err == nil {
				_, _ = l.file.WriteString(line)
			}
		}
		if l.file != nil {
			_, _ = l.file.WriteString(line)
		}
	}
	if l.echo {
		fmt.Fprint(os.Stderr, line)
	}
}

// Debugf logs at debug level.
func (l *Logger) Debugf(format string, args ...any) { l.write(LevelDebug, format, args...) }

// Infof logs at info level.
func (l *Logger) Infof(format string, args ...any) { l.write(LevelInfo, format, args...) }

// Warnf logs at warn level.
func (l *Logger) Warnf(format string, args ...any) { l.write(LevelWarn, format, args...) }

// Errorf logs at error level.
func (l *Logger) Errorf(format string, args ...any) { l.write(LevelError, format, args...) }

// Tail returns the last n lines of the active log file.
func (l *Logger) Tail(n int) []string {
	if n <= 0 || n > 2000 {
		n = 200
	}
	l.mu.Lock()
	path := l.path
	l.mu.Unlock()
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], "\r")
	}
	return lines
}

// Close flushes and closes the log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file == nil {
		return nil
	}
	err := l.file.Close()
	l.file = nil
	return err
}
