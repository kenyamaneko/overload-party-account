package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/presenter"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"

	internalauth "github.com/kenyamaneko/overload-party-gateway/packages/internalauth-go"
)

const (
	minVolume = 0
	maxVolume = 100
)

// PlayerSettingsHandler はプレイヤー設定の REST の受け口である。
type PlayerSettingsHandler struct {
	settingsInteractor *usecase.PlayerSettingsInteractor
}

// NewPlayerSettingsHandler は PlayerSettingsHandler を生成する。
func NewPlayerSettingsHandler(settingsInteractor *usecase.PlayerSettingsInteractor) *PlayerSettingsHandler {
	return &PlayerSettingsHandler{settingsInteractor: settingsInteractor}
}

// GetSettings は認証済みプレイヤーのプレイヤー設定を返す。
func (h *PlayerSettingsHandler) GetSettings(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)

	settings, err := h.settingsInteractor.GetPlayerSettings(c.Request.Context(), playerID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, presenter.ToPlayerSettingsResponse(settings))
}

// UpdateSettings は認証済みプレイヤーのプレイヤー設定のうち、指定された項目だけを更新し、更新後の設定を返す。
func (h *PlayerSettingsHandler) UpdateSettings(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)

	var req apiaccount.UpdateSettingsRequest
	if !bindRequestBody(c, &req) {
		return
	}

	patch := &port.PlayerSettingsPatch{
		Language:      req.Language,
		BgmVolume:     req.BgmVolume,
		SeVolume:      req.SeVolume,
		IsPushEnabled: req.PushEnabled,
	}
	if patch.IsEmpty() {
		respondBadRequest(c, "at least one settings field is required")
		return
	}
	if patch.Language != nil {
		if err := domain.ValidateLanguage(*patch.Language); err != nil {
			respondError(c, err)
			return
		}
	}
	if isVolumeOutOfRange(patch.BgmVolume) {
		respondBadRequest(c, invalidFieldMessage("bgm_volume"))
		return
	}
	if isVolumeOutOfRange(patch.SeVolume) {
		respondBadRequest(c, invalidFieldMessage("se_volume"))
		return
	}

	if err := h.settingsInteractor.UpdatePlayerSettings(c.Request.Context(), playerID, patch); err != nil {
		respondError(c, err)
		return
	}
	updated, err := h.settingsInteractor.GetPlayerSettings(c.Request.Context(), playerID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, presenter.ToPlayerSettingsResponse(updated))
}

// isVolumeOutOfRange は指定された音量が許される範囲の外かを返す。指定されていないときは false を返す。
func isVolumeOutOfRange(volume *int64) bool {
	return volume != nil && (*volume < minVolume || *volume > maxVolume)
}
