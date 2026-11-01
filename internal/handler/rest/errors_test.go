package rest_test

import (
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/logging/loggingtest"
	"github.com/kenyamaneko/overload-party-account/internal/handler/handlertest"
	"github.com/kenyamaneko/overload-party-account/internal/handler/rest/resttest"
	"github.com/kenyamaneko/overload-party-account/internal/router/routertest"
)

var errStorageFailed = errors.New("storage-unavailable")

func TestErrorStatusAndLog(t *testing.T) {
	t.Run("[エラー応答]エラーのHTTPステータスとログ", func(t *testing.T) {
		config := map[string]int64{routertest.ConfigKeyExpCoefficient: 100}
		t.Run("異常系", func(t *testing.T) {
			t.Run("未登録のFirebaseUIDでログインすると、プレイヤー不在を示す404の応答になる", func(t *testing.T) {
				s := routertest.NewSeededServer(config)
				rec := s.Call(t, http.MethodPost, "/internal/v1/auth/login", routertest.NoAuth, `{"firebase_uid":"firebase-unknown"}`)
				handlertest.AssertErrorResponse(t, rec, http.StatusNotFound, "player not found")
			})
			t.Run("存在しないプレイヤーのとき、プレイヤー情報を取得すると", func(t *testing.T) {
				t.Run("プレイヤー不在を示す404の応答になる", func(t *testing.T) {
					s := routertest.NewSeededServer(config)
					handlertest.AssertErrorResponse(t, s.GetPlayer(t, routertest.TokenA), http.StatusNotFound, "player not found")
				})
				t.Run("Error以上のレベルのログは出ない", func(t *testing.T) {
					logs := loggingtest.Capture(t)
					s := routertest.NewSeededServer(config)
					s.GetPlayer(t, routertest.TokenA)
					assert.Empty(t, logs.SelectAtLeastLevel(slog.LevelError))
				})
			})
			t.Run("登録済みのFirebaseUIDで新規登録すると、登録済みを示す409の応答になる", func(t *testing.T) {
				s := routertest.NewSeededServer(config, routertest.NewPlayerSeedA())
				rec := s.Call(t, http.MethodPost, "/internal/v1/auth/register", routertest.NoAuth, `{"firebase_uid":"`+routertest.FirebaseUIDA+`"}`)
				handlertest.AssertErrorResponse(t, rec, http.StatusConflict, "player already registered")
			})
			t.Run("初期陣営を選択済みのプレイヤーが初期陣営を選び直すと、初期陣営選択済みを示す409の応答になる", func(t *testing.T) {
				s := routertest.NewSeededServer(config, routertest.NewPlayerSeedA())
				s.Memory.AddInitialFaction(routertest.PlayerAID, "SHE")
				rec := s.PostAs(t, routertest.TokenA, "/api/v1/account/me/factions/select", `{"faction_id":"Tenki"}`)
				handlertest.AssertErrorResponse(t, rec, http.StatusConflict, "initial faction already selected")
			})
			t.Run("無料の1日のバトル回数の上限に達したプレイヤーがバトル回数を加算すると、上限超過を示す429の応答になる", func(t *testing.T) {
				s := routertest.NewSeededServer(map[string]int64{routertest.ConfigKeyFreeDailyBattles: 3}, routertest.NewPlayerSeedA())
				for range 3 {
					s.PostAs(t, routertest.TokenA, "/api/v1/account/me/battle-limit/increment", "")
				}
				rec := s.PostAs(t, routertest.TokenA, "/api/v1/account/me/battle-limit/increment", "")
				handlertest.AssertErrorResponse(t, rec, http.StatusTooManyRequests, "daily battle limit exceeded")
			})
			t.Run("選択対象外の陣営Neutralを初期陣営に選ぶと、陣営不正を示す400の応答になる", func(t *testing.T) {
				s := routertest.NewSeededServer(config, routertest.NewPlayerSeedA())
				rec := s.PostAs(t, routertest.TokenA, "/api/v1/account/me/factions/select", `{"faction_id":"Neutral"}`)
				resttest.AssertErrorResponseWithPrefix(t, rec, http.StatusBadRequest, "invalid faction")
			})
			getPlayerWithStorageError := func(t *testing.T) (*loggingtest.Logs, *httptest.ResponseRecorder) {
				logs := loggingtest.Capture(t)
				s := routertest.NewServer(routertest.Options{
					GameConfig:      config,
					Tokens:          map[string]string{routertest.TokenA: routertest.PlayerAID},
					ViewRepoFailure: errStorageFailed,
				})
				return logs, s.GetPlayer(t, routertest.TokenA)
			}
			getPlayerWithoutGameConfig := func(t *testing.T) (*loggingtest.Logs, *httptest.ResponseRecorder) {
				logs := loggingtest.Capture(t)
				s := routertest.NewSeededServer(map[string]int64{}, routertest.NewPlayerSeedA())
				return logs, s.GetPlayer(t, routertest.TokenA)
			}
			getPlayerWhenStorageCrashes := func(t *testing.T) (*loggingtest.Logs, *httptest.ResponseRecorder) {
				logs := loggingtest.Capture(t)
				s := routertest.NewServer(routertest.Options{
					GameConfig:    config,
					Tokens:        map[string]string{routertest.TokenA: routertest.PlayerAID},
					ViewRepoPanic: "storage-crashed",
				})
				return logs, s.GetPlayer(t, routertest.TokenA)
			}
			t.Run("プレイヤー情報を取得したときの応答", func(t *testing.T) {
				cases := []struct {
					name      string
					getPlayer func(t *testing.T) (*loggingtest.Logs, *httptest.ResponseRecorder)
				}{
					{name: "プレイヤー情報の保存先がstorage-unavailableというエラーを返すとき、サーバー内部エラーを示す500の応答になる", getPlayer: getPlayerWithStorageError},
					{name: "ゲーム設定が存在しないとき、サーバー内部エラーを示す500の応答になる", getPlayer: getPlayerWithoutGameConfig},
					{name: "プレイヤー情報の保存先が処理中に異常終了するとき、サーバー内部エラーを示す500の応答になる", getPlayer: getPlayerWhenStorageCrashes},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						_, rec := tc.getPlayer(t)
						handlertest.AssertErrorResponse(t, rec, http.StatusInternalServerError, "internal server error")
					})
				}
			})
			t.Run("プレイヤー情報を取得したときのログ", func(t *testing.T) {
				cases := []struct {
					name      string
					getPlayer func(t *testing.T) (*loggingtest.Logs, *httptest.ResponseRecorder)
				}{
					{name: "プレイヤー情報の保存先がstorage-unavailableというエラーを返すとき、Errorレベルの内部エラーのログが1件出る", getPlayer: getPlayerWithStorageError},
					{name: "プレイヤー情報の保存先が処理中に異常終了するとき、Errorレベルの内部エラーのログが1件出る", getPlayer: getPlayerWhenStorageCrashes},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						logs, _ := tc.getPlayer(t)
						assert.Equal(t, "handler internal error", resttest.SelectLoggedError(t, logs).Message)
					})
				}
			})
			t.Run("プレイヤー情報の保存先がstorage-unavailableというエラーを返すとき、プレイヤー情報を取得すると", func(t *testing.T) {
				t.Run("ログのリクエストのメソッドがGETになる", func(t *testing.T) {
					logs, _ := getPlayerWithStorageError(t)
					assert.Equal(t, "GET", resttest.SelectLoggedError(t, logs).String("method"))
				})
				t.Run("ログのリクエストのパスが／api／v1／account／meになる", func(t *testing.T) {
					logs, _ := getPlayerWithStorageError(t)
					assert.Equal(t, "/api/v1/account/me", resttest.SelectLoggedError(t, logs).String("path"))
				})
				t.Run("ログの応答のステータスが500になる", func(t *testing.T) {
					logs, _ := getPlayerWithStorageError(t)
					assert.EqualValues(t, 500, resttest.SelectLoggedError(t, logs).Any("status"))
				})
				t.Run("ログのエラーの内容がstorage-unavailableを含む", func(t *testing.T) {
					logs, _ := getPlayerWithStorageError(t)
					assert.Contains(t, resttest.SelectLoggedError(t, logs).String("error"), "storage-unavailable")
				})
			})
			t.Run("ゲーム設定が存在しないとき、プレイヤー情報を取得すると、ログのエラーの内容がゲーム設定が存在しないことを示す文面を含む", func(t *testing.T) {
				logs, _ := getPlayerWithoutGameConfig(t)
				assert.Contains(t, resttest.SelectLoggedError(t, logs).String("error"), "game config not found")
			})
			t.Run("プレイヤー情報の保存先が処理中に異常終了するとき、プレイヤー情報を取得すると、ログのエラーの内容が異常終了の内容storage-crashedを含む", func(t *testing.T) {
				logs, _ := getPlayerWhenStorageCrashes(t)
				assert.Contains(t, resttest.SelectLoggedError(t, logs).String("error"), "storage-crashed")
			})
		})
	})
}
