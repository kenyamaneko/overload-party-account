package pubsubpush

import (
	"encoding/base64"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// pushEnvelope は Pub/Sub push subscription が送る HTTP body のうち、
// 受け口が実際に読むフィールドだけを持つ。
type pushEnvelope struct {
	Message struct {
		Data      string `json:"data"`
		MessageID string `json:"messageId"`
	} `json:"message"`
}

// EventHandler は 1 イベント種別ぶんの push 受け口を組み立てる。
// envelope の decode と応答コードへの変換を共通化し、イベントごとの処理は
// port.MessageHandler の差し替えで表現する。
type EventHandler struct {
	handle port.MessageHandler
}

// NewEventHandler は handle を処理本体とする push 受け口を返す。
func NewEventHandler(handle port.MessageHandler) *EventHandler {
	return &EventHandler{handle: handle}
}

// Handle は push envelope を decode して handle に渡し、結果を応答コードへ変換する。
// envelope 自体が壊れている場合は handle を呼ばず 400 を返す。handle が返すエラーは
// 2xx / 5xx をそのまま呼び出し元 (Pub/Sub) への ack / 再配信の判断材料にする。
func (h *EventHandler) Handle(c *gin.Context) {
	var req pushEnvelope
	if err := c.ShouldBindJSON(&req); err != nil {
		slog.ErrorContext(c.Request.Context(), "pubsub push: malformed envelope",
			"path", c.FullPath(), "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "malformed push envelope"})
		return
	}

	data, err := base64.StdEncoding.DecodeString(req.Message.Data)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "pubsub push: undecodable message data",
			"path", c.FullPath(), "message_id", req.Message.MessageID, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "undecodable message data"})
		return
	}

	if err := h.handle(c.Request.Context(), data); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "handler failed"})
		return
	}
	c.Status(http.StatusOK)
}
