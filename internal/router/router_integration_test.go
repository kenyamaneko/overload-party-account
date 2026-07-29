//go:build integration

package router

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"

	pubsubadapter "github.com/kenyamaneko/overload-party-account/internal/adapter/pubsub"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush"
	"github.com/kenyamaneko/overload-party-account/internal/handler/rest"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// nullVerifier は auth 経路を通らないルートで Verify が呼ばれてはならないことを示す。
type nullVerifier struct{}

func (nullVerifier) Verify(string) (string, error) {
	panic("Verify should not be called for routes outside /api/v1/account")
}

// newIntegrationRouter は実 interactor・実リポジトリで結線した router を返す。
func newIntegrationRouter() *gin.Engine {
	playerRepo := postgres.NewPlayerRepository(sharedPg.Pool)
	viewRepo := postgres.NewPlayerViewRepository(sharedPg.Pool)
	settingsRepo := postgres.NewPlayerSettingsRepository(sharedPg.Pool)
	factionRepo := postgres.NewFactionRepository(sharedPg.Pool)
	battleCountReversalRepo := postgres.NewBattleCountReversalRepository(sharedPg.Pool)
	eventRepo := postgres.NewProcessedEventRepository(sharedPg.Pool)
	tx := postgres.NewTxManager(sharedPg.Pool)
	gameConfig := &fakeGameConfigRepo{values: map[string]int64{
		usecase.ConfigKeyExpFormulaCoefficient: 60,
		"exp_win":                              40,
		"exp_loss":                             20,
		"exp_draw":                             30,
	}}

	authInteractor := usecase.NewAuthInteractor(playerRepo, viewRepo, settingsRepo, gameConfig, tx)
	playerInteractor := usecase.NewPlayerInteractor(playerRepo, playerRepo, playerRepo, playerRepo, battleCountReversalRepo, viewRepo, gameConfig, tx)
	factionInteractor := usecase.NewFactionInteractor(playerRepo, factionRepo, tx)
	onboardingInteractor := usecase.NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, tx)
	settingsInteractor := usecase.NewPlayerSettingsInteractor(settingsRepo)

	factionSub := pubsubadapter.NewFactionAcquiredSubscriber(factionRepo, tx, eventRepo)
	premiumSub := pubsubadapter.NewPremiumUpdatedSubscriber(playerRepo, tx, eventRepo)
	onboardedSub := pubsubadapter.NewPlayerOnboardedSubscriber(onboardingInteractor)
	nameSetSub := pubsubadapter.NewOnboardingNameSetSubscriber(onboardingInteractor)
	factionSetSub := pubsubadapter.NewOnboardingFactionSetSubscriber(onboardingInteractor)

	pubsubHandlers := pubsubpush.Handlers{
		FactionAcquired:      pubsubpush.NewEventHandler(factionSub.HandleMessage),
		PremiumUpdated:       pubsubpush.NewEventHandler(premiumSub.HandleMessage),
		PlayerOnboarded:      pubsubpush.NewEventHandler(onboardedSub.HandleMessage),
		OnboardingNameSet:    pubsubpush.NewEventHandler(nameSetSub.HandleMessage),
		OnboardingFactionSet: pubsubpush.NewEventHandler(factionSetSub.HandleMessage),
	}

	return New(
		rest.NewAuthHandler(authInteractor),
		rest.NewPlayerHandler(playerInteractor),
		rest.NewFactionHandler(factionInteractor),
		rest.NewPlayerSettingsHandler(settingsInteractor),
		nullVerifier{},
		pubsubHandlers,
	)
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// pushEnvelope は Pub/Sub push subscription が送る HTTP body を模したテスト用型。
type pushEnvelope struct {
	Message struct {
		Data      string `json:"data"`
		MessageID string `json:"messageId"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

// doPubsubPush は event を JSON化 + base64 encode して push envelope に包み、path へ POST する。
func doPubsubPush(t *testing.T, r *gin.Engine, path string, event any, messageID string) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(event)
	require.NoError(t, err)
	env := pushEnvelope{Subscription: "test-sub"}
	env.Message.Data = base64.StdEncoding.EncodeToString(data)
	env.Message.MessageID = messageID
	return doJSON(t, r, http.MethodPost, path, env)
}

const (
	routerTestPlayerLogin                = "11111111-1111-1111-1111-111111111111"
	routerTestPlayerLookup               = "22222222-2222-2222-2222-222222222222"
	routerTestPlayerExp1                 = "33333333-3333-3333-3333-333333333333"
	routerTestPlayerExp2                 = "44444444-4444-4444-4444-444444444444"
	routerTestPlayerNoSuch               = "99999999-9999-9999-9999-999999999999"
	routerTestPlayerFactionAcquired      = "55555555-5555-5555-5555-555555555555"
	routerTestPlayerFactionAcquiredDup   = "66666666-6666-6666-6666-666666666666"
	routerTestPlayerPremiumUpdated       = "77777777-7777-7777-7777-777777777777"
	routerTestPlayerOnboardingNameSet    = "88888888-8888-8888-8888-888888888888"
	routerTestPlayerOnboardingFactionSet = "a1a1a1a1-a1a1-a1a1-a1a1-a1a1a1a1a1a1"
	routerTestPlayerPlayerOnboarded      = "b2b2b2b2-b2b2-b2b2-b2b2-b2b2b2b2b2b2"
	routerTestPlayerRevert1              = "c3c3c3c3-c3c3-c3c3-c3c3-c3c3c3c3c3c3"
	routerTestPlayerRevert2              = "d4d4d4d4-d4d4-d4d4-d4d4-d4d4d4d4d4d4"
)

func TestNewAuthFreeRoutesReachRealHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("認証不要の内部ルート", func(t *testing.T) {
		t.Run("新規 Firebase UID で登録すると、採番されたプレイヤーID と登録した UID が返る", func(t *testing.T) {
			sharedPg.Truncate(t)
			r := newIntegrationRouter()

			w := doJSON(t, r, http.MethodPost, "/internal/v1/auth/register", apiaccount.RegisterRequest{FirebaseUID: "uid-register"})

			require.Equal(t, http.StatusCreated, w.Code)
			var got apiaccount.PlayerResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
			assert.NotEmpty(t, got.PlayerID)
			assert.Equal(t, "uid-register", got.FirebaseUID)
		})

		t.Run("登録済みの Firebase UID でログインすると、そのプレイヤーが返る", func(t *testing.T) {
			sharedPg.Truncate(t)
			r := newIntegrationRouter()
			seedPlayer(t, routerTestPlayerLogin, "uid-login")

			w := doJSON(t, r, http.MethodPost, "/internal/v1/auth/login", apiaccount.LoginRequest{FirebaseUID: "uid-login"})

			require.Equal(t, http.StatusOK, w.Code)
			var got apiaccount.PlayerResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
			assert.Equal(t, routerTestPlayerLogin, got.PlayerID)
		})

		t.Run("登録済みの Firebase UID を照会すると、そのプレイヤーが返る", func(t *testing.T) {
			sharedPg.Truncate(t)
			r := newIntegrationRouter()
			seedPlayer(t, routerTestPlayerLookup, "uid-lookup")

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/internal/v1/auth/by-firebase-uid/uid-lookup", nil))

			require.Equal(t, http.StatusOK, w.Code)
			var got apiaccount.PlayerResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
			assert.Equal(t, routerTestPlayerLookup, got.PlayerID)
		})

		t.Run("所持ファクションが無いプレイヤーを照会すると、空のファクション一覧が返る", func(t *testing.T) {
			sharedPg.Truncate(t)
			r := newIntegrationRouter()

			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/internal/v1/players/"+routerTestPlayerNoSuch+"/factions", nil))

			require.Equal(t, http.StatusOK, w.Code)
			var got apiaccount.FactionListing
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
			assert.Empty(t, got.Factions)
		})

		t.Run("対戦結果を送ると、勝者の経験値が増える", func(t *testing.T) {
			sharedPg.Truncate(t)
			r := newIntegrationRouter()
			seedPlayer(t, routerTestPlayerExp1, "uid-exp-1")
			seedPlayer(t, routerTestPlayerExp2, "uid-exp-2")

			w := doJSON(t, r, http.MethodPost, "/internal/v1/players/award-game-exp", apiaccount.AwardGameExpRequest{
				Player1ID: routerTestPlayerExp1, Player2ID: routerTestPlayerExp2, WinnerNum: 1, Reason: "test", MatchType: "pvp",
			})

			require.Equal(t, http.StatusNoContent, w.Code)
			var winnerExp int64
			require.NoError(t, sharedPg.Pool.QueryRow(context.Background(),
				`SELECT exp FROM account.player_progression WHERE player_id = $1`, routerTestPlayerExp1).Scan(&winnerExp))
			assert.Positive(t, winnerExp)
		})

		t.Run("停止で無効になった対戦の消費バトル回数を戻すと、両プレイヤーの当日カウントが 1 減る", func(t *testing.T) {
			sharedPg.Truncate(t)
			r := newIntegrationRouter()
			seedPlayer(t, routerTestPlayerRevert1, "uid-revert-1")
			seedPlayer(t, routerTestPlayerRevert2, "uid-revert-2")
			today := civil.DateOf(time.Now().UTC().Add(4 * time.Hour))
			ctx := context.Background()
			for _, playerID := range []string{routerTestPlayerRevert1, routerTestPlayerRevert2} {
				_, err := sharedPg.Pool.Exec(ctx,
					`INSERT INTO account.player_daily_battle (player_id, game_date, daily_battle_count) VALUES ($1, $2, 1)`,
					playerID, time.Date(today.Year, today.Month, today.Day, 0, 0, 0, 0, time.UTC))
				require.NoError(t, err)
			}

			w := doJSON(t, r, http.MethodPost, "/internal/v1/players/revert-battle-count", apiaccount.RevertBattleCountRequest{
				GameID:           "77777777-7777-7777-7777-777777777777",
				Player1ID:        routerTestPlayerRevert1,
				Player2ID:        routerTestPlayerRevert2,
				ConsumedAtMillis: time.Now().UnixMilli(),
			})

			require.Equal(t, http.StatusNoContent, w.Code)
			for _, playerID := range []string{routerTestPlayerRevert1, routerTestPlayerRevert2} {
				var count int64
				require.NoError(t, sharedPg.Pool.QueryRow(ctx,
					`SELECT daily_battle_count FROM account.player_daily_battle WHERE player_id = $1 AND game_date = $2`,
					playerID, time.Date(today.Year, today.Month, today.Day, 0, 0, 0, 0, time.UTC)).Scan(&count))
				assert.Equal(t, int64(0), count)
			}
		})
	})
}

func TestNewPubsubPushRoutesReachRealHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("認証不要の Pub/Sub push 受け口", func(t *testing.T) {
		t.Run("faction-acquired の push を受けると、player_factions に is_initial=FALSE で追加される", func(t *testing.T) {
			sharedPg.Truncate(t)
			r := newIntegrationRouter()
			seedPlayer(t, routerTestPlayerFactionAcquired, "uid-push-faction-acquired")

			w := doPubsubPush(t, r, "/internal/v1/pubsub/faction-acquired", apishop.FactionAcquiredEvent{
				EventType: apishop.EventTypeFactionAcquired,
				EventID:   "c1c1c1c1-c1c1-c1c1-c1c1-c1c1c1c1c1c1",
				PlayerID:  routerTestPlayerFactionAcquired,
				Faction:   "SHE",
			}, "msg-faction-acquired-1")

			require.Equal(t, http.StatusOK, w.Code)
			var isInitial bool
			require.NoError(t, sharedPg.Pool.QueryRow(context.Background(),
				`SELECT is_initial FROM account.player_factions WHERE player_id = $1 AND faction = $2`,
				routerTestPlayerFactionAcquired, "SHE").Scan(&isInitial))
			assert.False(t, isInitial)
		})

		t.Run("faction-acquired の push を同じ event_id で内容の異なる push として二度受けても、最初の内容だけが反映される", func(t *testing.T) {
			sharedPg.Truncate(t)
			r := newIntegrationRouter()
			seedPlayer(t, routerTestPlayerFactionAcquiredDup, "uid-push-faction-acquired-dup")
			const redeliveredEventID = "c2c2c2c2-c2c2-c2c2-c2c2-c2c2c2c2c2c2"

			// event_id は同じだが faction が異なる 2 通目を送る。processed_events の
			// 冪等ガードが効いていなければ (player_id, faction) の PK が異なるため
			// 2 行目が INSERT されてしまい、count=1 のアサーションだけでは検知できない。
			first := doPubsubPush(t, r, "/internal/v1/pubsub/faction-acquired", apishop.FactionAcquiredEvent{
				EventType: apishop.EventTypeFactionAcquired,
				EventID:   redeliveredEventID,
				PlayerID:  routerTestPlayerFactionAcquiredDup,
				Faction:   "SHE",
			}, "msg-dup-1")
			redelivered := doPubsubPush(t, r, "/internal/v1/pubsub/faction-acquired", apishop.FactionAcquiredEvent{
				EventType: apishop.EventTypeFactionAcquired,
				EventID:   redeliveredEventID,
				PlayerID:  routerTestPlayerFactionAcquiredDup,
				Faction:   "Tenki",
			}, "msg-dup-1-redelivered")

			require.Equal(t, http.StatusOK, first.Code)
			require.Equal(t, http.StatusOK, redelivered.Code)
			var count int
			require.NoError(t, sharedPg.Pool.QueryRow(context.Background(),
				`SELECT COUNT(*) FROM account.player_factions WHERE player_id = $1`,
				routerTestPlayerFactionAcquiredDup).Scan(&count))
			require.Equal(t, 1, count, "同一 event_id の再配信は二重に反映されない")
			var faction string
			require.NoError(t, sharedPg.Pool.QueryRow(context.Background(),
				`SELECT faction FROM account.player_factions WHERE player_id = $1`,
				routerTestPlayerFactionAcquiredDup).Scan(&faction))
			assert.Equal(t, "SHE", faction, "最初の push の内容だけが反映される")
		})

		t.Run("premium-updated の push を受けると、players.is_premium が更新される", func(t *testing.T) {
			sharedPg.Truncate(t)
			r := newIntegrationRouter()
			seedPlayer(t, routerTestPlayerPremiumUpdated, "uid-push-premium-updated")

			w := doPubsubPush(t, r, "/internal/v1/pubsub/premium-updated", apishop.PremiumUpdatedEvent{
				EventType: apishop.EventTypePremiumUpdated,
				EventID:   "c3c3c3c3-c3c3-c3c3-c3c3-c3c3c3c3c3c3",
				PlayerID:  routerTestPlayerPremiumUpdated,
				IsPremium: true,
			}, "msg-premium-updated-1")

			require.Equal(t, http.StatusOK, w.Code)
			var isPremium bool
			require.NoError(t, sharedPg.Pool.QueryRow(context.Background(),
				`SELECT is_premium FROM account.players WHERE player_id = $1`,
				routerTestPlayerPremiumUpdated).Scan(&isPremium))
			assert.True(t, isPremium)
		})

		t.Run("onboarding-name-set の push を受けると、players.name が更新される", func(t *testing.T) {
			sharedPg.Truncate(t)
			r := newIntegrationRouter()
			seedPlayer(t, routerTestPlayerOnboardingNameSet, "uid-push-onboarding-name-set")

			w := doPubsubPush(t, r, "/internal/v1/pubsub/onboarding-name-set", apiscenario.OnboardingNameSetEvent{
				EventType: apiscenario.EventTypeOnboardingNameSet,
				EventID:   "c4c4c4c4-c4c4-c4c4-c4c4-c4c4c4c4c4c4",
				PlayerID:  routerTestPlayerOnboardingNameSet,
				Name:      "PushName",
			}, "msg-onboarding-name-set-1")

			require.Equal(t, http.StatusOK, w.Code)
			var name string
			require.NoError(t, sharedPg.Pool.QueryRow(context.Background(),
				`SELECT name FROM account.players WHERE player_id = $1`,
				routerTestPlayerOnboardingNameSet).Scan(&name))
			assert.Equal(t, "PushName", name)
		})

		t.Run("onboarding-faction-set の push を受けると、player_factions に is_initial=TRUE で追加される", func(t *testing.T) {
			sharedPg.Truncate(t)
			r := newIntegrationRouter()
			seedPlayer(t, routerTestPlayerOnboardingFactionSet, "uid-push-onboarding-faction-set")

			w := doPubsubPush(t, r, "/internal/v1/pubsub/onboarding-faction-set", apiscenario.OnboardingFactionSetEvent{
				EventType:        apiscenario.EventTypeOnboardingFactionSet,
				EventID:          "c5c5c5c5-c5c5-c5c5-c5c5-c5c5c5c5c5c5",
				PlayerID:         routerTestPlayerOnboardingFactionSet,
				InitialFactionID: "SHE",
			}, "msg-onboarding-faction-set-1")

			require.Equal(t, http.StatusOK, w.Code)
			var isInitial bool
			require.NoError(t, sharedPg.Pool.QueryRow(context.Background(),
				`SELECT is_initial FROM account.player_factions WHERE player_id = $1 AND faction = $2`,
				routerTestPlayerOnboardingFactionSet, "SHE").Scan(&isInitial))
			assert.True(t, isInitial)
		})

		t.Run("player-onboarded の push を受けると、onboarding_status が completed になる", func(t *testing.T) {
			sharedPg.Truncate(t)
			r := newIntegrationRouter()
			seedPlayer(t, routerTestPlayerPlayerOnboarded, "uid-push-player-onboarded")

			w := doPubsubPush(t, r, "/internal/v1/pubsub/player-onboarded", apiscenario.PlayerOnboardedEvent{
				EventType: apiscenario.EventTypePlayerOnboarded,
				EventID:   "c6c6c6c6-c6c6-c6c6-c6c6-c6c6c6c6c6c6",
				PlayerID:  routerTestPlayerPlayerOnboarded,
			}, "msg-player-onboarded-1")

			require.Equal(t, http.StatusOK, w.Code)
			var onboardingStatus string
			require.NoError(t, sharedPg.Pool.QueryRow(context.Background(),
				`SELECT onboarding_status FROM account.players WHERE player_id = $1`,
				routerTestPlayerPlayerOnboarded).Scan(&onboardingStatus))
			assert.Equal(t, "completed", onboardingStatus)
		})
	})
}
