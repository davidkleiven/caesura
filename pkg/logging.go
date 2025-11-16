package pkg

import (
	"context"
	"log/slog"
)

const (
	ReqIdKey  = "requestId"
	UserIdKey = "userId"
	OrgIdKey  = "orgId"
	HostKey   = "host"
)

type CtxHandler struct {
	base slog.Handler
}

func (c *CtxHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return c.base.Enabled(ctx, level)
}

func (c *CtxHandler) Handle(ctx context.Context, record slog.Record) error {
	stringAttrs := []string{ReqIdKey, UserIdKey, OrgIdKey, HostKey}
	for _, attr := range stringAttrs {
		if value, ok := ctx.Value(attr).(string); ok {
			record.AddAttrs(slog.String(attr, value))
		}
	}
	return c.base.Handle(ctx, record)
}

func (c *CtxHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &CtxHandler{base: c.base.WithAttrs(attrs)}
}

func (c *CtxHandler) WithGroup(name string) slog.Handler {
	return &CtxHandler{base: c.base.WithGroup(name)}
}

func NewHandler(h slog.Handler) *CtxHandler {
	return &CtxHandler{base: h}
}
