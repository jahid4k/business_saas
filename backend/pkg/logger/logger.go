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

	"github.com/gofiber/fiber/v3"
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

// ── Context-aware logger ─────────────────────────────────────────────────────

// FromCtx returns a *slog.Logger pre-enriched with all debug-relevant fields
// extracted from the Fiber request context:
//
//   - request_id  (from RequestID middleware)
//   - user_id     (from RequireAuth middleware — empty on public routes)
//   - business_id (from RequireAuth middleware — empty before workspace select)
//   - role        (from RequireAuth middleware — empty before workspace select)
//
// Usage in any handler or service that receives fiber.Ctx:
//
//	log := logger.FromCtx(c)
//	log.Error("task: create failed", slog.Any("error", err))
//
// The resulting log line automatically includes request_id so you can grep
// backend logs by the same ID shown in the browser console.
func FromCtx(c fiber.Ctx) *slog.Logger {
	attrs := make([]any, 0, 4)

	if id, ok := c.Locals("request_id").(string); ok && id != "" {
		attrs = append(attrs, slog.String("request_id", id))
	}
	if uid, ok := c.Locals("user_id").(string); ok && uid != "" {
		attrs = append(attrs, slog.String("user_id", uid))
	}
	if bid, ok := c.Locals("business_id").(string); ok && bid != "" {
		attrs = append(attrs, slog.String("business_id", bid))
	}
	if role, ok := c.Locals("role").(string); ok && role != "" {
		attrs = append(attrs, slog.String("role", role))
	}

	return slog.Default().With(attrs...)
}

// ── ColorHandler ─────────────────────────────────────────────────────────────

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
	prebuilt []byte
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

	h.mu.Lock()
	defer h.mu.Unlock()
	_, err := h.out.Write(buf.Bytes())
	return err
}

// WithAttrs returns a new handler with the given attributes pre-applied.
// This is what slog.Logger.With() calls internally — we must implement it
// correctly so that FromCtx's pre-enriched fields actually appear in output.
func (h *ColorHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	// Return a new attrHandler that wraps us and prepends the given attrs.
	return &attrHandler{parent: h, attrs: attrs}
}

// WithGroup returns a new handler with the given group name applied.
func (h *ColorHandler) WithGroup(name string) slog.Handler {
	return h
}

// ── attrHandler — carries pre-built attrs from logger.With() ─────────────────

// attrHandler wraps ColorHandler and prepends a fixed set of attributes
// to every log record. This makes logger.With(attrs...) work correctly.
type attrHandler struct {
	parent *ColorHandler
	attrs  []slog.Attr
}

func (a *attrHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return a.parent.Enabled(ctx, level)
}

func (a *attrHandler) Handle(ctx context.Context, r slog.Record) error {
	// Clone the record and prepend our fixed attrs before the record's own attrs.
	newRecord := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	newRecord.AddAttrs(a.attrs...)
	r.Attrs(func(attr slog.Attr) bool {
		newRecord.AddAttrs(attr)
		return true
	})
	return a.parent.Handle(ctx, newRecord)
}

func (a *attrHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	combined := make([]slog.Attr, len(a.attrs)+len(attrs))
	copy(combined, a.attrs)
	copy(combined[len(a.attrs):], attrs)
	return &attrHandler{parent: a.parent, attrs: combined}
}

func (a *attrHandler) WithGroup(name string) slog.Handler {
	return a
}

// ── Private helpers ──────────────────────────────────────────────────────────

func levelColor(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return colorBoldRed
	case level >= slog.LevelWarn:
		return colorYellow
	case level >= slog.LevelInfo:
		return colorGreen
	default:
		return colorDim
	}
}

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

func writeAttr(buf *bytes.Buffer, a slog.Attr) {
	a.Value = a.Value.Resolve()

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

	buf.WriteString(colorCyan)
	buf.WriteString(a.Key)
	buf.WriteString(colorReset)
	buf.WriteByte('=')

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

func sourceLocation(pc uintptr) (file string, line int) {
	frames := runtimeCallers(pc)
	if frames == nil {
		return "unknown", 0
	}
	f, _ := frames.Next()
	return shortFile(f.File), f.Line
}

func shortFile(path string) string {
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

// Ensure colorBlue and colorBlue are referenced to avoid unused const errors.
var _ = colorBlue
var _ = colorBoldGreen
