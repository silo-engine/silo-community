package log

import (
	"log/slog"
	"os"
)

type LogManager struct {
	logger *slog.Logger
}

func NewLogManager() *LogManager {
	logger := slog.New(
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo,
		}),
	)

	return &LogManager{
		logger: logger,
	}
}

func (lm *LogManager) Logger() *slog.Logger {
	return lm.logger
}
