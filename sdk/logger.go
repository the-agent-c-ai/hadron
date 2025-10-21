package sdk

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// ConfigureDefaultLogger configures the global zerolog logger with sensible defaults.
// It uses a console writer with RFC3339 timestamps for human-readable output.
// If a log level is provided, it sets that level. Otherwise, it reads from the LOG_LEVEL
// environment variable (defaults to "info" if not set or invalid).
func ConfigureDefaultLogger(level ...zerolog.Level) {
	zerolog.TimeFieldFormat = time.RFC3339
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	if len(level) > 0 {
		// Explicit level provided
		zerolog.SetGlobalLevel(level[0])
	} else {
		// Read from LOG_LEVEL environment variable
		logLevel := os.Getenv("LOG_LEVEL")
		if logLevel == "" {
			logLevel = "info"
		}

		parsedLevel, err := zerolog.ParseLevel(logLevel)
		if err != nil {
			// Invalid level, default to info
			parsedLevel = zerolog.InfoLevel

			log.Warn().Str("LOG_LEVEL", logLevel).Msg("Invalid log level, defaulting to info")
		}

		zerolog.SetGlobalLevel(parsedLevel)
	}
}

// NewLogger creates a new zerolog logger with console output.
// This can be used if you want a separate logger instance instead of the global one.
// Optionally accepts a log level; if not provided, uses the global level.
func NewLogger(level ...zerolog.Level) zerolog.Logger {
	logger := zerolog.New(zerolog.ConsoleWriter{Out: os.Stderr}).
		With().
		Timestamp().
		Logger()

	if len(level) > 0 {
		logger = logger.Level(level[0])
	}

	return logger
}
