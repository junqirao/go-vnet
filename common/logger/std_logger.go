package logger

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type (
	stdLogger struct {
		l       Logger
		ctxKeys []string
	}
)

func NewStdLogger(l Logger, ctxKeys ...string) Logger {
	if l == nil {
		l = defaultLogger{}
	}
	return &stdLogger{
		l:       l,
		ctxKeys: ctxKeys,
	}
}

func (s stdLogger) Info(ctx context.Context, msg string) {
	s.l.Info(ctx, fmt.Sprintf("%s%s", s.buildCtxKeys(ctx, LevelInfo), msg))
}

func (s stdLogger) Infof(ctx context.Context, format string, args ...any) {
	s.l.Infof(ctx, fmt.Sprintf("%s%s", s.buildCtxKeys(ctx, LevelInfo), format), args...)
}

func (s stdLogger) Error(ctx context.Context, msg string) {
	s.l.Error(ctx, fmt.Sprintf("%s%s", s.buildCtxKeys(ctx, LevelError), msg))
}

func (s stdLogger) Errorf(ctx context.Context, format string, args ...any) {
	s.l.Errorf(ctx, fmt.Sprintf("%s%s", s.buildCtxKeys(ctx, LevelError), format), args...)
}

func (s stdLogger) buildCtxKeys(ctx context.Context, level string) string {
	sb := strings.Builder{}
	sb.WriteString("[")
	sb.WriteString(level)
	sb.WriteString("]")
	sb.WriteString("[")
	sb.WriteString(time.Now().Format(time.RFC3339))
	sb.WriteString("]")

	if ctx != nil {
		for _, key := range s.ctxKeys {
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
