//go:build integration

package rest

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"

	internalauth "github.com/kenyamaneko/overload-party-gateway/packages/internalauth-go"
)

const (
	contractPlayerID = "11111111-1111-1111-1111-111111111111"
	missingPlayerID  = "99999999-9999-9999-9999-999999999999"

	// contractFreeDailyBattleLimitKey は free_daily_battle_limit の SSoT
	// (usecase.configKeyFreeDailyBattleLimit) が非公開のため、テスト側で再宣言する
	// ダミーキー参照。
	contractFreeDailyBattleLimitKey = "free_daily_battle_limit"
)

// defaultContractGameConfig は newContractEngine が使う既定のゲーム設定 fake を返す。
func defaultContractGameConfig() *fakeGameConfigRepo {
	return &fakeGameConfigRepo{
		values: map[string]int64{
			usecase.ConfigKeyExpFormulaCoefficient: 60,
			contractFreeDailyBattleLimitKey:        10,
		},
		errKeys: map[string]error{},
	}
}

// newContractEngine は実 PostgreSQL repository を結線した account ハンドラを
// 認証済み player_id を注入する gin エンジンとして組む。
func newContractEngine(playerID string) *gin.Engine {
	return newContractEngineWithGameConfig(playerID, defaultContractGameConfig())
}

// newContractEngineWithGameConfig は gameConfig を差し替え可能な newContractEngine。
// 設定値欠落・読み取りエラーの伝播を HTTP 経路で検証するケース向け。
func newContractEngineWithGameConfig(playerID string, gameConfig *fakeGameConfigRepo) *gin.Engine {
	playerRepo := postgres.NewPlayerRepository(sharedPg.Pool)
	viewRepo := postgres.NewPlayerViewRepository(sharedPg.Pool)
	settingsRepo := postgres.NewPlayerSettingsRepository(sharedPg.Pool)
	factionRepo := postgres.NewFactionRepository(sharedPg.Pool)
	battleCountReversalRepo := postgres.NewBattleCountReversalRepository(sharedPg.Pool)
	tx := postgres.NewTxManager(sharedPg.Pool)

	authInteractor := usecase.NewAuthInteractor(playerRepo, viewRepo, settingsRepo, gameConfig, tx)
	playerInteractor := usecase.NewPlayerInteractor(playerRepo, playerRepo, playerRepo, playerRepo, battleCountReversalRepo, viewRepo, gameConfig, tx)
	settingsInteractor := usecase.NewPlayerSettingsInteractor(settingsRepo)
	factionInteractor := usecase.NewFactionInteractor(playerRepo, factionRepo, tx)
	authH := NewAuthHandler(authInteractor)
	playerH := NewPlayerHandler(playerInteractor)
	settingsH := NewPlayerSettingsHandler(settingsInteractor)
	factionH := NewFactionHandler(factionInteractor)

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(internalauth.PlayerIDContextKey, playerID) })
	r.POST("/auth/register", authH.Register)
	r.POST("/auth/login", authH.Login)
	r.GET("/me", playerH.GetPlayer)
	r.PUT("/me/name", playerH.UpdateName)
	r.POST("/me/onboarding/name/validate", playerH.ValidateNameForOnboarding)
	r.GET("/me/battle-limit", playerH.GetBattleLimit)
	r.POST("/me/battle-limit/increment", playerH.IncrementBattleCount)
	r.PUT("/me/premium", playerH.UpdatePremium)
	r.POST("/me/exp", playerH.AddExp)
	r.POST("/internal/v1/players/award-game-exp", playerH.AwardGameExp)
	r.POST("/internal/v1/players/revert-battle-count", playerH.RevertBattleCount)
	r.PUT("/me/settings", settingsH.UpdateSettings)
	r.POST("/me/factions/select", factionH.SelectInitialFaction)
	r.POST("/me/factions", factionH.GrantFaction)
	return r
}

