package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/service"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// FactionHandler は初期ファクション選択フローの REST エンドポイントを処理します。
type FactionHandler struct {
	factionService *service.FactionService
}

// NewFactionHandler は FactionHandler を生成します。
func NewFactionHandler(factionService *service.FactionService) *FactionHandler {
	return &FactionHandler{factionService: factionService}
}

// SelectInitialFaction は初期ファクション選択を処理します。
// 200=成功、409=選択済み（冪等）、400=不正ファクション、404=プレイヤー不在。
func (h *FactionHandler) SelectInitialFaction(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "playerId is required"})
		return
	}

	var req apiaccount.SelectInitialFactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.FactionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "faction_id is required"})
		return
	}
	// 将来の source 拡張（admin override 等）に備えてリクエストに含めるが、
	// 現時点では initial_selection のみ受け付ける。
	if req.Source != "" && req.Source != service.FactionSourceInitialSelection {
		c.JSON(http.StatusBadRequest, gin.H{"error": "source must be \"initial_selection\""})
		return
	}

	if err := h.factionService.SelectInitialFaction(c.Request.Context(), playerID, req.FactionID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusOK)
}
