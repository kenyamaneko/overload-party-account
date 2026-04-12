package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
	"github.com/kenyamaneko/overload-party-account/internal/service"
)

// PlayerHandler はプレイヤー情報の REST エンドポイントを処理します。
type PlayerHandler struct {
	playerService *service.PlayerService
}

// NewPlayerHandler は PlayerHandler を生成します。
func NewPlayerHandler(playerService *service.PlayerService) *PlayerHandler {
	return &PlayerHandler{playerService: playerService}
}

// GetPlayer はプレイヤー情報を返します。
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

// GetPlayerByFirebaseUID は Firebase UID でプレイヤーを検索します。
func (h *PlayerHandler) GetPlayerByFirebaseUID(c *gin.Context) {
	firebaseUID := c.Param("firebaseUID")
	if firebaseUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "firebaseUID is required"})
		return
	}
	player, err := h.playerService.FindByFirebaseUID(c.Request.Context(), firebaseUID)
	if err != nil {
		respondError(c, err)
		return
	}
	if player == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "player not found"})
		return
	}
	c.JSON(http.StatusOK, player)
}

// UpdateName はプレイヤー名を更新します。
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
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}

	player, err := h.playerService.UpdateUsername(c.Request.Context(), playerID, req.Name)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, player)
}

// GetBattleLimit はプレイヤーの日次バトル制限情報を返します。
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

// IncrementBattleCount は日次バトル回数をインクリメントします。
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

// UpdatePremium はプレイヤーのプレミアムステータスを更新します。
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

// UpdateFaction はプレイヤーの選択ファクションを更新します。
func (h *PlayerHandler) UpdateFaction(c *gin.Context) {
	playerID := c.Param("playerId")
	if playerID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "playerId is required"})
		return
	}
	var req apiaccount.UpdateFactionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Faction == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "faction is required"})
		return
	}
	if err := h.playerService.UpdateFaction(c.Request.Context(), playerID, req.Faction); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// AddExp はプレイヤーに経験値を付与します。
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

// AwardGameExp はゲーム終了後の経験値を両プレイヤーに付与します。
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

// GrantFaction はプレイヤーにファクションを付与します。
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
	if req.Faction == "" || req.Source == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "faction and source are required"})
		return
	}
	if err := h.playerService.GrantFaction(c.Request.Context(), playerID, req.Faction, req.Source); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// ListFactions はプレイヤーの所持ファクション一覧を返します。
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
