package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// PlayerHandler はプレイヤー情報の REST エンドポイントを処理する。
type PlayerHandler struct {
	playerService *usecase.PlayerInteractor
}

// NewPlayerHandler は PlayerHandler を生成する。
func NewPlayerHandler(playerService *usecase.PlayerInteractor) *PlayerHandler {
	return &PlayerHandler{playerService: playerService}
}

// GetPlayer はプレイヤー情報を返す。
func (h *PlayerHandler) GetPlayer(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "playerId is required"})
		return
	}
	resp, err := h.playerService.GetPlayerResponse(c.Request.Context(), playerID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, resp)
}

// UpdateName はプレイヤー名を更新する。
func (h *PlayerHandler) UpdateName(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "playerId is required"})
		return
	}

	var req apiaccount.UpdateNameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	player, err := h.playerService.UpdateName(c.Request.Context(), playerID, req.Name)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, player)
}

// ValidateOnboardingName はオンボード内 name 入力ステップで scenario が呼ぶ
// バリデーション専用ハンドラ。書き込みは onboarding-name-set subscriber が担うため、
// ここでは行わない。
func (h *PlayerHandler) ValidateOnboardingName(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "playerId is required"})
		return
	}

	var req apiaccount.OnboardingNameValidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.playerService.ValidateOnboardingName(c.Request.Context(), playerID, req.Name); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// GetBattleLimit はプレイヤーの日次バトル制限情報を返す。
func (h *PlayerHandler) GetBattleLimit(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "playerId is required"})
		return
	}
	result, err := h.playerService.GetBattleLimit(c.Request.Context(), playerID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}

// IncrementBattleCount は日次バトル回数をインクリメントする。
func (h *PlayerHandler) IncrementBattleCount(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "playerId is required"})
		return
	}
	if err := h.playerService.IncrementBattleCount(c.Request.Context(), playerID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// UpdatePremium はプレイヤーのプレミアムステータスを更新する。
func (h *PlayerHandler) UpdatePremium(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "playerId is required"})
		return
	}
	var req apiaccount.UpdatePremiumRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.playerService.UpdatePremium(c.Request.Context(), playerID, req.IsPremium, req.ExpiresAtMillis); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// AddExp はプレイヤーに経験値を付与する。
func (h *PlayerHandler) AddExp(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "playerId is required"})
		return
	}
	var req apiaccount.AddExpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.playerService.AwardExp(c.Request.Context(), playerID, req.ExpGain); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// AwardGameExp はゲーム終了後の経験値を両プレイヤーに付与する。
func (h *PlayerHandler) AwardGameExp(c *gin.Context) {
	var req apiaccount.AwardGameExpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := h.playerService.AwardGameExp(c.Request.Context(), req.Player1ID, req.Player2ID, req.WinnerNum, req.Reason, req.MatchType); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// GrantFaction はプレイヤーにファクションを付与する。
func (h *PlayerHandler) GrantFaction(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "playerId is required"})
		return
	}
	var req apiaccount.FactionGrantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Faction == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "faction is required"})
		return
	}
	if err := h.playerService.GrantFaction(c.Request.Context(), playerID, req.Faction); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListFactions はプレイヤーの所持ファクション一覧を返す。
func (h *PlayerHandler) ListFactions(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "playerId is required"})
		return
	}
	factions, err := h.playerService.ListFactions(c.Request.Context(), playerID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, apiaccount.ListFactionsResponse{Factions: factions})
}
