package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/service"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// UserSettingsHandler はユーザー設定の REST エンドポイントを処理します。
type UserSettingsHandler struct {
	svc *service.UserSettingsService
}

// NewUserSettingsHandler は UserSettingsHandler を生成します。
func NewUserSettingsHandler(svc *service.UserSettingsService) *UserSettingsHandler {
	return &UserSettingsHandler{svc: svc}
}

// GetSettings はプレイヤーのユーザー設定を返します。
func (h *UserSettingsHandler) GetSettings(c *gin.Context) {
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

// UpdateSettings はプレイヤーのユーザー設定を更新します。
func (h *UserSettingsHandler) UpdateSettings(c *gin.Context) {
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
	if req.Language == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "language is required"})
		return
	}

	s := &apiaccount.UserSettings{
		PlayerID:    playerID,
		Language:    req.Language,
		BgmVolume:   req.BgmVolume,
		SeVolume:    req.SeVolume,
		PushEnabled: req.PushEnabled,
	}

	if err := h.svc.Update(c.Request.Context(), s); err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, s)
}
