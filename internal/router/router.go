package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush"
	"github.com/kenyamaneko/overload-party-account/internal/handler/rest"

	internalauth "github.com/kenyamaneko/overload-party-gateway/packages/internalauth-go"
)

// New は account サービスの HTTP ルーターを生成する。
// /api/v1/account 配下は内部認証が必要で、/internal/v1 配下と /health は認証しない。
func New(
	authHandler *rest.AuthHandler,
	playerHandler *rest.PlayerHandler,
	factionHandler *rest.FactionHandler,
	settingsHandler *rest.PlayerSettingsHandler,
	authVerifier internalauth.Verifier,
	pubsubHandlers pubsubpush.Handlers,
) *gin.Engine {
	r := gin.New()
	r.Use(rest.NewRecovery())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	internal := r.Group("/internal/v1")
	{
		internal.POST("/auth/register", authHandler.Register)
		internal.POST("/auth/login", authHandler.Login)
		internal.GET("/auth/by-firebase-uid/:firebaseUID", authHandler.GetPlayerByFirebaseUID)

		internal.GET("/players/:playerID/factions", factionHandler.ListPlayerFactions)
		internal.POST("/players/award-game-exp", playerHandler.AwardGameExp)
		internal.POST("/players/revert-battle-count", playerHandler.RevertBattleCount)

		pubsubGroup := internal.Group("/pubsub")
		{
			pubsubGroup.POST("/faction-acquired", pubsubHandlers.FactionAcquired.ReceivePush)
			pubsubGroup.POST("/premium-updated", pubsubHandlers.PremiumUpdated.ReceivePush)
			pubsubGroup.POST("/player-onboarded", pubsubHandlers.PlayerOnboarded.ReceivePush)
			pubsubGroup.POST("/onboarding-name-set", pubsubHandlers.OnboardingNameSet.ReceivePush)
			pubsubGroup.POST("/onboarding-faction-set", pubsubHandlers.OnboardingFactionSet.ReceivePush)
		}
	}

	api := r.Group("/api/v1/account")
	api.Use(internalauth.VerifyInternalAuth(authVerifier))
	{
		me := api.Group("/me")
		{
			me.GET("", playerHandler.GetPlayer)
			me.PUT("/name", playerHandler.UpdateName)
			me.POST("/onboarding/name/validate", playerHandler.ValidateNameForOnboarding)
			me.GET("/battle-limit", playerHandler.GetBattleLimit)
			me.POST("/battle-limit/increment", playerHandler.IncrementBattleCount)
			me.PUT("/premium", playerHandler.UpdatePremium)
			me.POST("/exp", playerHandler.AddExp)
			me.POST("/factions", factionHandler.GrantFaction)
			me.POST("/factions/select", factionHandler.SelectInitialFaction)
			me.GET("/factions", factionHandler.ListFactions)
			me.GET("/settings", settingsHandler.GetSettings)
			me.PUT("/settings", settingsHandler.UpdateSettings)
		}
	}
	return r
}
