// Package resttest は REST ハンドラのテストが共有する、エラー応答とログを確かめる部品を提供する。
package resttest

import (
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/logging/loggingtest"
	"github.com/kenyamaneko/overload-party-account/internal/handler/handlertest"
)

// AssertErrorResponseWithPrefix は応答のステータスコードが status で、error 項目が prefix で始まることを確かめる。
func AssertErrorResponseWithPrefix(t *testing.T, rec *httptest.ResponseRecorder, status int, prefix string) {
	t.Helper()
	assert.Equal(t, status, rec.Code)
	message := handlertest.DecodeErrorMessage(t, rec)
	assert.True(t, strings.HasPrefix(message, prefix), "error %q does not start with %q", message, prefix)
}

// SelectLoggedError は記録されたエラーレベル以上のログが 1 件であることを確かめて、そのログを返す。
func SelectLoggedError(t *testing.T, logs *loggingtest.Logs) loggingtest.Record {
	t.Helper()
	records := logs.SelectAtLeastLevel(slog.LevelError)
	require.Len(t, records, 1)
	return records[0]
}
