package router_test

import (
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/handler/handlertest"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush/pubsubpushtest"
	"github.com/kenyamaneko/overload-party-account/internal/router/routertest"
)

var errHandlerFailed = errors.New("event handler failed")

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	os.Exit(m.Run())
}

func ptr[T any](value T) *T {
	return &value
}

func newRoutingTestServer() *routertest.Server {
	s := routertest.NewServer(routertest.Options{
		GameConfig: map[string]int64{
			routertest.ConfigKeyExpCoefficient:   100,
			routertest.ConfigKeyFreeDailyBattles: 3,
			routertest.ConfigKeyExpWin:           100,
			routertest.ConfigKeyExpLoss:          20,
			routertest.ConfigKeyExpDraw:          50,
		},
		Tokens: map[string]string{routertest.TokenA: routertest.PlayerAID, routertest.TokenB: routertest.PlayerBID},
	})
	firstPlayer := routertest.NewPlayerSeedA()
	firstPlayer.Name = ptr("太郎")
	secondPlayer := routertest.NewPlayerSeedB()
	secondPlayer.Name = ptr("次郎")
	s.Memory.AddPlayer(firstPlayer)
	s.Memory.AddPlayer(secondPlayer)
	s.Memory.AddSettings(domain.PlayerSettings{
		PlayerID:      routertest.PlayerAID,
		Language:      "ja",
		BgmVolume:     50,
		SeVolume:      50,
		IsPushEnabled: true,
		UpdatedAt:     routertest.FixedNow,
	})
	return s
}

func TestHealthCheck(t *testing.T) {
	t.Run("[死活監視API]ヘルスチェック", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("X-Internal-Authヘッダが無いとき", func(t *testing.T) {
				t.Run("ステータスが200になる", func(t *testing.T) {
					rec := newRoutingTestServer().Call(t, http.MethodGet, "/health", routertest.NoAuth, "")
					handlertest.AssertStatus(t, rec, http.StatusOK)
				})
				t.Run("応答のstatusがokになる", func(t *testing.T) {
					rec := newRoutingTestServer().Call(t, http.MethodGet, "/health", routertest.NoAuth, "")
					handlertest.AssertJSONField(t, rec, "status", `"ok"`)
				})
			})
		})
	})
}

