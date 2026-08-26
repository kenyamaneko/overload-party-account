package pubsubpush_test

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// callHandle は EventHandler.Handle を gin.Context 経由で呼び出し、応答の *httptest.ResponseRecorder を返す。
func callHandle(t *testing.T, h *pubsubpush.EventHandler, body []byte) *httptest.ResponseRecorder {
	t.Helper()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest("POST", "/internal/v1/pubsub/dummy", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req

	h.Handle(c)
	return w
}

func envelopeJSON(t *testing.T, data string) []byte {
	t.Helper()
	env := map[string]any{
		"message": map[string]any{
			"data":      data,
			"messageId": "msg-1",
		},
	}
	b, err := json.Marshal(env)
	require.NoError(t, err)
	return b
}

func TestEventHandler_Handle(t *testing.T) {
	t.Run("push受け口の応答変換", func(t *testing.T) {
		t.Run("リクエストボディがJSONとして解析できないとき、ステータスコード400で、メッセージ形式が不正である旨のエラー内容を返し、処理関数は呼び出されない", func(t *testing.T) {
			called := false
			h := pubsubpush.NewEventHandler(func(ctx context.Context, data []byte) error { called = true; return nil })

			w := callHandle(t, h, []byte("not-json"))

			assert.Equal(t, 400, w.Code)
			assert.Contains(t, w.Body.String(), "malformed push envelope")
			assert.False(t, called, "envelopeの形式が不正なときは処理関数の呼び出しに到達しない")
		})

		t.Run("リクエストボディのデータ部分が空のとき、ステータスコード400で、メッセージ形式が不正である旨のエラー内容を返し、処理関数は呼び出されない", func(t *testing.T) {
			called := false
			h := pubsubpush.NewEventHandler(func(ctx context.Context, data []byte) error { called = true; return nil })

			w := callHandle(t, h, envelopeJSON(t, ""))

			assert.Equal(t, 400, w.Code)
			assert.Contains(t, w.Body.String(), "malformed push envelope")
			assert.False(t, called, "データ部分が空なときは処理関数の呼び出しに到達しない")
		})

		t.Run("データ部分がBase64として正しくデコードできない値のとき、ステータスコード400で、データ形式が不正である旨のエラー内容を返し、処理関数は呼び出されない", func(t *testing.T) {
			called := false
			h := pubsubpush.NewEventHandler(func(ctx context.Context, data []byte) error { called = true; return nil })

			w := callHandle(t, h, envelopeJSON(t, "not-valid-base64!!!"))

			assert.Equal(t, 400, w.Code)
			assert.Contains(t, w.Body.String(), "undecodable message data")
			assert.False(t, called, "データ部分をデコードできないときは処理関数の呼び出しに到達しない")
		})

		t.Run("データ部分が正しくBase64デコードできるとき、デコード結果を処理関数の引数として渡す", func(t *testing.T) {
			var received []byte
			h := pubsubpush.NewEventHandler(func(ctx context.Context, data []byte) error {
				received = data
				return nil
			})
			payload := []byte(`{"event_type":"faction_acquired"}`)

			w := callHandle(t, h, envelopeJSON(t, base64.StdEncoding.EncodeToString(payload)))

			assert.Equal(t, 200, w.Code)
			assert.Equal(t, payload, received)
		})

		t.Run("データ部分のデコードには成功したが、その後の処理関数の呼び出しが失敗したとき、ステータスコード500で、処理に失敗した旨のエラー内容を返す", func(t *testing.T) {
			h := pubsubpush.NewEventHandler(func(ctx context.Context, data []byte) error {
				return errors.New("boom")
			})

			w := callHandle(t, h, envelopeJSON(t, base64.StdEncoding.EncodeToString([]byte("{}"))))

			assert.Equal(t, 500, w.Code)
			assert.Contains(t, w.Body.String(), "handler failed")
		})

		t.Run("データ部分のデコードにも、その後の処理関数の呼び出しにも成功したとき、200を返す", func(t *testing.T) {
			h := pubsubpush.NewEventHandler(func(ctx context.Context, data []byte) error { return nil })

			w := callHandle(t, h, envelopeJSON(t, base64.StdEncoding.EncodeToString([]byte("{}"))))

			assert.Equal(t, 200, w.Code)
		})
	})
}
