package pubsubpushtest

import (
	"encoding/json"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/logging/loggingtest"
	"github.com/kenyamaneko/overload-party-account/internal/handler/handlertest"
)

const (
	// PushPathFactionAcquired は陣営獲得イベントの push 受け口のパスである。
	PushPathFactionAcquired = "/internal/v1/pubsub/faction-acquired"
	// PushPathPremiumUpdated はプレミアム状態更新イベントの push 受け口のパスである。
	PushPathPremiumUpdated = "/internal/v1/pubsub/premium-updated"
	// PushPathPlayerOnboarded はオンボーディング完了イベントの push 受け口のパスである。
	PushPathPlayerOnboarded = "/internal/v1/pubsub/player-onboarded"
	// PushPathOnboardingNameSet はオンボーディング表示名設定イベントの push 受け口のパスである。
	PushPathOnboardingNameSet = "/internal/v1/pubsub/onboarding-name-set"
	// PushPathOnboardingFactionSet はオンボーディング陣営設定イベントの push 受け口のパスである。
	PushPathOnboardingFactionSet = "/internal/v1/pubsub/onboarding-faction-set"

	// DummyEventID は受信内容の検証で使うイベントIDのダミー値である。
	DummyEventID = "3f2b8c1e-5a4d-4e6f-9b7a-1c2d3e4f5a6b"
	// DummyPlayerID は受信内容の検証で使うプレイヤーIDのダミー値である。
	DummyPlayerID = "7d9e1a2b-3c4d-4e5f-8a6b-0c1d2e3f4a5b"

	dummyMessageID    = "msg-1"
	dummySubscription = "projects/test-project/subscriptions/test-subscription"
)

// PushCheck は push の応答と捕捉したログを確かめる。
type PushCheck func(t *testing.T, rec *httptest.ResponseRecorder, logs *loggingtest.Logs)

// NewPushEngine はパス path への POST を receive で受ける HTTP サーバを返す。
func NewPushEngine(path string, receive gin.HandlerFunc) *gin.Engine {
	engine := gin.New()
	engine.POST(path, receive)
	return engine
}

// PushEvent はイベント本文 eventBody を載せた push 受信本文を、engine のパス path へ送り、応答を返す。
func PushEvent(t *testing.T, engine *gin.Engine, path, eventBody string) *httptest.ResponseRecorder {
	t.Helper()
	envelope := BuildEventEnvelopeWithFields(t, eventBody, dummyMessageID, map[string]any{"subscription": dummySubscription})
	return handlertest.PostJSON(t, engine, path, envelope)
}

// MarshalJSON は value を JSON の文字列にして返す。
func MarshalJSON(t *testing.T, value map[string]any) string {
	t.Helper()
	encoded, err := json.Marshal(value)
	require.NoError(t, err)
	return string(encoded)
}

// MarshalJSONWith は base の項目 key を value にした JSON の文字列を返す。
func MarshalJSONWith(t *testing.T, base map[string]any, key string, value any) string {
	t.Helper()
	copied := maps.Clone(base)
	copied[key] = value
	return MarshalJSON(t, copied)
}

// MarshalJSONWithout は base から項目 key を除いた JSON の文字列を返す。
func MarshalJSONWithout(t *testing.T, base map[string]any, key string) string {
	t.Helper()
	copied := maps.Clone(base)
	delete(copied, key)
	return MarshalJSON(t, copied)
}

// AssertErrorLogged はメッセージが message の ERROR ログが記録されたことを確かめる。
func AssertErrorLogged(t *testing.T, logs *loggingtest.Logs, message string) {
	t.Helper()
	assert.NotEmpty(t, logs.SelectByMessageAndLevel(message, slog.LevelError), "ERROR log %q was not recorded; recorded: %v", message, logs.Records())
}

// AssertErrorLogAttr はメッセージが message の ERROR ログの項目 key が want であることを確かめる。
func AssertErrorLogAttr(t *testing.T, logs *loggingtest.Logs, message, key string, want any) {
	t.Helper()
	records := logs.SelectByMessageAndLevel(message, slog.LevelError)
	require.NotEmpty(t, records, "ERROR log %q was not recorded; recorded: %v", message, logs.Records())
	assert.Equal(t, want, records[0].Any(key))
}

// AssertAnyErrorLogAttr はいずれかの ERROR ログの項目 key が want であることを確かめる。
func AssertAnyErrorLogAttr(t *testing.T, logs *loggingtest.Logs, key string, want any) {
	t.Helper()
	records := logs.Records()
	assert.True(t, slices.ContainsFunc(records, func(r loggingtest.Record) bool {
		return r.Level == slog.LevelError && r.Any(key) == want
	}), "no ERROR log has %s=%v; recorded: %v", key, want, records)
}

// AssertAnyErrorLogContains はいずれかの ERROR ログの項目 key が wantText を含むことを確かめる。
func AssertAnyErrorLogContains(t *testing.T, logs *loggingtest.Logs, key, wantText string) {
	t.Helper()
	records := logs.Records()
	assert.True(t, slices.ContainsFunc(records, func(r loggingtest.Record) bool {
		return r.Level == slog.LevelError && strings.Contains(r.String(key), wantText)
	}), "no ERROR log has %s containing %q; recorded: %v", key, wantText, records)
}

// AssertErrorLogContains はメッセージが message の ERROR ログのエラーの内容が wantText を含むことを確かめる。
func AssertErrorLogContains(t *testing.T, logs *loggingtest.Logs, message, wantText string) {
	t.Helper()
	records := logs.SelectByMessageAndLevel(message, slog.LevelError)
	require.NotEmpty(t, records, "ERROR log %q was not recorded; recorded: %v", message, logs.Records())
	assert.Contains(t, records[0].String("error"), wantText)
}

// AssertHandlerFailed は応答が処理失敗を示す 500 であることを確かめる。
func AssertHandlerFailed(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "handler failed", handlertest.DecodeErrorMessage(t, rec))
}

// RespondsHandlerFailed は応答が処理失敗を示す 500 であることを確かめる PushCheck を返す。
func RespondsHandlerFailed() PushCheck {
	return func(t *testing.T, rec *httptest.ResponseRecorder, _ *loggingtest.Logs) {
		t.Helper()
		AssertHandlerFailed(t, rec)
	}
}

// LogsError はメッセージが message の ERROR ログが記録されたことを確かめる PushCheck を返す。
func LogsError(message string) PushCheck {
	return func(t *testing.T, _ *httptest.ResponseRecorder, logs *loggingtest.Logs) {
		t.Helper()
		AssertErrorLogged(t, logs, message)
	}
}

// LogsErrorAttr はメッセージが message の ERROR ログの項目 key が want であることを確かめる PushCheck を返す。
func LogsErrorAttr(message, key string, want any) PushCheck {
	return func(t *testing.T, _ *httptest.ResponseRecorder, logs *loggingtest.Logs) {
		t.Helper()
		AssertErrorLogAttr(t, logs, message, key, want)
	}
}

// LogsErrorContaining はメッセージが message の ERROR ログのエラーの内容が wantText を含むことを確かめる PushCheck を返す。
func LogsErrorContaining(message, wantText string) PushCheck {
	return func(t *testing.T, _ *httptest.ResponseRecorder, logs *loggingtest.Logs) {
		t.Helper()
		AssertErrorLogContains(t, logs, message, wantText)
	}
}
