package sdk

import (
	"os"

	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
)

// LoadEnv loads environment variables from the specified .env file.
// If the file doesn't exist or cannot be loaded, it logs a fatal error and exits.
func LoadEnv(path string) error {
	if err := godotenv.Load(path); err != nil {
		log.Fatal().Err(err).Str("path", path).Msg("Failed to load .env file")
	}

	return nil
}

// GetEnv retrieves an environment variable value.
// If the variable is not set, it logs a fatal error and exits.
func GetEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		log.Fatal().Str("key", key).Msg("Required environment variable not set")
	}

	return value
}

// GetEnvDefault retrieves an environment variable value with a default fallback.
// If the variable is not set, it returns the default value.
func GetEnvDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}

// MustGetEnv retrieves an environment variable value.
// If the variable is not set, it panics with an error.
func MustGetEnv(key string) string {
	value := os.Getenv(key)
	if value == "" {
		panic("required environment variable not set: " + key)
	}

	return value
}
