package rest

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/presenter"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"

	internalauth "github.com/kenyamaneko/overload-party-gateway/packages/internalauth-go"
)

// PlayerHandler はプレイヤー情報・表示名・バトル回数・プレミアム状態・経験値の REST の受け口である。
type PlayerHandler struct {
	playerInteractor  *usecase.PlayerInteractor
	premiumInteractor *usecase.PremiumInteractor
}

// NewPlayerHandler は PlayerHandler を生成する。
func NewPlayerHandler(playerInteractor *usecase.PlayerInteractor, premiumInteractor *usecase.PremiumInteractor) *PlayerHandler {
	return &PlayerHandler{playerInteractor: playerInteractor, premiumInteractor: premiumInteractor}
}

// GetPlayer は認証済みプレイヤーのプレイヤー情報を返す。
func (h *PlayerHandler) GetPlayer(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)
	info, err := h.playerInteractor.GetPlayer(c.Request.Context(), playerID)
	if err != nil {
		respondError(c, err)
		return
	}
	respondPlayer(c, http.StatusOK, info)
}

// UpdateName は認証済みプレイヤーの表示名を更新する。
func (h *PlayerHandler) UpdateName(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)

	var req apiaccount.UpdateNameRequest
	if !bindRequestBody(c, &req) {
		return
	}

	info, err := h.playerInteractor.UpdateName(c.Request.Context(), playerID, req.Name)
	if err != nil {
		respondError(c, err)
		return
	}
	respondPlayer(c, http.StatusOK, info)
}

// ValidateNameForOnboarding はオンボーディング中の表示名が正しいかを検証する。表示名は保存しない。
func (h *PlayerHandler) ValidateNameForOnboarding(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)

	var req apiaccount.ValidateNameForOnboardingRequest
	if !bindRequestBody(c, &req) {
		return
	}

	if err := h.playerInteractor.ValidateNameForOnboarding(c.Request.Context(), playerID, req.Name); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// GetBattleLimit は認証済みプレイヤーの1日のバトル回数と上限を返す。
func (h *PlayerHandler) GetBattleLimit(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)
	limit, err := h.playerInteractor.GetBattleLimit(c.Request.Context(), playerID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, presenter.ToBattleLimitResponse(limit))
}

// IncrementBattleCount は認証済みプレイヤーの当日のバトル回数を1回増やす。
func (h *PlayerHandler) IncrementBattleCount(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)
	if err := h.playerInteractor.IncrementBattleCount(c.Request.Context(), playerID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// UpdatePremium は認証済みプレイヤーのプレミアム状態を更新する。
func (h *PlayerHandler) UpdatePremium(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)

	var req apiaccount.UpdatePremiumRequest
	if !bindRequestBody(c, &req, "is_premium") {
		return
	}
	expiresAt, isValid := toExpiresAt(req.ExpiresAtMillis)
	if !isValid {
		respondBadRequest(c, invalidFieldMessage("expires_at_millis"))
		return
	}

	if err := h.premiumInteractor.UpdatePremium(c.Request.Context(), playerID, req.IsPremium, expiresAt); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// AddExp は認証済みプレイヤーに経験値を加算する。
func (h *PlayerHandler) AddExp(c *gin.Context) {
	playerID := c.GetString(internalauth.PlayerIDContextKey)

	var req apiaccount.AddExpRequest
	if !bindRequestBody(c, &req, "exp_gain") {
		return
	}

	if err := h.playerInteractor.AwardExp(c.Request.Context(), playerID, req.ExpGain); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// AwardGameExp は対戦の結果に応じた経験値を両プレイヤーに付与する。
func (h *PlayerHandler) AwardGameExp(c *gin.Context) {
	var req apiaccount.AwardGameExpRequest
	if !bindRequestBody(c, &req, "winner_num") {
		return
	}
	if respondIfEmpty(c, "player1_id", req.Player1ID) ||
		respondIfEmpty(c, "reason", req.Reason) ||
		respondIfEmpty(c, "match_type", req.MatchType) {
		return
	}
	player1ID, isPlayer1Valid := parsePlayerIDField(c, "player1_id", req.Player1ID)
	if !isPlayer1Valid {
		return
	}
	player2ID := ""
	if req.Player2ID != "" {
		var isPlayer2Valid bool
		player2ID, isPlayer2Valid = parsePlayerIDField(c, "player2_id", req.Player2ID)
		if !isPlayer2Valid {
			return
		}
	}

	err := h.playerInteractor.AwardGameExp(c.Request.Context(), player1ID, player2ID,
		domain.WinnerNumber(req.WinnerNum), domain.WinReason(req.Reason), domain.MatchType(req.MatchType))
	if err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// RevertBattleCount は停止で無効になった対戦の消費バトル回数を両プレイヤーに戻す。
func (h *PlayerHandler) RevertBattleCount(c *gin.Context) {
	var req apiaccount.RevertBattleCountRequest
	if !bindRequestBody(c, &req) {
		return
	}
	if respondIfEmpty(c, "game_id", req.GameID) ||
		respondIfEmpty(c, "player1_id", req.Player1ID) ||
		respondIfEmpty(c, "player2_id", req.Player2ID) {
		return
	}
	if req.ConsumedAtMillis <= 0 {
		respondBadRequest(c, "consumed_at_millis must be positive")
		return
	}
	player1ID, isPlayer1Valid := parsePlayerIDField(c, "player1_id", req.Player1ID)
	if !isPlayer1Valid {
		return
	}
	player2ID, isPlayer2Valid := parsePlayerIDField(c, "player2_id", req.Player2ID)
	if !isPlayer2Valid {
		return
	}

	consumedAt := time.UnixMilli(req.ConsumedAtMillis).UTC()
	if err := h.playerInteractor.RevertBattleCount(c.Request.Context(), req.GameID, consumedAt, player1ID, player2ID); err != nil {
		respondError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// toExpiresAt はプレミアムの有効期限の UNIX ミリ秒を時刻にする。
// 省略されたときは nil を返し、0 以下のときは false を返す。
func toExpiresAt(expiresAtMillis *int64) (expiresAt *time.Time, isValid bool) {
	if expiresAtMillis == nil {
		return nil, true
	}
	if *expiresAtMillis <= 0 {
		return nil, false
	}
	t := time.UnixMilli(*expiresAtMillis).UTC()
	return &t, true
}
