package pubsubpush

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// recordingHandler は EventHandler.Handle から渡された payload と呼び出し回数を
// 記録する port.MessageHandler の stub。
type recordingHandler struct {
	calls   int
	gotData []byte
	err     error
}

func (h *recordingHandler) handle(_ context.Context, data []byte) error {
	h.calls++
	h.gotData = data
	return h.err
}

func newTestEngine(h *EventHandler) *gin.Engine {
	r := gin.New()
	r.POST("/push", h.Handle)
	return r
}

func doPush(t *testing.T, r *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/push", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestEventHandlerHandle(t *testing.T) {
	t.Run("push envelopeのdecodeと応答コードへの変換", func(t *testing.T) {
		t.Run("正しいenvelopeを受けたとき、message.dataをbase64復号してイベント処理に渡し200になる", func(t *testing.T) {
			rec := &recordingHandler{}
			r := newTestEngine(NewEventHandler(rec.handle))

			payload := base64.StdEncoding.EncodeToString([]byte(`{"event_type":"faction_acquired"}`))
			w := doPush(t, r, `{"message":{"data":"`+payload+`","messageId":"m-1"},"subscription":"sub-1"}`)

			require.Equal(t, http.StatusOK, w.Code)
			require.Equal(t, 1, rec.calls)
			assert.Equal(t, `{"event_type":"faction_acquired"}`, string(rec.gotData))
		})

		t.Run("JSONとして壊れた本文を受けたとき、イベント処理を呼ばず400になり、応答本文に \"malformed push envelope\" が含まれる", func(t *testing.T) {
			rec := &recordingHandler{}
			r := newTestEngine(NewEventHandler(rec.handle))

			w := doPush(t, r, `{not-json`)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "malformed push envelope")
			assert.Zero(t, rec.calls, "envelope が壊れているときはイベント処理に到達しない")
		})

		t.Run("message.dataがbase64として復号できない本文を受けたとき、イベント処理を呼ばず400になり、応答本文に \"undecodable message data\" が含まれる", func(t *testing.T) {
			rec := &recordingHandler{}
			r := newTestEngine(NewEventHandler(rec.handle))

			w := doPush(t, r, `{"message":{"data":"not-valid-base64!!","messageId":"m-2"}}`)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "undecodable message data")
			assert.Zero(t, rec.calls, "data を復号できないときはイベント処理に到達しない")
		})

		t.Run("messageフィールドが無い本文を受けたとき、イベント処理を呼ばず400になり、応答本文に \"malformed push envelope\" が含まれる", func(t *testing.T) {
			rec := &recordingHandler{}
			r := newTestEngine(NewEventHandler(rec.handle))

			w := doPush(t, r, `{"subscription":"sub-1"}`)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "malformed push envelope")
			assert.Zero(t, rec.calls, "message フィールドが無いときはイベント処理に到達しない")
		})

		t.Run("message.dataが空文字の本文を受けたとき、イベント処理を呼ばず400になり、応答本文に \"malformed push envelope\" が含まれる", func(t *testing.T) {
			rec := &recordingHandler{}
			r := newTestEngine(NewEventHandler(rec.handle))

			w := doPush(t, r, `{"message":{"data":"","messageId":"m-4"}}`)

			assert.Equal(t, http.StatusBadRequest, w.Code)
			assert.Contains(t, w.Body.String(), "malformed push envelope")
			assert.Zero(t, rec.calls, "message.data が空文字のときはイベント処理に到達しない")
		})

		t.Run("イベント処理がエラーを返すとき、500になる", func(t *testing.T) {
			rec := &recordingHandler{err: errors.New("db unavailable")}
			r := newTestEngine(NewEventHandler(rec.handle))

			payload := base64.StdEncoding.EncodeToString([]byte(`{"event_type":"faction_acquired"}`))
			w := doPush(t, r, `{"message":{"data":"`+payload+`","messageId":"m-3"}}`)

			assert.Equal(t, http.StatusInternalServerError, w.Code)
			assert.Equal(t, 1, rec.calls, "イベント処理には到達しているが失敗する")
		})
	})
}
