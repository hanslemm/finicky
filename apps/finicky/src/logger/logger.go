package logger

import (
	"finicky/logger/membuf"
	"finicky/util"
	"finicky/window"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// memLog keeps the start-up log lines so they can be copied into the log
// file once file logging turns on. It is bounded: a long-running process must
// not keep every log line in memory.
var memLog = membuf.New(256 << 10)
var file *os.File
var fileMu sync.Mutex

// windowWriter implements io.Writer to send logs to the window
type windowWriter struct{}

func (w *windowWriter) Write(p []byte) (n int, err error) {
	// Remove trailing newline if present
	msg := string(p)
	if len(msg) > 0 && msg[len(msg)-1] == '\n' {
		msg = msg[:len(msg)-1]
	}

	window.SendMessageToWebView("log", msg)
	return len(p), nil
}

// createHandler creates a slog handler with the given writer
func createHandler(writer io.Writer) slog.Handler {
	return slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level:     slog.LevelDebug,
		AddSource: false,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			// Format time as ISO string with microseconds
			if a.Key == slog.TimeKey {
				return slog.Attr{
					Key:   slog.TimeKey,
					Value: slog.StringValue(a.Value.Time().Format("2006-01-02 15:04:05.000000")),
				}
			}
			return a
		},
	})
}

// Setup initializes the logger with basic configuration
func Setup() {
	slog.SetDefault(slog.New(createHandler(io.MultiWriter(memLog, os.Stdout, &windowWriter{}))))
}

// SetupFile turns file logging on or off. It is called after each config
// load. One log file is used per process; turning the option off closes it.
func SetupFile(shouldLog bool) error {
	fileMu.Lock()
	defer fileMu.Unlock()

	if !shouldLog {
		if file != nil {
			slog.Info("File logging turned off")
			Setup()
			file.Close()
			file = nil
		}
		return nil
	}

	if file != nil {
		return nil // already logging to a file
	}

	slog.Warn("Logging requests to disk. Logs may include sensitive information. Disable this by setting logRequests: false.")

	homeDir, err := util.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get user home directory: %w", err)
	}

	logDir := filepath.Join(homeDir, "Library", "Logs", "Finicky")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %v", err)
	}

	currentTime := time.Now().Format("2006-01-02_15-04-05.000")
	logFile := filepath.Join(logDir, fmt.Sprintf("Finicky_%s.log", currentTime))

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to open log file: %v", err)
	}

	// Write buffered logs to file
	if _, err := f.Write(memLog.Bytes()); err != nil {
		f.Close()
		return fmt.Errorf("failed to write buffered logs: %v", err)
	}
	file = f

	slog.SetDefault(slog.New(createHandler(io.MultiWriter(file, memLog, os.Stdout, &windowWriter{}))))
	slog.Info("Log file created", "path", logFile)
	return nil
}

// Close properly closes the logger and any open file handles
func Close() {
	slog.Info("Application closed!")
	fileMu.Lock()
	defer fileMu.Unlock()
	if file != nil {
		file.Close()
	}
}
