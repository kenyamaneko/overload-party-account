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

// RevertBattleCount は停止で無効になった対戦の消費バトル回数を両プレイヤーに戻す。
// gateway の停止時処理が直接呼ぶサーバー間バッチエンドポイントで body に両 player_id を
// 含むため、JWT sub では表現できない。/internal/v1 配下に残し JWT を要求しない。
func (h *PlayerHandler) RevertBattleCount(c *gin.Context) {
	var req apiaccount.RevertBattleCountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.GameID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "game_id is required"})
		return
	}
	if req.Player1ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "player1_id is required"})
		return
	}
	if req.Player2ID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "player2_id is required"})
		return
	}
	// 未指定のゼロ値を許すと 1970-01-01 のゲーム日で処理され戻し損ねたまま game_id だけ
	// 消費されるため、0 以下を弾く。
	if req.ConsumedAtMillis <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "consumed_at_millis must be positive"})
		return
	}
	if err := h.playerInteractor.RevertBattleCount(c.Request.Context(), req.GameID, req.ConsumedAtMillis, req.Player1ID, req.Player2ID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
