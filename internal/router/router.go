package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/handler/rest"

	internalauth "github.com/kenyamaneko/overload-party-gateway/packages/internalauth-go"
)

// New は account サービスの HTTP ルーターを構築する。
//
// ルーティング方針 (ADR-037 Phase 3):
//   - /internal/v1/auth/*: Firebase UID をキーとする bootstrap 系。player_id 確立前
//     のため JWT を要求しない
//   - /internal/v1/players/award-game-exp: battle が直接呼ぶサーバー間バッチ。body に
//     2 プレイヤー分の player_id を含み JWT sub では表現できないため /internal に残す
//   - /api/v1/account/me/*: gateway / shop / scenario が JWT を付けて呼ぶ player-scoped
//     API。VerifyInternalAuth が sub クレーム (= player_id) を context に注入し、
//     handler は path / body から player_id を読まない
func New(
	authH *rest.AuthHandler,
	playerH *rest.PlayerHandler,
	factionH *rest.FactionHandler,
	settingsH *rest.PlayerSettingsHandler,
	authVerifier internalauth.Verifier,
) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	internal := r.Group("/internal/v1")
	{
		internal.POST("/auth/register", authH.Register)
		internal.POST("/auth/login", authH.Login)
		internal.GET("/auth/by-firebase-uid/:firebaseUID", authH.GetPlayerByFirebaseUID)

		internal.GET("/players/:playerID/factions", factionH.ListPlayerFactions)
		internal.POST("/players/award-game-exp", playerH.AwardGameExp)
	}

	api := r.Group("/api/v1/account")
	api.Use(internalauth.VerifyInternalAuth(authVerifier))
	{
		me := api.Group("/me")
		{
			me.GET("", playerH.GetPlayer)
			me.PUT("/name", playerH.UpdateName)
			me.POST("/onboarding/name/validate", playerH.ValidateNameForOnboarding)
			me.GET("/battle-limit", playerH.GetBattleLimit)
			me.POST("/battle-limit/increment", playerH.IncrementBattleCount)
			me.PUT("/premium", playerH.UpdatePremium)
			me.POST("/exp", playerH.AddExp)
			me.POST("/factions", factionH.GrantFaction)
			me.POST("/factions/select", factionH.SelectInitialFaction)
			me.GET("/factions", factionH.ListFactions)
			me.GET("/settings", settingsH.GetSettings)
			me.PUT("/settings", settingsH.UpdateSettings)
		}
	}
	return r
}
