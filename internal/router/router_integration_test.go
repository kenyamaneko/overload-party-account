//go:build integration

package router

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/handler/rest"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// newIntegrationRouter は実 interactor・実リポジトリで結線した router を返す。
func newIntegrationRouter() *gin.Engine {
	playerRepo := postgres.NewPlayerRepository(sharedPg.Pool)
	viewRepo := postgres.NewPlayerViewRepository(sharedPg.Pool)
	settingsRepo := postgres.NewPlayerSettingsRepository(sharedPg.Pool)
	factionRepo := postgres.NewFactionRepository(sharedPg.Pool)
	battleCountReversalRepo := postgres.NewBattleCountReversalRepository(sharedPg.Pool)
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
	settingsInteractor := usecase.NewPlayerSettingsInteractor(settingsRepo)

	return New(
		rest.NewAuthHandler(authInteractor),
		rest.NewPlayerHandler(playerInteractor),
		rest.NewFactionHandler(factionInteractor),
		rest.NewPlayerSettingsHandler(settingsInteractor),
		nullVerifier{},
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

const (
	routerTestPlayerLogin   = "11111111-1111-1111-1111-111111111111"
	routerTestPlayerLookup  = "22222222-2222-2222-2222-222222222222"
	routerTestPlayerExp1    = "33333333-3333-3333-3333-333333333333"
	routerTestPlayerExp2    = "44444444-4444-4444-4444-444444444444"
	routerTestPlayerNoSuch  = "99999999-9999-9999-9999-999999999999"
	routerTestPlayerRevert1 = "55555555-5555-5555-5555-555555555555"
	routerTestPlayerRevert2 = "66666666-6666-6666-6666-666666666666"
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
