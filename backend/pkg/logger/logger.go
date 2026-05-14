// pkg/logger/logger.go
package logger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"
)

// ANSI color codes
const (
	colorReset = "\033[0m"
	colorBold  = "\033[1m"
	colorDim   = "\033[2m"

	colorRed     = "\033[31m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorWhite   = "\033[37m"

	colorBoldRed   = "\033[1;31m"
	colorBoldGreen = "\033[1;32m"
)

// sensitiveKeys are never logged as plain values in any environment.
var sensitiveKeys = map[string]bool{
	"password":      true,
	"token":         true,
	"secret":        true,
	"refresh_token": true,
	"access_token":  true,
	"reset_token":   true,
	"api_key":       true,
}

// ColorHandler is a custom slog.Handler that writes colorful,
// human-readable log lines to the terminal.
//
// Format:
//
//	15:04:05 INF message  key=value key=value
//
// It is safe for concurrent use.
type ColorHandler struct {
	mu       sync.Mutex
	out      io.Writer
	opts     ColorHandlerOptions
	prebuilt []byte // pre-built prefix bytes (groups, etc.) — unused for now
}

// ColorHandlerOptions controls ColorHandler behavior.
type ColorHandlerOptions struct {
	// Level is the minimum level to emit. Default: INFO.
	Level slog.Leveler

	// TimeFormat is the Go time layout for the timestamp prefix.
	// Default: "15:04:05"
	TimeFormat string

	// AddSource appends "file.go:line" to every log line when true.
	AddSource bool
}

// NewColorHandler creates a ColorHandler writing to out.
func NewColorHandler(out io.Writer, opts *ColorHandlerOptions) *ColorHandler {
	h := &ColorHandler{out: out}
	if opts != nil {
		h.opts = *opts
	}
	if h.opts.Level == nil {
		h.opts.Level = slog.LevelInfo
	}
	if h.opts.TimeFormat == "" {
		h.opts.TimeFormat = "15:04:05"
	}
	return h
}

// Enabled reports whether the handler handles records at the given level.
func (h *ColorHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

// Handle formats and writes the log record to the output writer.
func (h *ColorHandler) Handle(_ context.Context, r slog.Record) error {
	// Build the line into a buffer to minimise lock hold time.
	var buf bytes.Buffer

	// ── Timestamp ────────────────────────────────────────────
	buf.WriteString(colorDim)
	buf.WriteString(r.Time.Format(h.opts.TimeFormat))
	buf.WriteString(colorReset)
	buf.WriteByte(' ')

	// ── Level badge ──────────────────────────────────────────
	buf.WriteString(levelColor(r.Level))
	buf.WriteString(levelLabel(r.Level))
	buf.WriteString(colorReset)
	buf.WriteByte(' ')

	// ── Source (optional) ────────────────────────────────────
	if h.opts.AddSource && r.PC != 0 {
		frame, _ := r.PC, r.PC // slog.Source helper
		src := slog.Source{}
		frames := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
		frames.Attrs(func(a slog.Attr) bool { return true })
		_ = frame
		_ = src
		// Use runtime to get file:line
		file, line := sourceLocation(r.PC)
		buf.WriteString(colorDim)
		buf.WriteString(fmt.Sprintf("%s:%d ", file, line))
		buf.WriteString(colorReset)
	}

	// ── Message ──────────────────────────────────────────────
	buf.WriteString(messageColor(r.Level))
	buf.WriteString(colorBold)
	buf.WriteString(r.Message)
	buf.WriteString(colorReset)

	// ── Attributes ───────────────────────────────────────────
	r.Attrs(func(a slog.Attr) bool {
		buf.WriteByte(' ')
		writeAttr(&buf, a)
		return true
	})

	buf.WriteByte('\n')

	// Single write under lock — keeps lines from interleaving
	// across goroutines.
	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write(buf.Bytes())
	return err
}

// WithAttrs returns a new handler with the given attributes pre-applied.
func (h *ColorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// For BusinessSAAS Phase 1 this is a pass-through.
	// Extend when you need persistent fields (e.g. request_id on every line).
	return h
}

// WithGroup returns a new handler with the given group name applied.
func (h *ColorHandler) WithGroup(name string) slog.Handler {
	return h
}

// ── Private helpers ──────────────────────────────────────────────────────────

// levelColor returns the ANSI color for the given level badge.
func levelColor(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return colorBoldRed
	case level >= slog.LevelWarn:
		return colorYellow
	case level >= slog.LevelInfo:
		return colorGreen
	default: // DEBUG and below
		return colorDim
	}
}

// levelLabel returns the fixed-width 3-letter level badge.
func levelLabel(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "ERR"
	case level >= slog.LevelWarn:
		return "WRN"
	case level >= slog.LevelInfo:
		return "INF"
	default:
		return "DBG"
	}
}

