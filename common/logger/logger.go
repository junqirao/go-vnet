package logger

import (
	"context"
	"log"
	"strings"
)

type (
	Logger interface {
		Info(ctx context.Context, msg string)
		Infof(ctx context.Context, format string, args ...any)
		Error(ctx context.Context, msg string)
		Errorf(ctx context.Context, format string, args ...any)
	}
	logger struct {
		ctxKeys []string
	}
)

var (
	DefaultLogger = &logger{}
)

func NewLogger(ctxKeys ...string) Logger {
	return &logger{ctxKeys: ctxKeys}
}

func (l logger) Info(ctx context.Context, msg string) {
	log.Println(l.buildCtxKeys(ctx, "Info"), msg)
}

func (l logger) Infof(ctx context.Context, format string, args ...any) {
	log.Printf(l.buildCtxKeys(ctx, "Info")+format, args...)
}

func (l logger) Error(ctx context.Context, msg string) {
	log.Println(l.buildCtxKeys(ctx, "Error"), msg)
}

func (l logger) Errorf(ctx context.Context, format string, args ...any) {
	log.Printf(l.buildCtxKeys(ctx, "Error")+format, args...)
}

func (l logger) buildCtxKeys(ctx context.Context, level string) string {
	sb := strings.Builder{}
	sb.WriteString("[")
	sb.WriteString(level)
	sb.WriteString("]")

	if ctx != nil {
		for _, key := range l.ctxKeys {
			if val, ok := ctx.Value(key).(string); ok {
				sb.WriteString("[")
				sb.WriteString(key)
				sb.WriteString("=")
				sb.WriteString(val)
				sb.WriteString("]")
			}
		}
	}
	sb.WriteString(" ")
	return sb.String()
}
