package logger

import (
	"context"
	"log"
)

type (
	Logger interface {
		Info(ctx context.Context, msg string)
		Infof(ctx context.Context, format string, args ...any)
		Error(ctx context.Context, msg string)
		Errorf(ctx context.Context, format string, args ...any)
	}
	logger struct{}
)

var (
	DefaultLogger = &logger{}
)

func (l logger) Info(_ context.Context, msg string) {
	log.Println("[Info]", msg)
}

func (l logger) Infof(_ context.Context, format string, args ...any) {
	log.Printf("[Info] "+format, args...)
}

func (l logger) Error(ctx context.Context, msg string) {
	log.Println("[Error]", msg)
}

func (l logger) Errorf(ctx context.Context, format string, args ...any) {
	log.Printf("[Error] "+format, args...)
}
