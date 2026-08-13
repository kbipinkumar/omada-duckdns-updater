package main

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const logFileName = "updater.log"

var (
	logMu                   sync.Mutex
	lastIncompleteConfigLog time.Time
)

// getLogFilePath returns the path to the application log file.
func getLogFilePath() string {
	dir := getDataDir()
	if dir != "" {
		return filepath.Join(dir, logFileName)
	}
	return logFileName
}

// initLogging configures the default logger to write to stderr and updater.log.
// Additional writers, such as the Windows event log, may be supplied.
func initLogging(extraWriters ...io.Writer) io.Closer {
	log.SetFlags(log.Ldate | log.Ltime)

	writers := []io.Writer{os.Stderr}
	logPath := getLogFilePath()

	if dir := filepath.Dir(logPath); dir != "" && dir != "." {
		_ = os.MkdirAll(dir, 0755)
	}

	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("[WARN] could not open log file %s: %v", logPath, err)
	} else {
		writers = append(writers, logFile)
	}

	writers = append(writers, extraWriters...)
	log.SetOutput(bestEffortWriter{writers: writers})

	log.Printf("[INFO] logging initialized (file=%s)", logPath)
	return &logCloser{file: logFile, extra: extraWriters}
}

// bestEffortWriter writes to every sink and ignores individual sink failures.
type bestEffortWriter struct {
	writers []io.Writer
}

// Write duplicates p to each configured writer.
func (w bestEffortWriter) Write(p []byte) (int, error) {
	for _, writer := range w.writers {
		_, _ = writer.Write(p)
	}
	return len(p), nil
}

// logCloser closes the log file opened by initLogging.
type logCloser struct {
	file  *os.File
	extra []io.Writer
}

// Close closes the log file when one was opened.
func (c *logCloser) Close() error {
	if c == nil || c.file == nil {
		return nil
	}
	return c.file.Close()
}

// logInfo writes an informational log line.
func logInfo(format string, args ...interface{}) {
	log.Printf("[INFO] %s", sanitizeLogMessage(fmt.Sprintf(format, args...)))
}

// logWarn writes a warning log line.
func logWarn(format string, args ...interface{}) {
	log.Printf("[WARN] %s", sanitizeLogMessage(fmt.Sprintf(format, args...)))
}

// logError writes an error log line.
func logError(format string, args ...interface{}) {
	log.Printf("[ERROR] %s", sanitizeLogMessage(fmt.Sprintf(format, args...)))
}

// sanitizeLogMessage escapes control characters to reduce log forging risk.
func sanitizeLogMessage(message string) string {
	return strings.NewReplacer("\r", `\r`, "\n", `\n`).Replace(message)
}

// sanitizeURLForLog returns a URL safe for logging with userinfo removed.
func sanitizeURLForLog(raw string) string {
	if strings.TrimSpace(raw) == "" {
		return "empty"
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "invalid-url"
	}
	parsed.User = nil
	return parsed.String()
}

// describeConfig returns a safe summary of configuration state for troubleshooting.
func describeConfig(config *Config) string {
	if config == nil {
		return "config=nil"
	}

	parts := []string{
		fmt.Sprintf("omada_url=%s", configFieldStatus(config.OmadaURL)),
		fmt.Sprintf("omada_site_id=%q", config.OmadaSiteID),
		fmt.Sprintf("omadac_id=%s", configFieldStatus(config.OmadaOmadacID)),
		fmt.Sprintf("omada_client_id=%s", configFieldStatus(config.OmadaClientID)),
		fmt.Sprintf("omada_client_secret=%s", secretFieldStatus(config.OmadaClientSecret)),
		fmt.Sprintf("duckdns_token=%s", secretFieldStatus(config.DuckDNSToken)),
		fmt.Sprintf("duckdns_domain=%q", config.DuckDNSDomain),
		fmt.Sprintf("update_ipv4=%t", config.UpdateIPv4),
		fmt.Sprintf("update_ipv6=%t", config.UpdateIPv6),
		fmt.Sprintf("update_interval=%d", config.UpdateInterval),
	}
	if missing := missingConfigFields(config); len(missing) > 0 {
		parts = append(parts, fmt.Sprintf("missing=[%s]", strings.Join(missing, ", ")))
	}
	return strings.Join(parts, " ")
}

// configFieldStatus reports whether a non-secret configuration value is set.
func configFieldStatus(value string) string {
	if strings.TrimSpace(value) == "" {
		return "empty"
	}
	return "set"
}

// secretFieldStatus reports whether a secret value is present without logging it.
func secretFieldStatus(value string) string {
	if strings.TrimSpace(value) == "" {
		return "empty"
	}
	return "set"
}

// logIncompleteConfigThrottled logs incomplete configuration at most once every five minutes.
func logIncompleteConfigThrottled(config *Config) {
	logMu.Lock()
	defer logMu.Unlock()

	if time.Since(lastIncompleteConfigLog) < 5*time.Minute {
		return
	}
	lastIncompleteConfigLog = time.Now()

	if config == nil {
		logInfo("scheduled update skipped: configuration unavailable")
		return
	}
	logInfo("scheduled update skipped: incomplete configuration (%s)", describeConfig(config))
}
