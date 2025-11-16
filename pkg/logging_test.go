package pkg

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/davidkleiven/caesura/testutils"
)

type demoHandler struct {
	group   string
	enabled bool
	records []slog.Record
	attrs   []slog.Attr
}

func (d *demoHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return d.enabled
}

func (d *demoHandler) Handle(ctx context.Context, record slog.Record) error {
	d.records = append(d.records, record)
	return nil
}

func (d *demoHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	d.attrs = append(d.attrs, attrs...)
	return d
}

func (d *demoHandler) WithGroup(name string) slog.Handler {
	d.group = name
	return d
}

func newDemoHandler() *demoHandler {
	return &demoHandler{
		records: []slog.Record{},
		attrs:   []slog.Attr{},
	}
}

func TestEnabled(t *testing.T) {
	ctxHandler := NewHandler(newDemoHandler())
	enabled := ctxHandler.Enabled(context.Background(), slog.LevelInfo)
	testutils.AssertEqual(t, enabled, false)
}

func TestWithAttrs(t *testing.T) {
	attrs := []slog.Attr{{}}
	base := newDemoHandler()
	ctxHandler := NewHandler(base)
	ctxHandler.WithAttrs(attrs)
	testutils.AssertEqual(t, len(base.attrs), 1)
}

func TestWithGroup(t *testing.T) {
	base := newDemoHandler()
	ctxHandler := NewHandler(base)
	ctxHandler.WithGroup("my-group")
	testutils.AssertEqual(t, base.group, "my-group")
}

func TestContextAttrsAdded(t *testing.T) {
	var buf bytes.Buffer
	base := slog.NewTextHandler(&buf, &slog.HandlerOptions{})
	ctxHandler := NewHandler(base)

	// Known key
	ctx := context.WithValue(context.Background(), UserIdKey, "user-id")
	ctx = context.WithValue(ctx, "unknown-key", "whatever")
	record := slog.NewRecord(time.Now(), slog.LevelInfo, "info message", 0)
	ctxHandler.Handle(ctx, record)

	content := buf.String()
	testutils.AssertContains(t, content, UserIdKey, "user-id")

	if strings.Contains(content, "unknown-key") {
		t.Fatalf("Record should not contain 'unknown-key', but got %s", content)
	}
}