func TestHandlerErrorStatusContract(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("エラー種別の HTTP ステータス写像", func(t *testing.T) {
		tests := []struct {
			name             string
			seed             func(t *testing.T)
			playerID         string
			method           string
			path             string
			body             string
			configErrKeys    map[string]error
			wantStatus       int
			wantBodyContains string
		}{
			{
				name:       "存在するプレイヤーを取得するとき、200 になる",
				seed:       func(t *testing.T) { seedPlayer(t, contractPlayerID, "uid-1") },
				playerID:   contractPlayerID,
				method:     http.MethodGet,
				path:       "/me",
				wantStatus: http.StatusOK,
			},
			{
				name:             "存在しないプレイヤーを取得するとき、not-found を 404 に写像する",
				seed:             func(t *testing.T) {},
				playerID:         missingPlayerID,
				method:           http.MethodGet,
				path:             "/me",
				wantStatus:       http.StatusNotFound,
				wantBodyContains: port.ErrNotFound.Error(),
			},
			{
				name:             "更新対象のフィールドを一つも指定しない設定更新のとき、400 になり、応答本文に \"at least one settings field is required\" が含まれる",
				seed:             func(t *testing.T) {},
				playerID:         contractPlayerID,
				method:           http.MethodPut,
				path:             "/me/settings",
				body:             "{}",
				wantStatus:       http.StatusBadRequest,
				wantBodyContains: "at least one settings field is required",
			},
			{
				name: "初期陣営選択済みで再選択するとき、conflict を 409 に写像する",
				seed: func(t *testing.T) {
					seedPlayer(t, contractPlayerID, "uid-1")
					first := httptest.NewRequest(http.MethodPost, "/me/factions/select", strings.NewReader(`{"faction_id":"SHE"}`))
					first.Header.Set("Content-Type", "application/json")
					fw := httptest.NewRecorder()
					newContractEngine(contractPlayerID).ServeHTTP(fw, first)
					require.Equal(t, http.StatusOK, fw.Code)
				},
				playerID:         contractPlayerID,
				method:           http.MethodPost,
				path:             "/me/factions/select",
				body:             `{"faction_id":"SHE"}`,
				wantStatus:       http.StatusConflict,
				wantBodyContains: usecase.ErrFactionAlreadySelected.Error(),
			},
			{
				name:             "選択不可の Neutral を初期陣営に選ぶと、400 になる",
				seed:             func(t *testing.T) {},
				playerID:         contractPlayerID,
				method:           http.MethodPost,
				path:             "/me/factions/select",
				body:             `{"faction_id":"Neutral"}`,
				wantStatus:       http.StatusBadRequest,
				wantBodyContains: usecase.ErrInvalidFaction.Error(),
			},
			{
				name:             "空白だけの表示名に更新すると、400 になる",
				seed:             func(t *testing.T) {},
				playerID:         contractPlayerID,
				method:           http.MethodPut,
				path:             "/me/name",
				body:             `{"name":"   "}`,
				wantStatus:       http.StatusBadRequest,
				wantBodyContains: domain.ErrInvalidName.Error(),
			},
			{
				name:     "どの種別にも分類されないエラーのとき、500 になる",
				seed:     func(t *testing.T) { seedPlayer(t, contractPlayerID, "uid-1") },
				playerID: contractPlayerID,
				method:   http.MethodGet,
				path:     "/me",
				configErrKeys: map[string]error{
					usecase.ConfigKeyExpFormulaCoefficient: errors.New("game config read failure"),
				},
				wantStatus: http.StatusInternalServerError,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				tt.seed(t)

				gameConfig := defaultContractGameConfig()
				for key, injectedErr := range tt.configErrKeys {
					gameConfig.errKeys[key] = injectedErr
				}

				req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				newContractEngineWithGameConfig(tt.playerID, gameConfig).ServeHTTP(w, req)

				assert.Equal(t, tt.wantStatus, w.Code)
				assert.Contains(t, w.Body.String(), tt.wantBodyContains)
			})
		}
	})

	t.Run("設定した無料上限まで対戦済みのとき、さらに対戦回数を増やすと 429 になり回数は増えない", func(t *testing.T) {
		sharedPg.Truncate(t)
		seedPlayer(t, contractPlayerID, "uid-1")

		gameConfig := defaultContractGameConfig()
		gameConfig.values[contractFreeDailyBattleLimitKey] = 1
		engine := newContractEngineWithGameConfig(contractPlayerID, gameConfig)

		first := httptest.NewRequest(http.MethodPost, "/me/battle-limit/increment", nil)
		firstW := httptest.NewRecorder()
		engine.ServeHTTP(firstW, first)
		require.Equal(t, http.StatusNoContent, firstW.Code)

		second := httptest.NewRequest(http.MethodPost, "/me/battle-limit/increment", nil)
		secondW := httptest.NewRecorder()
		engine.ServeHTTP(secondW, second)
		assert.Equal(t, http.StatusTooManyRequests, secondW.Code)
		assert.Contains(t, secondW.Body.String(), usecase.ErrBattleLimitExceeded.Error())

		getReq := httptest.NewRequest(http.MethodGet, "/me/battle-limit", nil)
		getW := httptest.NewRecorder()
		engine.ServeHTTP(getW, getReq)
		assert.Equal(t, http.StatusOK, getW.Code)
		assert.Contains(t, getW.Body.String(), `"daily_battle_count":1`)
	})

	t.Run("bgm_volume だけを指定して更新すると、応答ボディの bgm_volume が更新され他の設定は維持される", func(t *testing.T) {
		sharedPg.Truncate(t)
		seedPlayer(t, contractPlayerID, "uid-1")
		seedPlayerSettings(t, contractPlayerID, "en", 20, 40, false)

		req := httptest.NewRequest(http.MethodPut, "/me/settings", strings.NewReader(`{"bgm_volume":50}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		newContractEngine(contractPlayerID).ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Body.String(), `"bgm_volume":50`)
		assert.Contains(t, w.Body.String(), `"language":"en"`)
		assert.Contains(t, w.Body.String(), `"se_volume":40`)
		assert.Contains(t, w.Body.String(), `"push_enabled":false`)
	})
}

func TestHandlerInputGuards(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("必須フィールド空文字ガードの 400 応答", func(t *testing.T) {
		tests := []struct {
			name             string
			method           string
			path             string
			body             string
			wantBodyContains string
		}{
			{
				name:             "firebase_uid が空文字の登録要求は、400 になる",
				method:           http.MethodPost,
				path:             "/auth/register",
				body:             `{"firebase_uid":""}`,
				wantBodyContains: "firebase_uid is required",
			},
			{
				name:             "firebase_uid が空文字のログイン要求は、400 になる",
				method:           http.MethodPost,
				path:             "/auth/login",
				body:             `{"firebase_uid":""}`,
				wantBodyContains: "firebase_uid is required",
			},
			{
				name:             "faction_id が空文字の初期陣営選択は、400 になる",
				method:           http.MethodPost,
				path:             "/me/factions/select",
				body:             `{"faction_id":""}`,
				wantBodyContains: "faction_id is required",
			},
			{
				name:             "faction が空文字の陣営付与は、400 になる",
				method:           http.MethodPost,
				path:             "/me/factions",
				body:             `{"faction":""}`,
				wantBodyContains: "faction is required",
			},
			{
				name:             "game_id が空文字の消費バトル回数返却は、400 になる",
				method:           http.MethodPost,
				path:             "/internal/v1/players/revert-battle-count",
				body:             `{"game_id":"","player1_id":"p1","player2_id":"p2","consumed_at_millis":1700000000000}`,
				wantBodyContains: "game_id is required",
			},
			{
				name:             "player1_id が空文字の消費バトル回数返却は、400 になる",
				method:           http.MethodPost,
				path:             "/internal/v1/players/revert-battle-count",
				body:             `{"game_id":"g1","player1_id":"","player2_id":"p2","consumed_at_millis":1700000000000}`,
				wantBodyContains: "player1_id is required",
			},
			{
				name:             "player2_id が空文字の消費バトル回数返却は、400 になる",
				method:           http.MethodPost,
				path:             "/internal/v1/players/revert-battle-count",
				body:             `{"game_id":"g1","player1_id":"p1","player2_id":"","consumed_at_millis":1700000000000}`,
				wantBodyContains: "player2_id is required",
			},
			{
				name:             "consumed_at_millis が 0 の消費バトル回数返却は、400 になる",
				method:           http.MethodPost,
				path:             "/internal/v1/players/revert-battle-count",
				body:             `{"game_id":"g1","player1_id":"p1","player2_id":"p2","consumed_at_millis":0}`,
				wantBodyContains: "consumed_at_millis must be positive",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				newContractEngine(contractPlayerID).ServeHTTP(w, req)

				assert.Equal(t, http.StatusBadRequest, w.Code)
				assert.Contains(t, w.Body.String(), tt.wantBodyContains)
			})
		}
	})

	t.Run("壊れた JSON を送ったときの 400 応答", func(t *testing.T) {
		tests := []struct {
			name   string
			method string
			path   string
		}{
			{name: "登録ルートのとき", method: http.MethodPost, path: "/auth/register"},
			{name: "ログインルートのとき", method: http.MethodPost, path: "/auth/login"},
			{name: "表示名更新ルートのとき", method: http.MethodPut, path: "/me/name"},
			{name: "表示名検証ルートのとき", method: http.MethodPost, path: "/me/onboarding/name/validate"},
			{name: "プレミアム更新ルートのとき", method: http.MethodPut, path: "/me/premium"},
			{name: "経験値加算ルートのとき", method: http.MethodPost, path: "/me/exp"},
			{name: "対戦結果送信ルートのとき", method: http.MethodPost, path: "/internal/v1/players/award-game-exp"},
			{name: "消費バトル回数返却ルートのとき", method: http.MethodPost, path: "/internal/v1/players/revert-battle-count"},
			{name: "初期陣営選択ルートのとき", method: http.MethodPost, path: "/me/factions/select"},
			{name: "陣営付与ルートのとき", method: http.MethodPost, path: "/me/factions"},
			{name: "設定更新ルートのとき", method: http.MethodPut, path: "/me/settings"},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(`{`))
				req.Header.Set("Content-Type", "application/json")
				w := httptest.NewRecorder()
				newContractEngine(contractPlayerID).ServeHTTP(w, req)

				assert.Equal(t, http.StatusBadRequest, w.Code)
			})
		}
	})
}
