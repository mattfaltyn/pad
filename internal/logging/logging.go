// Package logging provides structured logging using log/slog.
//
// Usage:
//
//	logging.Setup("info", "json")   // call once at startup
//	slog.Info("something happened", "key", value)
//
// All application code should use the slog package directly after Setup has
// been called — it configures the default slog logger.
package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

const (
	defaultMaxLogBytes = 10 * 1024 * 1024
	defaultLogBackups  = 5
)

// Setup configures the default slog logger.
//
//   - level: "debug", "info", "warn", "error" (default "info")
//   - format: "json" or "text" (default "text")
//
// After calling Setup, use slog.Info / slog.Error / etc. everywhere.
func Setup(level, format string) {
	if path := strings.TrimSpace(os.Getenv("PAD_LOG_FILE")); path != "" {
		maxBytes := positiveEnvInt64("PAD_LOG_MAX_BYTES", defaultMaxLogBytes)
		backups := int(positiveEnvInt64("PAD_LOG_BACKUPS", defaultLogBackups))
		writer, err := newRotatingFileWriter(path, maxBytes, backups)
		if err == nil {
			SetupWriter(writer, level, format)
			return
		}
		fmt.Fprintf(os.Stderr, "pad: open rotating log %s: %v; using stderr\n", path, err)
	}
	SetupWriter(os.Stderr, level, format)
}

func positiveEnvInt64(name string, fallback int64) int64 {
	value, err := strconv.ParseInt(strings.TrimSpace(os.Getenv(name)), 10, 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

type rotatingFileWriter struct {
	mu       sync.Mutex
	path     string
	file     *os.File
	size     int64
	maxBytes int64
	backups  int
}

func newRotatingFileWriter(path string, maxBytes int64, backups int) (*rotatingFileWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return &rotatingFileWriter{path: path, file: f, size: info.Size(), maxBytes: maxBytes, backups: backups}, nil
}

func (w *rotatingFileWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.size > 0 && w.size+int64(len(p)) > w.maxBytes {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingFileWriter) rotate() error {
	if err := w.file.Close(); err != nil {
		return err
	}
	if w.backups > 0 {
		_ = os.Remove(fmt.Sprintf("%s.%d", w.path, w.backups))
		for i := w.backups - 1; i >= 1; i-- {
			_ = os.Rename(fmt.Sprintf("%s.%d", w.path, i), fmt.Sprintf("%s.%d", w.path, i+1))
		}
		if err := os.Rename(w.path, w.path+".1"); err != nil && !os.IsNotExist(err) {
			return err
		}
	} else if err := os.Remove(w.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	w.file = f
	w.size = 0
	return nil
}

// SetupWriter is like Setup but writes to w instead of stderr (useful for tests).
func SetupWriter(w io.Writer, level, format string) {
	lvl := parseLevel(level)

	opts := &slog.HandlerOptions{
		Level: lvl,
	}

	var handler slog.Handler
	switch strings.ToLower(format) {
	case "json":
		handler = slog.NewJSONHandler(w, opts)
	default:
		handler = slog.NewTextHandler(w, opts)
	}

	slog.SetDefault(slog.New(handler))
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
