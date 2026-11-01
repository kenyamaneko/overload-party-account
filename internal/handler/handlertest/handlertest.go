// Package handlertest は HTTP の応答を確かめるテストヘルパを提供する。
package handlertest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// Call は handler へ、メソッド method・パス path・ヘッダ headers・JSON の本文 body のリクエストを送り、応答を返す。
func Call(t *testing.T, handler http.Handler, method, path string, headers http.Header, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	for name, values := range headers {
		req.Header[name] = values
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// PostJSON は handler へ、パス path に JSON の本文 body を POST し、応答を返す。
func PostJSON(t *testing.T, handler http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return Call(t, handler, http.MethodPost, path, nil, body)
}

// DecodeObject は応答本文の JSON オブジェクトを、項目名ごとの JSON に分けて返す。
func DecodeObject(t *testing.T, rec *httptest.ResponseRecorder) map[string]json.RawMessage {
	t.Helper()
	var object map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &object))
	return object
}

// DecodeErrorMessage はエラー応答の本文の error 項目を返す。
func DecodeErrorMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body apiaccount.ErrorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	return body.Error
}

// AssertStatus は応答のステータスコードが status であることを確かめる。
func AssertStatus(t *testing.T, rec *httptest.ResponseRecorder, status int) {
	t.Helper()
	assert.Equal(t, status, rec.Code)
}

// AssertJSONField は応答本文の項目 field の JSON が expectedJSON と等しいことを確かめる。
func AssertJSONField(t *testing.T, rec *httptest.ResponseRecorder, field, expectedJSON string) {
	t.Helper()
	assert.JSONEq(t, expectedJSON, string(DecodeObject(t, rec)[field]))
}

// AssertErrorResponse は応答のステータスコードが status で、error 項目が message であることを確かめる。
func AssertErrorResponse(t *testing.T, rec *httptest.ResponseRecorder, status int, message string) {
	t.Helper()
	assert.Equal(t, status, rec.Code)
	assert.Equal(t, message, DecodeErrorMessage(t, rec))
}
