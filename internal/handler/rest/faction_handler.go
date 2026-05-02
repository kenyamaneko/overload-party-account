package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// FactionHandler は初期ファクション選択フローの REST エンドポイントを処理する。
type FactionHandler struct {
	factionService *usecase.FactionInteractor
}

// NewFactionHandler は FactionHandler を生成する。
func NewFactionHandler(factionService *usecase.FactionInteractor) *FactionHandler {
	return &FactionHandler{factionService: factionService}
}

// SelectInitialFaction は初期ファクション選択を処理する。
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

	if err := h.factionService.SelectInitialFaction(c.Request.Context(), playerID, req.FactionID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusOK)
}
