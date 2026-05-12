package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"

	internalauth "github.com/kenyamaneko/overload-party-gateway/packages/internalauth-go"
)

// FactionHandler は初期ファクション選択フローの REST エンドポイントを処理する。
type FactionHandler struct {
	factionInteractor *usecase.FactionInteractor
}

// NewFactionHandler は FactionHandler を生成する。
func NewFactionHandler(factionInteractor *usecase.FactionInteractor) *FactionHandler {
	return &FactionHandler{factionInteractor: factionInteractor}
}

// SelectInitialFaction は初期ファクション選択を処理する。
func (h *FactionHandler) SelectInitialFaction(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)

	var req apiaccount.SelectInitialFactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.FactionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "faction_id is required"})
		return
	}

	if err := h.factionInteractor.SelectInitialFaction(c.Request.Context(), playerID, req.FactionID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusOK)
}

// GrantFaction はプレイヤーにファクションを付与する。
func (h *FactionHandler) GrantFaction(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)
	var req apiaccount.FactionGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Faction == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "faction is required"})
		return
	}
	if err := h.factionInteractor.GrantFaction(c.Request.Context(), playerID, req.Faction); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListFactions はプレイヤーの所持ファクション一覧を返す。
func (h *FactionHandler) ListFactions(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)
	factions, err := h.factionInteractor.ListFactions(c.Request.Context(), playerID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, apiaccount.ListFactionsResponse{Factions: factions})
}
