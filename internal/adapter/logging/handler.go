package logging

import (
	"fmt"
	"io"
	"log/slog"
)

const (
	cloudLoggingSeverityKey = "severity"
	cloudLoggingMessageKey  = "message"
)

// NewCloudLoggingHandler は、Cloud Logging が読める項目名 (severity・message) の JSON を w へ書き出すハンドラを返す。
func NewCloudLoggingHandler(w io.Writer) slog.Handler {
	return slog.NewJSONHandler(w, &slog.HandlerOptions{ReplaceAttr: replaceCloudLoggingAttr})
}

// NewLocalHandler は、Debug 以上のログを人間が読めるテキストで w へ書き出すハンドラを返す。
func NewLocalHandler(w io.Writer) slog.Handler {
	return slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug})
}

func replaceCloudLoggingAttr(_ []string, a slog.Attr) slog.Attr {
	// slog の既定の項目名 (level / msg) は Cloud Logging が認識しないため、severity / message に変換する。
	switch a.Key {
	case slog.LevelKey:
		a.Key = cloudLoggingSeverityKey
		if level, ok := a.Value.Any().(slog.Level); ok {
			a.Value = slog.StringValue(toCloudLoggingSeverity(level))
		}
	case slog.MessageKey:
		a.Key = cloudLoggingMessageKey
	}
	return a
}

func toCloudLoggingSeverity(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "ERROR"
	case level >= slog.LevelWarn:
		return "WARNING"
	case level >= slog.LevelInfo:
		return "INFO"
	default:
		panic(fmt.Sprintf("unexpected log level: %v", level))
	}
}
