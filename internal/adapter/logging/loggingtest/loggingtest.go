// Package loggingtest は既定のロガーに出たログを捕捉するテストヘルパを提供する。
package loggingtest

import (
	"context"
	"log/slog"
	"slices"
	"sync"
	"testing"
)

// Record は捕捉した 1 件のログである。
type Record struct {
	Level   slog.Level
	Message string
	Attrs   map[string]slog.Value
}

// String はログの項目 key の値を文字列で返す。
func (r Record) String(key string) string {
	return r.Attrs[key].String()
}

// Any はログの項目 key の値を any 型で返す。
func (r Record) Any(key string) any {
	return r.Attrs[key].Any()
}

// Logs は既定のロガーに出たログの捕捉結果である。
type Logs struct {
	mu      sync.Mutex
	records []Record
}

// Capture は既定のロガーを捕捉用に差し替え、テストの終了時に元へ戻す。
func Capture(t *testing.T) *Logs {
	t.Helper()
	logs := &Logs{}
	previous := slog.Default()
	slog.SetDefault(slog.New(&captureHandler{logs: logs}))
	t.Cleanup(func() { slog.SetDefault(previous) })
	return logs
}

// Records は捕捉したすべてのログを出力順に返す。
func (l *Logs) Records() []Record {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]Record(nil), l.records...)
}

// SelectByLevel はレベルが level のログを出力順に返す。
func (l *Logs) SelectByLevel(level slog.Level) []Record {
	return slices.DeleteFunc(l.Records(), func(record Record) bool {
		return record.Level != level
	})
}

// SelectAtLeastLevel はレベルが level 以上のログを出力順に返す。
func (l *Logs) SelectAtLeastLevel(level slog.Level) []Record {
	return slices.DeleteFunc(l.Records(), func(record Record) bool {
		return record.Level < level
	})
}

// SelectByMessageAndLevel はメッセージが message でレベルが level のログを出力順に返す。
func (l *Logs) SelectByMessageAndLevel(message string, level slog.Level) []Record {
	return slices.DeleteFunc(l.Records(), func(record Record) bool {
		return record.Message != message || record.Level != level
	})
}

func (l *Logs) append(record Record) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.records = append(l.records, record)
}

type captureHandler struct {
	logs  *Logs
	attrs []slog.Attr
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *captureHandler) Handle(_ context.Context, record slog.Record) error {
	attrs := make(map[string]slog.Value)
	for _, attr := range h.attrs {
		attrs[attr.Key] = attr.Value.Resolve()
	}
	record.Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attr.Value.Resolve()
		return true
	})
	h.logs.append(Record{Level: record.Level, Message: record.Message, Attrs: attrs})
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &captureHandler{logs: h.logs, attrs: append(append([]slog.Attr(nil), h.attrs...), attrs...)}
}

func (h *captureHandler) WithGroup(string) slog.Handler {
	return h
}
