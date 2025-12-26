package logger

import (
	"context"
	"fmt"
)

type (
	Logger interface {
		Info(ctx context.Context, msg string)
		Infof(ctx context.Context, format string, args ...any)
		Error(ctx context.Context, msg string)
		Errorf(ctx context.Context, format string, args ...any)
	}
	defaultLogger struct{}
	nopLogger     struct{}
	Level         = string
)

const (
	LevelInfo  Level = "Info"
	LevelError Level = "Error"
)

var (
	DefaultLogger = NewStdLogger(defaultLogger{})
	NopLogger     = nopLogger{}
)

func (d defaultLogger) Info(_ context.Context, msg string) {
	fmt.Println(msg)
}

func (d defaultLogger) Infof(_ context.Context, format string, args ...any) {
	fmt.Println(fmt.Sprintf(format, args...))
}

func (d defaultLogger) Error(_ context.Context, msg string) {
	fmt.Println(msg)
}

func (d defaultLogger) Errorf(_ context.Context, format string, args ...any) {
	fmt.Println(fmt.Sprintf(format, args...))
}

func (n nopLogger) Info(ctx context.Context, msg string) {
}

func (n nopLogger) Infof(ctx context.Context, format string, args ...any) {
}

func (n nopLogger) Error(ctx context.Context, msg string) {
}

func (n nopLogger) Errorf(ctx context.Context, format string, args ...any) {
}
