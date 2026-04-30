package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/handler/rest"
)

// New は account サービスの HTTP ルーターを構築します。
func New(
	authH *rest.AuthHandler,
	playerH *rest.PlayerHandler,
	factionH *rest.FactionHandler,
	settingsH *rest.PlayerSettingsHandler,
) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := r.Group("/internal/v1")
	{
		v1.POST("/auth/register", authH.Register)
		v1.POST("/auth/login", authH.Login)
		v1.GET("/auth/by-firebase-uid/:firebaseUID", authH.GetPlayerByFirebaseUID)

		v1.POST("/players/award-game-exp", playerH.AwardGameExp)

		players := v1.Group("/players/:playerId")
		{
			players.GET("", playerH.GetPlayer)
			players.PUT("/name", playerH.UpdateName)
			players.POST("/onboarding/name/validate", playerH.ValidateOnboardingName)
			players.GET("/battle-limit", playerH.GetBattleLimit)
			players.POST("/battle-limit/increment", playerH.IncrementBattleCount)
			players.PUT("/premium", playerH.UpdatePremium)
			players.POST("/exp", playerH.AddExp)
			players.POST("/factions", playerH.GrantFaction)
			players.POST("/factions/select", factionH.SelectInitialFaction)
			players.GET("/factions", playerH.ListFactions)
			players.GET("/settings", settingsH.GetSettings)
			players.PUT("/settings", settingsH.UpdateSettings)
		}
	}
	return r
}
