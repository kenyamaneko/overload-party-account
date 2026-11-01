package pubsubpush_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/logging/loggingtest"
	"github.com/kenyamaneko/overload-party-account/internal/handler/handlertest"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush/pubsubpushtest"
)

const receivePath = "/internal/v1/pubsub/faction-acquired"

const validEventBody = `{"event_type":"faction_acquired","event_id":"3f2b8c1e-5a4d-4e6f-9b7a-1c2d3e4f5a6b","timestamp":"2026-01-01T00:00:00Z","player_id":"7d9e1a2b-3c4d-4e5f-8a6b-0c1d2e3f4a5b","faction":"Tenki"}`

func init() {
	gin.SetMode(gin.TestMode)
}

func newReceiveEngine(handle func(context.Context, apishop.FactionAcquiredEvent) error) *gin.Engine {
	engine := gin.New()
	engine.POST(receivePath, pubsubpush.NewEventHandler("faction-acquired", handle).ReceivePush)
	return engine
}

func succeeds(context.Context, apishop.FactionAcquiredEvent) error { return nil }

func fails(context.Context, apishop.FactionAcquiredEvent) error { return errors.New("handler failure") }

const messageID = "msg-1"

func postBody(t *testing.T, engine *gin.Engine, body string) *httptest.ResponseRecorder {
	t.Helper()
	return handlertest.PostJSON(t, engine, receivePath, body)
}

func validEnvelope(t *testing.T) string {
	t.Helper()
	return pubsubpushtest.BuildEventEnvelope(t, validEventBody, messageID)
}

type receiveCheck func(t *testing.T, rec *httptest.ResponseRecorder, logs *loggingtest.Logs)

func respondsWith(status int, wantError string) receiveCheck {
	return func(t *testing.T, rec *httptest.ResponseRecorder, _ *loggingtest.Logs) {
		t.Helper()
		assert.Equal(t, status, rec.Code)
		assert.Equal(t, wantError, handlertest.DecodeErrorMessage(t, rec))
	}
}

func logsError(wantMessage string) receiveCheck {
	return func(t *testing.T, _ *httptest.ResponseRecorder, logs *loggingtest.Logs) {
		t.Helper()
		assert.NotEmpty(t, logs.SelectByMessageAndLevel(wantMessage, slog.LevelError))
	}
}

func logsErrorWithMessageID(wantMessage, wantMessageID string) receiveCheck {
	return func(t *testing.T, _ *httptest.ResponseRecorder, logs *loggingtest.Logs) {
		t.Helper()
		records := logs.SelectByMessageAndLevel(wantMessage, slog.LevelError)
		require.NotEmpty(t, records)
		assert.Equal(t, wantMessageID, records[0].String("message_id"))
	}
}

func logsErrorContaining(wantMessage, wantText string) receiveCheck {
	return func(t *testing.T, _ *httptest.ResponseRecorder, logs *loggingtest.Logs) {
		t.Helper()
		records := logs.SelectByMessageAndLevel(wantMessage, slog.LevelError)
		require.NotEmpty(t, records)
		assert.Contains(t, records[0].String("error"), wantText)
	}
}