// messageColor returns a subtle color for the message text itself.
func messageColor(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return colorRed
	case level >= slog.LevelWarn:
		return colorYellow
	default:
		return colorWhite
	}
}

// writeAttr writes a single slog.Attr as  key=value  into buf.
// Sensitive keys are always redacted.
func writeAttr(buf *bytes.Buffer, a slog.Attr) {
	// Resolve the value (handles LogValuer interface).
	a.Value = a.Value.Resolve()

	// Redact sensitive keys regardless of environment.
	if sensitiveKeys[a.Key] {
		buf.WriteString(colorCyan)
		buf.WriteString(a.Key)
		buf.WriteString(colorReset)
		buf.WriteByte('=')
		buf.WriteString(colorDim)
		buf.WriteString("[REDACTED]")
		buf.WriteString(colorReset)
		return
	}

	// Key
	buf.WriteString(colorCyan)
	buf.WriteString(a.Key)
	buf.WriteString(colorReset)
	buf.WriteByte('=')

	// Value — color by kind
	switch a.Value.Kind() {
	case slog.KindString:
		buf.WriteString(colorYellow)
		buf.WriteString(a.Value.String())
		buf.WriteString(colorReset)

	case slog.KindInt64:
		buf.WriteString(colorMagenta)
		buf.WriteString(fmt.Sprintf("%d", a.Value.Int64()))
		buf.WriteString(colorReset)

	case slog.KindFloat64:
		buf.WriteString(colorMagenta)
		buf.WriteString(fmt.Sprintf("%g", a.Value.Float64()))
		buf.WriteString(colorReset)

	case slog.KindBool:
		if a.Value.Bool() {
			buf.WriteString(colorGreen)
		} else {
			buf.WriteString(colorRed)
		}
		buf.WriteString(fmt.Sprintf("%t", a.Value.Bool()))
		buf.WriteString(colorReset)

	case slog.KindDuration:
		buf.WriteString(colorMagenta)
		buf.WriteString(a.Value.Duration().String())
		buf.WriteString(colorReset)

	case slog.KindTime:
		buf.WriteString(colorDim)
		buf.WriteString(a.Value.Time().Format(time.RFC3339))
		buf.WriteString(colorReset)

	case slog.KindGroup:
		// Recursively write grouped attrs as  key.subkey=value
		for _, ga := range a.Value.Group() {
			grouped := slog.Attr{
				Key:   a.Key + "." + ga.Key,
				Value: ga.Value,
			}
			buf.WriteByte(' ')
			writeAttr(buf, grouped)
		}

	default:
		buf.WriteString(colorDim)
		buf.WriteString(fmt.Sprintf("%v", a.Value.Any()))
		buf.WriteString(colorReset)
	}
}

// sourceLocation extracts file and line from a program counter.
func sourceLocation(pc uintptr) (file string, line int) {
	// runtime.CallersFrames is the correct way to resolve a PC.
	// slog stores the PC of the log call site in Record.PC.
	frames := runtimeCallers(pc)
	if frames == nil {
		return "unknown", 0
	}
	f, _ := frames.Next()
	// Trim to just the last two path segments for readability.
	return shortFile(f.File), f.Line
}

// shortFile trims the full path to "pkg/file.go" for readability.
func shortFile(path string) string {
	// Walk back two slashes from the end.
	count := 0
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			count++
			if count == 2 {
				return path[i+1:]
			}
		}
	}
	return path
}
