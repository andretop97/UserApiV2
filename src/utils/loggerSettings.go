package utils

import (
	"log/slog"
	"os"
)

func SetLoggerSettings() {
	level := slog.LevelDebug
	logger :=
		slog.New(
			slog.NewJSONHandler(
				os.Stdout,
				&slog.HandlerOptions{Level: level},
			),
		)
	logger = logger.With("AppName", "UserApiV2")
	slog.SetDefault(logger)
}
