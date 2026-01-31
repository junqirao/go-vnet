package logger

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

type (
	jsonLogger struct {
		logger  Logger
		ctxKeys []string
	}
	logDats struct {
		Level     string         `json:"level"`
		Timestamp string         `json:"timestamp"`
		Msg       string         `json:"msg"`
		Ctx       map[string]any `json:"context"`
	}
)

func NewJsonLogger(l Logger, ctxKeys ...string) Logger {
	if l == nil {
		l = defaultLogger{}
	}
	return &jsonLogger{
		logger:  l,
		ctxKeys: ctxKeys,
	}
}

func (j jsonLogger) Info(ctx context.Context, msg string) {
	j.logger.Info(ctx, j.buildCtxKeys(ctx, LevelInfo, msg))
}

func (j jsonLogger) Infof(ctx context.Context, format string, args ...any) {
	j.logger.Infof(ctx, j.buildCtxKeys(ctx, LevelInfo, fmt.Sprintf(format, args...)))
}

func (j jsonLogger) Error(ctx context.Context, msg string) {
	j.logger.Error(ctx, j.buildCtxKeys(ctx, LevelError, msg))
}

func (j jsonLogger) Errorf(ctx context.Context, format string, args ...any) {
	j.logger.Errorf(ctx, j.buildCtxKeys(ctx, LevelError, fmt.Sprintf(format, args...)))
}

func (j jsonLogger) buildCtxKeys(ctx context.Context, level, msg string) string {
	ld := logDats{
		Level:     level,
		Msg:       msg,
		Timestamp: time.Now().Format(time.RFC3339),
	}
	if ctx != nil {
		ld.Ctx = make(map[string]any)
		for _, key := range j.ctxKeys {
			if val, ok := ctx.Value(key).(string); ok {
				ld.Ctx[key] = val
			}
		}
	}
	bs, _ := json.Marshal(ld)
	return string(bs)
}
