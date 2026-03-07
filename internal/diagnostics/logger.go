package diagnostics

import (
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

type Level string

const (
	LevelError Level = "error"
	LevelWarn  Level = "warn"
	LevelInfo  Level = "info"
	LevelDebug Level = "debug"
)

type Format string

const (
	FormatText  Format = "text"
	FormatJSONL Format = "jsonl"
)

type Config struct {
	Level  Level
	Format Format
}

type Logger struct {
	level  Level
	format Format
	writer io.Writer
	mu     sync.Mutex
}

var (
	outputWriter   io.Writer = os.Stderr
	outputWriterMu sync.RWMutex
)

func SetOutputWriter(writer io.Writer) {
	if writer == nil {
		writer = io.Discard
	}

	outputWriterMu.Lock()
	defer outputWriterMu.Unlock()
	outputWriter = writer
}

func OutputWriter() io.Writer {
	outputWriterMu.RLock()
	defer outputWriterMu.RUnlock()
	return outputWriter
}

func ParseLevel(value string) (Level, error) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	switch Level(trimmed) {
	case LevelError, LevelWarn, LevelInfo, LevelDebug:
		return Level(trimmed), nil
	default:
		return "", fmt.Errorf("invalid log level %q", value)
	}
}

func ParseFormat(value string) (Format, error) {
	trimmed := strings.TrimSpace(strings.ToLower(value))
	switch Format(trimmed) {
	case FormatText, FormatJSONL:
		return Format(trimmed), nil
	default:
		return "", fmt.Errorf("invalid log format %q", value)
	}
}

func NewLogger(config Config, writer io.Writer) *Logger {
	if writer == nil {
		writer = io.Discard
	}

	level := config.Level
	if level == "" {
		level = LevelError
	}

	format := config.Format
	if format == "" {
		format = FormatText
	}

	return &Logger{level: level, format: format, writer: writer}
}

func (logger *Logger) Error(message string, fields map[string]any) {
	logger.log(LevelError, message, fields)
}

func (logger *Logger) Warn(message string, fields map[string]any) {
	logger.log(LevelWarn, message, fields)
}

func (logger *Logger) Info(message string, fields map[string]any) {
	logger.log(LevelInfo, message, fields)
}

func (logger *Logger) Debug(message string, fields map[string]any) {
	logger.log(LevelDebug, message, fields)
}

func (logger *Logger) log(level Level, message string, fields map[string]any) {
	if logger == nil || !logger.Enabled(level) {
		return
	}

	sanitized := RedactFields(fields)
	if sanitized == nil {
		sanitized = map[string]any{}
	}

	logger.mu.Lock()
	defer logger.mu.Unlock()

	timestamp := time.Now().UTC().Format(time.RFC3339Nano)
	if logger.format == FormatJSONL {
		event := map[string]any{
			"timestamp": timestamp,
			"level":     level,
			"message":   message,
		}
		for key, value := range sanitized {
			event[key] = value
		}
		encoded, err := json.Marshal(event)
		if err != nil {
			return
		}
		_, _ = fmt.Fprintln(logger.writer, string(encoded))
		return
	}

	keys := make([]string, 0, len(sanitized))
	for key := range sanitized {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	builder := strings.Builder{}
	builder.WriteString(timestamp)
	builder.WriteString(" level=")
	builder.WriteString(string(level))
	builder.WriteString(" msg=")
	builder.WriteString(strconvQuote(message))

	for _, key := range keys {
		builder.WriteString(" ")
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(strconvQuote(fmt.Sprintf("%v", sanitized[key])))
	}

	_, _ = fmt.Fprintln(logger.writer, builder.String())
}

func (logger *Logger) Enabled(level Level) bool {
	if logger == nil {
		return false
	}

	return levelRank(level) <= levelRank(logger.level)
}

func levelRank(level Level) int {
	switch level {
	case LevelError:
		return 0
	case LevelWarn:
		return 1
	case LevelInfo:
		return 2
	case LevelDebug:
		return 3
	default:
		return 0
	}
}

func RedactFields(fields map[string]any) map[string]any {
	if fields == nil {
		return nil
	}

	sanitized := make(map[string]any, len(fields))
	for key, value := range fields {
		sanitized[key] = redactValue(key, value)
	}

	return sanitized
}

func redactValue(key string, value any) any {
	if isSensitiveKey(key) {
		return "[REDACTED]"
	}

	switch typed := value.(type) {
	case string:
		return redactURLString(typed)
	case map[string]any:
		return RedactFields(typed)
	case map[string]string:
		converted := make(map[string]any, len(typed))
		for nestedKey, nestedValue := range typed {
			converted[nestedKey] = nestedValue
		}
		return RedactFields(converted)
	case []any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, redactValue("", item))
		}
		return items
	default:
		return value
	}
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	if normalized == "" {
		return false
	}

	sensitiveTokens := []string{"token", "password", "secret", "authorization", "cookie", "set-cookie", "apikey", "api-key", "credential"}
	for _, token := range sensitiveTokens {
		if strings.Contains(normalized, token) {
			return true
		}
	}

	return false
}

func redactURLString(value string) string {
	if !strings.Contains(value, "://") {
		return value
	}

	parsed, err := url.Parse(value)
	if err != nil {
		return value
	}

	if parsed.User != nil {
		username := parsed.User.Username()
		if username == "" {
			username = "redacted"
		}
		parsed.User = url.UserPassword(username, "[REDACTED]")
	}

	query := parsed.Query()
	for key := range query {
		if isSensitiveKey(key) {
			query.Set(key, "[REDACTED]")
		}
	}
	parsed.RawQuery = query.Encode()

	return parsed.String()
}

func strconvQuote(value string) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "\"\""
	}
	return string(encoded)
}
