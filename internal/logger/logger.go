// Copyright 2025 Arion Yau
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
)

var logger zerolog.Logger

const (
	LOG_INFO  = "info"
	LOG_DEBUG = "debug"
	LOG_WARN  = "warn"
	LOG_ERROR = "error"
)

func init() {
	// Default to silent mode (no output)
	SetSilentMode(true)
}

// SetSilentMode configures whether logging should be silent or output to stderr
func SetSilentMode(silent bool) {

	var output io.Writer
	if silent {
		output = io.Discard
	} else {
		// Setup console writer for CLI-friendly output
		output = zerolog.ConsoleWriter{
			Out:        os.Stderr,
			TimeFormat: time.RFC3339,
			NoColor:    false,
		}
	}

	logger = zerolog.New(output).With().Timestamp().Logger()

	// Set default level to info
	zerolog.SetGlobalLevel(zerolog.InfoLevel)
}

// New returns a new logger instance
func New() zerolog.Logger {
	return logger
}

// SetLevel sets the global log level
func SetLevel(level string) {
	switch level {
	case LOG_DEBUG:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	case LOG_INFO:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case LOG_WARN:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case LOG_ERROR:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}
}

// Info logs an info message
func Info(msg string) {
	logger.Info().Msg(msg)
}

// Debug logs a debug message
func Debug(msg string) {
	logger.Debug().Msg(msg)
}

// Error logs an error message
func Error(err error, msg string) {
	logger.Error().Err(err).Msg(msg)
}

// Warn logs a warning message
func Warn(msg string) {
	logger.Warn().Msg(msg)
}
