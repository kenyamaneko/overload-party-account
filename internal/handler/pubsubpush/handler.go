package pubsubpush

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	malformedEnvelopeMessage = "malformed push envelope"
	undecodableDataMessage   = "undecodable message data"
	handlerFailedMessage     = "handler failed"
)

var (
	errEnvelopeNotJSON        = errors.New("envelope is not valid JSON")
	errPayloadNotJSON         = errors.New("event payload is not valid JSON")
	errPayloadTypeMismatch    = errors.New("event payload type mismatch")
	errPayloadDateTimeInvalid = errors.New("event payload date-time is invalid")
	errPayloadUnreadable      = errors.New("event payload could not be read for an unexpected reason")
)

var dateTimeType = reflect.TypeFor[time.Time]()

// pushEnvelope は Pub/Sub の push 購読が送る本文のうち、受け口が読む項目だけを持つ。
type pushEnvelope struct {
	Message struct {
		Data      string `json:"data"`
		MessageID string `json:"messageId"`
	} `json:"message"`
}

// EventHandler は1つの購読の push 受け口である。
// 受信したメッセージを型 E のイベントに変換して渡し、処理の結果を応答にする。
type EventHandler[E any] struct {
	subscription string
	handle       func(ctx context.Context, event E) error
	dateTimeKeys []string
}

// NewEventHandler は購読 subscription の push 受け口を生成する。
// subscription はログの文面に使う購読の名前である。
func NewEventHandler[E any](subscription string, handle func(ctx context.Context, event E) error) *EventHandler[E] {
	return &EventHandler[E]{
		subscription: subscription,
		handle:       handle,
		dateTimeKeys: listDateTimeKeys(reflect.TypeFor[E]()),
	}
}

// ReceivePush は push されたメッセージを受け取り、イベントの処理結果に応じた応答を返す。
// 本文を読めないときは 400、イベントの変換や処理に失敗したときは再配信の対象になる 500 を返す。
func (h *EventHandler[E]) ReceivePush(c *gin.Context) {
	ctx := c.Request.Context()

	var envelope pushEnvelope
	if err := c.ShouldBindJSON(&envelope); err != nil {
		slog.ErrorContext(ctx, "pubsub push: malformed envelope", "path", c.FullPath(),
			"error", fmt.Errorf("%w: %w", errEnvelopeNotJSON, err))
		c.JSON(http.StatusBadRequest, gin.H{"error": malformedEnvelopeMessage})
		return
	}
	if envelope.Message.Data == "" {
		slog.ErrorContext(ctx, "pubsub push: malformed envelope", "path", c.FullPath(), "error", "missing message.data")
		c.JSON(http.StatusBadRequest, gin.H{"error": malformedEnvelopeMessage})
		return
	}

	data, err := base64.StdEncoding.DecodeString(envelope.Message.Data)
	if err != nil {
		slog.ErrorContext(ctx, "pubsub push: undecodable message data",
			"path", c.FullPath(), "message_id", envelope.Message.MessageID, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": undecodableDataMessage})
		return
	}

	var event E
	if err := json.Unmarshal(data, &event); err != nil {
		slog.ErrorContext(ctx, h.subscription+" subscriber: bad payload", "error", h.classifyPayloadError(data, err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": handlerFailedMessage})
		return
	}
	if err := h.handle(ctx, event); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": handlerFailedMessage})
		return
	}
	c.Status(http.StatusOK)
}

// classifyPayloadError はイベント本文 data を読めなかった原因 err を、このパッケージが決めた文面のエラーに分類する。
func (h *EventHandler[E]) classifyPayloadError(data []byte, err error) error {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	dateTimeErr := h.classifyDateTimeError(data)
	switch {
	case errors.As(err, &syntaxErr):
		return fmt.Errorf("%w: %w", errPayloadNotJSON, err)
	case errors.As(err, &typeErr) && typeErr.Field != "":
		// 埋め込み型を経由した項目は Field の先頭に型名が付くため、JSON の項目名だけを取り出す。
		jsonKey := typeErr.Field[strings.LastIndex(typeErr.Field, ".")+1:]
		return fmt.Errorf("%w: %s: %w", errPayloadTypeMismatch, jsonKey, err)
	case dateTimeErr != nil:
		return dateTimeErr
	default:
		return fmt.Errorf("%w: %w", errPayloadUnreadable, err)
	}
}

// classifyDateTimeError は本文 data の日時の項目のうち、契約の型 (文字列) か形式 (date-time) に合わない最初の項目を、項目名つきのエラーに分類する。
// 該当する項目が無いときは nil を返す。
func (h *EventHandler[E]) classifyDateTimeError(data []byte) error {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return nil
	}
	// time.Time の読み取りエラーは JSON の項目名を持たないため、項目ごとに読み直して原因の項目を特定する。
	for _, key := range h.dateTimeKeys {
		rawValue, ok := fields[key]
		if !ok {
			continue
		}
		var text string
		if err := json.Unmarshal(rawValue, &text); err != nil {
			return fmt.Errorf("%w: %s: %w", errPayloadTypeMismatch, key, err)
		}
		var dateTime time.Time
		if err := json.Unmarshal(rawValue, &dateTime); err != nil {
			return fmt.Errorf("%w: %s: %w", errPayloadDateTimeInvalid, key, err)
		}
	}
	return nil
}

// listDateTimeKeys は型 t が JSON で受ける日時の項目の名前を、埋め込み型の項目も含めて返す。
func listDateTimeKeys(t reflect.Type) []string {
	if t.Kind() != reflect.Struct {
		return nil
	}
	var keys []string
	for i := range t.NumField() {
		field := t.Field(i)
		if !field.IsExported() {
			continue
		}
		tag := field.Tag.Get("json")
		if field.Anonymous && tag == "" {
			keys = append(keys, listDateTimeKeys(field.Type)...)
			continue
		}
		if field.Type != dateTimeType && field.Type != reflect.PointerTo(dateTimeType) {
			continue
		}
		name, _, _ := strings.Cut(tag, ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		keys = append(keys, name)
	}
	return keys
}
