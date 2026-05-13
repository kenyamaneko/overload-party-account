package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"

	internalauth "github.com/kenyamaneko/overload-party-gateway/packages/internalauth-go"
)

// PlayerHandler はプレイヤー情報の REST エンドポイントを処理する。
type PlayerHandler struct {
	playerInteractor *usecase.PlayerInteractor
}

// NewPlayerHandler は PlayerHandler を生成する。
func NewPlayerHandler(playerInteractor *usecase.PlayerInteractor) *PlayerHandler {
	return &PlayerHandler{playerInteractor: playerInteractor}
}

// GetPlayer はプレイヤー情報を返す。
func (h *PlayerHandler) GetPlayer(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)
	resp, err := h.playerInteractor.GetPlayerResponse(c.Request.Context(), playerID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// GetPlayerByID は path で指定された player_id のプレイヤー情報を返す。
func (h *PlayerHandler) GetPlayerByID(c *gin.Context) {
	playerID := c.Param("playerID")
	resp, err := h.playerInteractor.GetPlayerResponse(c.Request.Context(), playerID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateName はプレイヤー名を更新する。
func (h *PlayerHandler) UpdateName(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)

	var req apiaccount.UpdateNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	player, err := h.playerInteractor.UpdateName(c.Request.Context(), playerID, req.Name)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, player)
}

// ValidateNameForOnboarding はオンボード内 name 入力ステップで scenario が呼ぶ
// バリデーション専用ハンドラ。書き込みは onboarding-name-set subscriber が担うため、
// ここでは行わない。
func (h *PlayerHandler) ValidateNameForOnboarding(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)

	var req apiaccount.ValidateNameForOnboardingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.playerInteractor.ValidateNameForOnboarding(c.Request.Context(), playerID, req.Name); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// GetBattleLimit はプレイヤーの日次バトル制限情報を返す。
func (h *PlayerHandler) GetBattleLimit(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)
	result, err := h.playerInteractor.GetBattleLimit(c.Request.Context(), playerID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// IncrementBattleCount は日次バトル回数をインクリメントする。
func (h *PlayerHandler) IncrementBattleCount(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)
	if err := h.playerInteractor.IncrementBattleCount(c.Request.Context(), playerID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// UpdatePremium はプレイヤーのプレミアムステータスを更新する。
func (h *PlayerHandler) UpdatePremium(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)
	var req apiaccount.UpdatePremiumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.playerInteractor.UpdatePremium(c.Request.Context(), playerID, req.IsPremium, req.ExpiresAtMillis); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// AddExp はプレイヤーに経験値を付与する。
func (h *PlayerHandler) AddExp(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)
	var req apiaccount.AddExpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.playerInteractor.AwardExp(c.Request.Context(), playerID, req.ExpGain); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// AwardGameExp はゲーム終了後の経験値を両プレイヤーに付与する。
// battle が直接呼ぶサーバー間バッチエンドポイントで body に両 player_id を含むため、
// JWT sub では表現できない。/internal/v1 配下に残し JWT を要求しない。
func (h *PlayerHandler) AwardGameExp(c *gin.Context) {
	var req apiaccount.AwardGameExpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.playerInteractor.AwardGameExp(c.Request.Context(), req.Player1ID, req.Player2ID, req.WinnerNum, req.Reason, req.MatchType); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
