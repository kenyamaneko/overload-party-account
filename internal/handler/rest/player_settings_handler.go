package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// PlayerSettingsHandler はプレイヤー設定の REST エンドポイントを処理します。
type PlayerSettingsHandler struct {
	svc *usecase.PlayerSettingsInteractor
}

// NewPlayerSettingsHandler は PlayerSettingsHandler を生成します。
func NewPlayerSettingsHandler(svc *usecase.PlayerSettingsInteractor) *PlayerSettingsHandler {
	return &PlayerSettingsHandler{svc: svc}
}

// GetSettings はプレイヤーの設定を返します。
func (h *PlayerSettingsHandler) GetSettings(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "playerId is required"})
		return
	}

	s, err := h.svc.Get(c.Request.Context(), playerID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

// UpdateSettings はプレイヤーの設定を部分更新します。
// PUT ですが partial update 契約（nil フィールドは現状維持）。
// 1 つもフィールドが指定されていないリクエストは 400 で弾きます。
func (h *PlayerSettingsHandler) UpdateSettings(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "playerId is required"})
		return
	}

	var req apiaccount.UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	patch := &port.PlayerSettingsPatch{
		Language:    req.Language,
		BgmVolume:   req.BgmVolume,
		SeVolume:    req.SeVolume,
		PushEnabled: req.PushEnabled,
	}
	if patch.IsEmpty() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one settings field is required"})
		return
	}

	if err := h.svc.Update(c.Request.Context(), playerID, patch); err != nil {
		respondError(c, err)
		return
	}

	updated, err := h.svc.Get(c.Request.Context(), playerID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}