func TestInternalAuthPlacement(t *testing.T) {
	t.Run("[内部API認証]認証が必要な経路と不要な経路", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("X-Internal-Authヘッダ付きで呼ぶとき", func(t *testing.T) {
				t.Run("プレイヤー情報を取得するとき", func(t *testing.T) {
					t.Run("トークンAがプレイヤーAのトークンとして有効なとき、応答のplayer_idがaaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaaになる", func(t *testing.T) {
						rec := newRoutingTestServer().GetPlayer(t, routertest.TokenA)
						handlertest.AssertJSONField(t, rec, "player_id", `"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"`)
					})
					t.Run("トークンBがプレイヤーBのトークンとして有効なとき、応答のplayer_idがbbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbbになる", func(t *testing.T) {
						rec := newRoutingTestServer().GetPlayer(t, routertest.TokenB)
						handlertest.AssertJSONField(t, rec, "player_id", `"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"`)
					})
				})
			})
			t.Run("X-Internal-Authヘッダが無いとき", func(t *testing.T) {
				t.Run("新規登録のとき、ステータスが201になる", func(t *testing.T) {
					rec := newRoutingTestServer().Call(t, http.MethodPost, "/internal/v1/auth/register", routertest.NoAuth, `{"firebase_uid":"firebase-new"}`)
					handlertest.AssertStatus(t, rec, http.StatusCreated)
				})
				t.Run("ログインのとき、ステータスが200になる", func(t *testing.T) {
					rec := newRoutingTestServer().Call(t, http.MethodPost, "/internal/v1/auth/login", routertest.NoAuth, `{"firebase_uid":"`+routertest.FirebaseUIDA+`"}`)
					handlertest.AssertStatus(t, rec, http.StatusOK)
				})
				t.Run("FirebaseUIDによるプレイヤー検索のとき、ステータスが200になる", func(t *testing.T) {
					rec := newRoutingTestServer().Call(t, http.MethodGet, "/internal/v1/auth/by-firebase-uid/"+routertest.FirebaseUIDA, routertest.NoAuth, "")
					handlertest.AssertStatus(t, rec, http.StatusOK)
				})
				t.Run("指定プレイヤーの所持陣営一覧のとき、ステータスが200になる", func(t *testing.T) {
					rec := newRoutingTestServer().Call(t, http.MethodGet, "/internal/v1/players/"+routertest.PlayerAID+"/factions", routertest.NoAuth, "")
					handlertest.AssertStatus(t, rec, http.StatusOK)
				})
				t.Run("ゲーム終了時の経験値付与のとき、ステータスが204になる", func(t *testing.T) {
					body := `{"player1_id":"` + routertest.PlayerAID + `","player2_id":"` + routertest.PlayerBID + `","winner_num":1,"reason":"test-reason","match_type":"pvp"}`
					rec := newRoutingTestServer().Call(t, http.MethodPost, "/internal/v1/players/award-game-exp", routertest.NoAuth, body)
					handlertest.AssertStatus(t, rec, http.StatusNoContent)
				})
				t.Run("停止で無効になった対戦の消費バトル回数の返却のとき、ステータスが204になる", func(t *testing.T) {
					body := `{"game_id":"game-1","player1_id":"` + routertest.PlayerAID + `","player2_id":"` + routertest.PlayerBID + `","consumed_at_millis":1}`
					rec := newRoutingTestServer().Call(t, http.MethodPost, "/internal/v1/players/revert-battle-count", routertest.NoAuth, body)
					handlertest.AssertStatus(t, rec, http.StatusNoContent)
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("表示名が次郎のプレイヤーBのplayer_idをリクエストボディに含めて、トークンAのプレイヤーの表示名を花子に変更するとき", func(t *testing.T) {
				rename := func(t *testing.T) *routertest.Server {
					s := newRoutingTestServer()
					s.PutAs(t, routertest.TokenA, "/api/v1/account/me/name", `{"player_id":"`+routertest.PlayerBID+`","name":"花子"}`)
					return s
				}

				t.Run("続けてトークンAでプレイヤー情報を取得すると、nameが花子になる", func(t *testing.T) {
					handlertest.AssertJSONField(t, rename(t).GetPlayer(t, routertest.TokenA), "name", `"花子"`)
				})
				t.Run("続けてトークンBでプレイヤー情報を取得すると、nameが次郎のままになる", func(t *testing.T) {
					handlertest.AssertJSONField(t, rename(t).GetPlayer(t, routertest.TokenB), "name", `"次郎"`)
				})
			})
			t.Run("X-Internal-Authヘッダが無いとき", func(t *testing.T) {
				t.Run("表示名の変更のとき、ヘッダ不足を示す401の応答になる", func(t *testing.T) {
					rec := newRoutingTestServer().Call(t, http.MethodPut, "/api/v1/account/me/name", routertest.NoAuth, `{"name":"花子"}`)
					handlertest.AssertErrorResponse(t, rec, http.StatusUnauthorized, "X-Internal-Auth header is required")
				})
				t.Run("オンボーディング表示名の検証のとき、ヘッダ不足を示す401の応答になる", func(t *testing.T) {
					rec := newRoutingTestServer().Call(t, http.MethodPost, "/api/v1/account/me/onboarding/name/validate", routertest.NoAuth, `{"name":"花子"}`)
					handlertest.AssertErrorResponse(t, rec, http.StatusUnauthorized, "X-Internal-Auth header is required")
				})
				t.Run("バトル回数制限の取得のとき、ヘッダ不足を示す401の応答になる", func(t *testing.T) {
					rec := newRoutingTestServer().Call(t, http.MethodGet, "/api/v1/account/me/battle-limit", routertest.NoAuth, "")
					handlertest.AssertErrorResponse(t, rec, http.StatusUnauthorized, "X-Internal-Auth header is required")
				})
				t.Run("バトル回数の加算のとき、ヘッダ不足を示す401の応答になる", func(t *testing.T) {
					rec := newRoutingTestServer().Call(t, http.MethodPost, "/api/v1/account/me/battle-limit/increment", routertest.NoAuth, "")
					handlertest.AssertErrorResponse(t, rec, http.StatusUnauthorized, "X-Internal-Auth header is required")
				})
				t.Run("プレミアム状態の更新のとき、ヘッダ不足を示す401の応答になる", func(t *testing.T) {
					rec := newRoutingTestServer().Call(t, http.MethodPut, "/api/v1/account/me/premium", routertest.NoAuth, `{"is_premium":true}`)
					handlertest.AssertErrorResponse(t, rec, http.StatusUnauthorized, "X-Internal-Auth header is required")
				})
				t.Run("経験値の加算のとき、ヘッダ不足を示す401の応答になる", func(t *testing.T) {
					rec := newRoutingTestServer().Call(t, http.MethodPost, "/api/v1/account/me/exp", routertest.NoAuth, `{"exp_gain":10}`)
					handlertest.AssertErrorResponse(t, rec, http.StatusUnauthorized, "X-Internal-Auth header is required")
				})
				t.Run("所持陣営の付与のとき、ヘッダ不足を示す401の応答になる", func(t *testing.T) {
					rec := newRoutingTestServer().Call(t, http.MethodPost, "/api/v1/account/me/factions", routertest.NoAuth, `{"faction":"Sugar"}`)
					handlertest.AssertErrorResponse(t, rec, http.StatusUnauthorized, "X-Internal-Auth header is required")
				})
				t.Run("所持陣営一覧のとき、ヘッダ不足を示す401の応答になる", func(t *testing.T) {
					rec := newRoutingTestServer().Call(t, http.MethodGet, "/api/v1/account/me/factions", routertest.NoAuth, "")
					handlertest.AssertErrorResponse(t, rec, http.StatusUnauthorized, "X-Internal-Auth header is required")
				})
				t.Run("初期陣営の選択のとき、ヘッダ不足を示す401の応答になる", func(t *testing.T) {
					rec := newRoutingTestServer().Call(t, http.MethodPost, "/api/v1/account/me/factions/select", routertest.NoAuth, `{"faction_id":"SHE"}`)
					handlertest.AssertErrorResponse(t, rec, http.StatusUnauthorized, "X-Internal-Auth header is required")
				})
				t.Run("プレイヤー設定の取得のとき、ヘッダ不足を示す401の応答になる", func(t *testing.T) {
					rec := newRoutingTestServer().Call(t, http.MethodGet, "/api/v1/account/me/settings", routertest.NoAuth, "")
					handlertest.AssertErrorResponse(t, rec, http.StatusUnauthorized, "X-Internal-Auth header is required")
				})
				t.Run("プレイヤー設定の更新のとき、ヘッダ不足を示す401の応答になる", func(t *testing.T) {
					rec := newRoutingTestServer().Call(t, http.MethodPut, "/api/v1/account/me/settings", routertest.NoAuth, `{"language":"en"}`)
					handlertest.AssertErrorResponse(t, rec, http.StatusUnauthorized, "X-Internal-Auth header is required")
				})
				t.Run("プレイヤー情報の取得のとき、ヘッダ不足を示す401の応答になる", func(t *testing.T) {
					rec := newRoutingTestServer().Call(t, http.MethodGet, "/api/v1/account/me", routertest.NoAuth, "")
					handlertest.AssertErrorResponse(t, rec, http.StatusUnauthorized, "X-Internal-Auth header is required")
				})
			})
			t.Run("プレイヤー情報の取得で、トークンが無効なとき、トークン無効を示す401の応答になる", func(t *testing.T) {
				rec := newRoutingTestServer().GetPlayer(t, "token-unknown")
				handlertest.AssertErrorResponse(t, rec, http.StatusUnauthorized, "invalid internal auth token")
			})
		})
	})
}

func TestPubsubEventEndpoints(t *testing.T) {
	t.Run("[イベント受信API]イベント別の受信", func(t *testing.T) {
		t.Run("異常系", func(t *testing.T) {
			t.Run("X-Internal-Authヘッダが無く、正しい形式のメッセージを送るとき", func(t *testing.T) {
				receiveWithFailingEvent := func(t *testing.T, path, failingEvent string) {
					s := routertest.NewServer(routertest.Options{HandlerFailures: map[string]error{failingEvent: errHandlerFailed}})
					rec := s.Call(t, http.MethodPost, path, routertest.NoAuth, pubsubpushtest.BuildEventEnvelope(t, "{}", "m-1"))
					handlertest.AssertErrorResponse(t, rec, http.StatusInternalServerError, "handler failed")
				}

				t.Run("陣営獲得の処理だけが失敗するとき、陣営獲得の受信は処理失敗を示す500の応答になる", func(t *testing.T) {
					receiveWithFailingEvent(t, "/internal/v1/pubsub/faction-acquired", routertest.SubscriptionFactionAcquired)
				})
				t.Run("プレミアム状態更新の処理だけが失敗するとき、プレミアム状態更新の受信は処理失敗を示す500の応答になる", func(t *testing.T) {
					receiveWithFailingEvent(t, "/internal/v1/pubsub/premium-updated", routertest.SubscriptionPremiumUpdated)
				})
				t.Run("プレイヤーのオンボーディング完了の処理だけが失敗するとき、オンボーディング完了の受信は処理失敗を示す500の応答になる", func(t *testing.T) {
					receiveWithFailingEvent(t, "/internal/v1/pubsub/player-onboarded", routertest.SubscriptionPlayerOnboarded)
				})
				t.Run("オンボーディングの表示名設定の処理だけが失敗するとき、表示名設定の受信は処理失敗を示す500の応答になる", func(t *testing.T) {
					receiveWithFailingEvent(t, "/internal/v1/pubsub/onboarding-name-set", routertest.SubscriptionOnboardingNameSet)
				})
				t.Run("オンボーディングの陣営設定の処理だけが失敗するとき、陣営設定の受信は処理失敗を示す500の応答になる", func(t *testing.T) {
					receiveWithFailingEvent(t, "/internal/v1/pubsub/onboarding-faction-set", routertest.SubscriptionOnboardingFactionSet)
				})
			})
		})
	})
}