func TestReceivePush(t *testing.T) {
	t.Run("[イベント受信]受信メッセージの応答", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("メッセージを受信すると", func(t *testing.T) {
				extraFields := map[string]any{
					"subscription": "projects/test-project/subscriptions/test-subscription",
					"attributes":   map[string]any{"key": "value"},
				}
				cases := []struct {
					name string
					body string
				}{
					{"受信したイベントの処理が成功したとき、応答は200になる", validEnvelope(t)},
					{"受信本文にsubscriptionとattributesの項目があるとき、応答は200になる", pubsubpushtest.BuildEventEnvelopeWithFields(t, validEventBody, messageID, extraFields)},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						rec := postBody(t, newReceiveEngine(succeeds), tc.body)

						assert.Equal(t, http.StatusOK, rec.Code)
					})
				}
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("受信したイベントの処理が失敗したとき、処理失敗を示す500の応答になる", func(t *testing.T) {
				rec := postBody(t, newReceiveEngine(fails), validEnvelope(t))

				assert.Equal(t, http.StatusInternalServerError, rec.Code)
				assert.Equal(t, "handler failed", handlertest.DecodeErrorMessage(t, rec))
			})

			t.Run("受信本文が不正なとき", func(t *testing.T) {
				const (
					malformedEnvelopeMessage = "pubsub push: malformed envelope"
					undecodableBody          = `{"message":{"data":"!!!","messageId":"msg-1"}}`
					missingMessageBody       = `{"subscription":"projects/test-project/subscriptions/test-subscription"}`
					missingDataBody          = `{"message":{"messageId":"msg-1"}}`
					emptyDataBody            = `{"message":{"data":"","messageId":"msg-1"}}`
				)
				cases := []struct {
					name  string
					body  string
					check receiveCheck
				}{
					{"受信本文が空のとき、受信本文不正を示す400の応答になる", "", respondsWith(http.StatusBadRequest, "malformed push envelope")},
					{"受信本文がJSONでない「not-json」のとき、受信本文不正を示す400の応答になる", "not-json", respondsWith(http.StatusBadRequest, "malformed push envelope")},
					{"messageが無いとき、受信本文不正を示す400の応答になる", missingMessageBody, respondsWith(http.StatusBadRequest, "malformed push envelope")},
					{"message.dataが無いとき、受信本文不正を示す400の応答になる", missingDataBody, respondsWith(http.StatusBadRequest, "malformed push envelope")},
					{"message.dataが空文字のとき、受信本文不正を示す400の応答になる", emptyDataBody, respondsWith(http.StatusBadRequest, "malformed push envelope")},
					{"message.dataがbase64として復号できない「!!!」のとき、復号不能を示す400の応答になる", undecodableBody, respondsWith(http.StatusBadRequest, "undecodable message data")},
					{"受信本文がJSONでない「not-json」のとき、ERRORログに「受信本文不正」が記録される", "not-json", logsError(malformedEnvelopeMessage)},
					{"message.dataがbase64として復号できない「!!!」のとき、ERRORログに「復号不能」が記録される", undecodableBody, logsError("pubsub push: undecodable message data")},
					{"message.dataがbase64として復号できない「!!!」で、messageIdが「msg-1」のとき、ERRORログにメッセージIDとして「msg-1」が記録される", undecodableBody, logsErrorWithMessageID("pubsub push: undecodable message data", "msg-1")},
					{"受信本文が空のとき、ERRORログのエラーの内容は、受信本文がJSONとして読めないことを示す", "", logsErrorContaining(malformedEnvelopeMessage, "envelope is not valid JSON")},
					{"受信本文がJSONでない「not-json」のとき、ERRORログのエラーの内容は、受信本文がJSONとして読めないことを示す", "not-json", logsErrorContaining(malformedEnvelopeMessage, "envelope is not valid JSON")},
					{"messageが無いとき、ERRORログのエラーの内容は、message.dataが無いことを示す", missingMessageBody, logsErrorContaining(malformedEnvelopeMessage, "missing message.data")},
					{"message.dataが無いとき、ERRORログのエラーの内容は、message.dataが無いことを示す", missingDataBody, logsErrorContaining(malformedEnvelopeMessage, "missing message.data")},
					{"message.dataが空文字のとき、ERRORログのエラーの内容は、message.dataが無いことを示す", emptyDataBody, logsErrorContaining(malformedEnvelopeMessage, "missing message.data")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						logs := loggingtest.Capture(t)

						rec := postBody(t, newReceiveEngine(succeeds), tc.body)

						tc.check(t, rec, logs)
					})
				}
			})
		})
	})
}
