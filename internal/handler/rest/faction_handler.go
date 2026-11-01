package rest

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"

	internalauth "github.com/kenyamaneko/overload-party-gateway/packages/internalauth-go"
)

// FactionHandler は陣営の選択・付与・一覧の REST の受け口である。
type FactionHandler struct {
	factionInteractor *usecase.FactionInteractor
}

// NewFactionHandler は FactionHandler を生成する。
func NewFactionHandler(factionInteractor *usecase.FactionInteractor) *FactionHandler {
	return &FactionHandler{factionInteractor: factionInteractor}
}

// SelectInitialFaction は初期陣営を選択する。
func (h *FactionHandler) SelectInitialFaction(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)

	var req apiaccount.SelectInitialFactionRequest
	if !bindRequestBody(c, &req) || respondIfEmpty(c, "faction_id", req.FactionID) {
		return
	}

	if err := h.factionInteractor.SelectInitialFaction(c.Request.Context(), playerID, req.FactionID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusOK)
}

// GrantFaction は陣営を所持陣営に加える。
func (h *FactionHandler) GrantFaction(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)

	var req apiaccount.FactionGrantRequest
	if !bindRequestBody(c, &req) || respondIfEmpty(c, "faction", req.Faction) {
		return
	}

	if err := h.factionInteractor.GrantFaction(c.Request.Context(), playerID, req.Faction); err != nil {
		if errors.Is(err, usecase.ErrInvalidFaction) {
			respondBadRequest(c, invalidFieldMessage("faction"))
			return
		}
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListFactions は認証済みプレイヤーの所持陣営の一覧を返す。
func (h *FactionHandler) ListFactions(c *gin.Context) {
	h.respondFactionListing(c, c.GetString(internalauth.PlayerIDContextKey))
}

// ListPlayerFactions はパスで指定されたプレイヤーの所持陣営の一覧を返す。
func (h *FactionHandler) ListPlayerFactions(c *gin.Context) {
	playerID, isValid := parsePlayerID(c.Param("playerID"))
	if !isValid {
		respondBadRequest(c, invalidPlayerIDMessage)
		return
	}
	h.respondFactionListing(c, playerID)
}

// respondFactionListing は playerID の所持陣営の一覧を応答する。
func (h *FactionHandler) respondFactionListing(c *gin.Context, playerID string) {
	factions, err := h.factionInteractor.ListFactions(c.Request.Context(), playerID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, apiaccount.FactionListing{Factions: factions})
}
