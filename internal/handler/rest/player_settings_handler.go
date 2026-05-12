package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"

	internalauth "github.com/kenyamaneko/overload-party-gateway/packages/internalauth-go"
)

// PlayerSettingsHandler はプレイヤー設定の REST エンドポイントを処理する。
type PlayerSettingsHandler struct {
	settingsInteractor *usecase.PlayerSettingsInteractor
}

// NewPlayerSettingsHandler は PlayerSettingsHandler を生成する。
func NewPlayerSettingsHandler(settingsInteractor *usecase.PlayerSettingsInteractor) *PlayerSettingsHandler {
	return &PlayerSettingsHandler{settingsInteractor: settingsInteractor}
}

// GetSettings はプレイヤーの設定を返す。
func (h *PlayerSettingsHandler) GetSettings(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)

	s, err := h.settingsInteractor.Get(c.Request.Context(), playerID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}

// UpdateSettings はプレイヤーの設定を部分更新する。
// PUT だが partial update 契約 (nil フィールドは現状維持)。全 nil リクエストは 400 で弾く。
func (h *PlayerSettingsHandler) UpdateSettings(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)

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

	if err := h.settingsInteractor.Update(c.Request.Context(), playerID, patch); err != nil {
		respondError(c, err)
		return
	}

	updated, err := h.settingsInteractor.Get(c.Request.Context(), playerID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}
