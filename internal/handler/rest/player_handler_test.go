package rest_test

import (
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/logging/loggingtest"
	"github.com/kenyamaneko/overload-party-account/internal/handler/handlertest"
	"github.com/kenyamaneko/overload-party-account/internal/handler/rest/resttest"
	"github.com/kenyamaneko/overload-party-account/internal/router/routertest"
)

func ptr[T any](value T) *T {
	return &value
}

func assertEmptyBody(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	assert.Empty(t, rec.Body.String())
}

func assertFieldAbsent(t *testing.T, rec *httptest.ResponseRecorder, field string) {
	t.Helper()
	assert.NotContains(t, handlertest.DecodeObject(t, rec), field)
}

func assertTimeField(t *testing.T, rec *httptest.ResponseRecorder, field string, expected time.Time) {
	t.Helper()
	var actual time.Time
	require.NoError(t, json.Unmarshal(handlertest.DecodeObject(t, rec)[field], &actual))
	assert.True(t, expected.Equal(actual), "expected %s but got %s", expected, actual)
}

func buildBodyWith(t *testing.T, base map[string]any, overrides map[string]any) string {
	t.Helper()
	merged := maps.Clone(base)
	maps.Copy(merged, overrides)
	return marshalBody(t, merged)
}

func buildBodyWithout(t *testing.T, base map[string]any, keys ...string) string {
	t.Helper()
	trimmed := maps.Clone(base)
	for _, key := range keys {
		delete(trimmed, key)
	}
	return marshalBody(t, trimmed)
}

func marshalBody(t *testing.T, body map[string]any) string {
	t.Helper()
	encoded, err := json.Marshal(body)
	require.NoError(t, err)
	return string(encoded)
}

const (
	incrementBattleCountPath = "/api/v1/account/me/battle-limit/increment"
	revertBattleCountPath    = "/internal/v1/players/revert-battle-count"
	awardGameExpPath         = "/internal/v1/players/award-game-exp"
)

var (
	revertBattleCountBase = map[string]any{
		"game_id":            "game-1",
		"player1_id":         routertest.PlayerAID,
		"player2_id":         routertest.PlayerBID,
		"consumed_at_millis": routertest.FixedNow.UnixMilli(),
	}
	awardGameExpBase = map[string]any{
		"player1_id": routertest.PlayerAID,
		"player2_id": routertest.PlayerBID,
		"winner_num": 1,
		"reason":     "test-reason",
		"match_type": "pvp",
	}
)

func awardGameExpConfig() map[string]int64 {
	return map[string]int64{
		routertest.ConfigKeyExpCoefficient: 100,
		routertest.ConfigKeyExpWin:         100,
		routertest.ConfigKeyExpLoss:        20,
		routertest.ConfigKeyExpDraw:        50,
	}
}

func TestPlayerInfoResponseAndLog(t *testing.T) {
	t.Run("[プレイヤー情報API]プレイヤー情報の応答とログ", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("設定した経験値係数が100で、経験値500のプレイヤーを取得するとき", func(t *testing.T) {
				getPlayer := func(t *testing.T) *httptest.ResponseRecorder {
					seed := routertest.NewPlayerSeedA()
					seed.Exp = 500
					s := routertest.NewSeededServer(map[string]int64{routertest.ConfigKeyExpCoefficient: 100}, seed)
					return s.GetPlayer(t, routertest.TokenA)
				}

				t.Run("応答のlevelが2になる", func(t *testing.T) {
					handlertest.AssertJSONField(t, getPlayer(t), "level", `2`)
				})
				t.Run("応答のlevel_exp_currentが100になる", func(t *testing.T) {
					handlertest.AssertJSONField(t, getPlayer(t), "level_exp_current", `100`)
				})
				t.Run("応答のlevel_exp_requiredが500になる", func(t *testing.T) {
					handlertest.AssertJSONField(t, getPlayer(t), "level_exp_required", `500`)
				})
			})
			t.Run("保存されているオンボーディング進行状態が次の値のプレイヤーを取得するとき", func(t *testing.T) {
				cases := []struct {
					name         string
					status       string
					expectedJSON string
				}{
					{name: "name_setのとき、応答のonboarding_statusがname_setになる", status: "name_set", expectedJSON: `"name_set"`},
					{name: "faction_setのとき、応答のonboarding_statusがfaction_setになる", status: "faction_set", expectedJSON: `"faction_set"`},
					{name: "completedのとき、応答のonboarding_statusがcompletedになる", status: "completed", expectedJSON: `"completed"`},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						seed := routertest.NewPlayerSeedA()
						seed.OnboardingStatus = tc.status
						s := routertest.NewSeededServer(map[string]int64{routertest.ConfigKeyExpCoefficient: 100}, seed)
						handlertest.AssertJSONField(t, s.GetPlayer(t, routertest.TokenA), "onboarding_status", tc.expectedJSON)
					})
				}
			})
		})
		t.Run("異常系", func(t *testing.T) {
			getPlayerWithNegativeCoefficient := func(t *testing.T) (*loggingtest.Logs, *httptest.ResponseRecorder) {
				logs := loggingtest.Capture(t)
				seed := routertest.NewPlayerSeedA()
				seed.Exp = 0
				s := routertest.NewSeededServer(map[string]int64{routertest.ConfigKeyExpCoefficient: -1}, seed)
				return logs, s.GetPlayer(t, routertest.TokenA)
			}
			t.Run("設定した経験値係数が-1のときにプレイヤー情報を取得すると、サーバー内部エラーを示す500の応答になる", func(t *testing.T) {
				_, rec := getPlayerWithNegativeCoefficient(t)
				handlertest.AssertErrorResponse(t, rec, http.StatusInternalServerError, "internal server error")
			})
			t.Run("設定した経験値係数が-1のときにプレイヤー情報を取得すると、ログのエラーの内容が経験値係数が1未満であることを示す文面を含む", func(t *testing.T) {
				logs, _ := getPlayerWithNegativeCoefficient(t)
				assert.Contains(t, resttest.SelectLoggedError(t, logs).String("error"), "exp coefficient is below the minimum")
			})
		})
	})
}

func TestUpdatePlayerName(t *testing.T) {
	t.Run("[表示名API]表示名の変更", func(t *testing.T) {
		config := map[string]int64{routertest.ConfigKeyExpCoefficient: 100}
		t.Run("正常系", func(t *testing.T) {
			t.Run("表示名を花子に変更するとき", func(t *testing.T) {
				update := func(t *testing.T) (*routertest.Server, *httptest.ResponseRecorder) {
					s := routertest.NewSeededServer(config, routertest.NewPlayerSeedA())
					return s, s.PutAs(t, routertest.TokenA, "/api/v1/account/me/name", `{"name":"花子"}`)
				}

				t.Run("ステータスが200になる", func(t *testing.T) {
					_, rec := update(t)
					handlertest.AssertStatus(t, rec, http.StatusOK)
				})
				t.Run("応答のnameが花子になる", func(t *testing.T) {
					_, rec := update(t)
					handlertest.AssertJSONField(t, rec, "name", `"花子"`)
				})
				t.Run("続けてプレイヤー情報を取得すると、nameが花子になる", func(t *testing.T) {
					s, _ := update(t)
					handlertest.AssertJSONField(t, s.GetPlayer(t, routertest.TokenA), "name", `"花子"`)
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("nameが指定されないとき", func(t *testing.T) {
				cases := []struct {
					name string
					body string
				}{
					{name: "空文字のとき、表示名不正を示す400の応答になる", body: `{"name":""}`},
					{name: "キーが無いとき、表示名不正を示す400の応答になる", body: `{}`},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						s := routertest.NewSeededServer(config, routertest.NewPlayerSeedA())
						resttest.AssertErrorResponseWithPrefix(t, s.PutAs(t, routertest.TokenA, "/api/v1/account/me/name", tc.body), http.StatusBadRequest, "invalid name")
					})
				}
			})
			t.Run("リクエストボディが無いとき、リクエストボディ不正を示す400の応答になる", func(t *testing.T) {
				s := routertest.NewSeededServer(config, routertest.NewPlayerSeedA())
				handlertest.AssertErrorResponse(t, s.PutAs(t, routertest.TokenA, "/api/v1/account/me/name", ""), http.StatusBadRequest, "invalid request body")
			})
			t.Run("表示名が花子のプレイヤーが空文字の表示名に変更しようとしたあと、続けてプレイヤー情報を取得すると、nameが花子のままになる", func(t *testing.T) {
				seed := routertest.NewPlayerSeedA()
				seed.Name = ptr("花子")
				s := routertest.NewSeededServer(config, seed)
				s.PutAs(t, routertest.TokenA, "/api/v1/account/me/name", `{"name":""}`)
				handlertest.AssertJSONField(t, s.GetPlayer(t, routertest.TokenA), "name", `"花子"`)
			})
		})
	})
}

func TestValidateOnboardingName(t *testing.T) {
	t.Run("[表示名API]オンボーディング表示名の検証", func(t *testing.T) {
		config := map[string]int64{routertest.ConfigKeyExpCoefficient: 100}
		validatePath := "/api/v1/account/me/onboarding/name/validate"
		t.Run("正常系", func(t *testing.T) {
			t.Run("表示名が未設定のプレイヤーが表示名を花子として検証するとき", func(t *testing.T) {
				validate := func(t *testing.T) (*routertest.Server, *httptest.ResponseRecorder) {
					s := routertest.NewSeededServer(config, routertest.NewPlayerSeedA())
					return s, s.PostAs(t, routertest.TokenA, validatePath, `{"name":"花子"}`)
				}

				t.Run("ステータスが204になる", func(t *testing.T) {
					_, rec := validate(t)
					handlertest.AssertStatus(t, rec, http.StatusNoContent)
				})
				t.Run("応答本文が空になる", func(t *testing.T) {
					_, rec := validate(t)
					assertEmptyBody(t, rec)
				})
				t.Run("続けてプレイヤー情報を取得しても、応答にnameが含まれない", func(t *testing.T) {
					s, _ := validate(t)
					assertFieldAbsent(t, s.GetPlayer(t, routertest.TokenA), "name")
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("nameが指定されないとき", func(t *testing.T) {
				cases := []struct {
					name string
					body string
				}{
					{name: "空文字のとき、表示名不正を示す400の応答になる", body: `{"name":""}`},
					{name: "キーが無いとき、表示名不正を示す400の応答になる", body: `{}`},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						s := routertest.NewSeededServer(config, routertest.NewPlayerSeedA())
						resttest.AssertErrorResponseWithPrefix(t, s.PostAs(t, routertest.TokenA, validatePath, tc.body), http.StatusBadRequest, "invalid name")
					})
				}
			})
			t.Run("リクエストボディが無いとき、リクエストボディ不正を示す400の応答になる", func(t *testing.T) {
				s := routertest.NewSeededServer(config, routertest.NewPlayerSeedA())
				handlertest.AssertErrorResponse(t, s.PostAs(t, routertest.TokenA, validatePath, ""), http.StatusBadRequest, "invalid request body")
			})
		})
	})
}

func TestBattleLimit(t *testing.T) {
	t.Run("[バトル回数API]バトル回数制限の取得と加算", func(t *testing.T) {
		freeConfig := map[string]int64{routertest.ConfigKeyFreeDailyBattles: 3}
		newFreeServer := func() *routertest.Server {
			return routertest.NewSeededServer(freeConfig, routertest.NewPlayerSeedA())
		}
		t.Run("正常系", func(t *testing.T) {
			t.Run("バトル回数を加算していないプレイヤーがバトル回数制限を取得すると、応答のdaily_battle_countが0になる", func(t *testing.T) {
				handlertest.AssertJSONField(t, newFreeServer().GetBattleLimit(t, routertest.TokenA), "daily_battle_count", `0`)
			})
			t.Run("無料プレイヤーが1日のバトル回数の上限を3回に設定した状態で、バトル回数を1回加算するとき", func(t *testing.T) {
				increment := func(t *testing.T) (*routertest.Server, *httptest.ResponseRecorder) {
					s := newFreeServer()
					return s, s.PostAs(t, routertest.TokenA, incrementBattleCountPath, "")
				}

				t.Run("加算のステータスが204になる", func(t *testing.T) {
					_, rec := increment(t)
					handlertest.AssertStatus(t, rec, http.StatusNoContent)
				})
				t.Run("続けてバトル回数制限を取得すると、daily_battle_limitが3になる", func(t *testing.T) {
					s, _ := increment(t)
					handlertest.AssertJSONField(t, s.GetBattleLimit(t, routertest.TokenA), "daily_battle_limit", `3`)
				})
				t.Run("続けてバトル回数制限を取得すると、can_battleがtrueになる", func(t *testing.T) {
					s, _ := increment(t)
					handlertest.AssertJSONField(t, s.GetBattleLimit(t, routertest.TokenA), "can_battle", `true`)
				})
			})
			t.Run("バトル回数を1回加算したあと、続けてバトル回数制限を取得すると、daily_battle_countが1になる", func(t *testing.T) {
				s := newFreeServer()
				s.PostAs(t, routertest.TokenA, incrementBattleCountPath, "")
				handlertest.AssertJSONField(t, s.GetBattleLimit(t, routertest.TokenA), "daily_battle_count", `1`)
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("存在しないプレイヤーがバトル回数制限を取得すると、プレイヤー不在を示す404の応答になる", func(t *testing.T) {
				s := routertest.NewServer(routertest.Options{
					GameConfig: freeConfig,
					Tokens:     map[string]string{routertest.TokenA: routertest.PlayerAID},
				})
				handlertest.AssertErrorResponse(t, s.GetBattleLimit(t, routertest.TokenA), http.StatusNotFound, "player not found")
			})
		})
	})
}

func TestRevertBattleCount(t *testing.T) {
	t.Run("[バトル回数API]停止で無効になった対戦の消費バトル回数の返却", func(t *testing.T) {
		newFreeServer := func() *routertest.Server {
			return routertest.NewSeededServer(
				map[string]int64{routertest.ConfigKeyFreeDailyBattles: 3},
				routertest.NewPlayerSeedA(),
				routertest.NewPlayerSeedB(),
			)
		}
		consumeBattleCount := func(t *testing.T, s *routertest.Server, times int) {
			for range times {
				s.PostAs(t, routertest.TokenA, incrementBattleCountPath, "")
				s.PostAs(t, routertest.TokenB, incrementBattleCountPath, "")
			}
		}
		t.Run("正常系", func(t *testing.T) {
			t.Run("プレイヤーAとプレイヤーBがそれぞれ当日にバトル回数を加算したとき", func(t *testing.T) {
				t.Run("1回ずつのとき", func(t *testing.T) {
					t.Run("停止で無効になった対戦の消費バトル回数を、消費時刻を1にして返却すると、ステータスが204になる", func(t *testing.T) {
						s := newFreeServer()
						consumeBattleCount(t, s, 1)
						body := buildBodyWith(t, revertBattleCountBase, map[string]any{"consumed_at_millis": 1})
						handlertest.AssertStatus(t, s.Call(t, http.MethodPost, revertBattleCountPath, routertest.NoAuth, body), http.StatusNoContent)
					})
					t.Run("停止で無効になった対戦の消費バトル回数を、バトル回数を加算した時刻を消費時刻に指定して返却すると", func(t *testing.T) {
						revert := func(t *testing.T) (*routertest.Server, *httptest.ResponseRecorder) {
							s := newFreeServer()
							consumeBattleCount(t, s, 1)
							return s, s.Call(t, http.MethodPost, revertBattleCountPath, routertest.NoAuth, buildBodyWith(t, revertBattleCountBase, nil))
						}

						t.Run("ステータスが204になる", func(t *testing.T) {
							_, rec := revert(t)
							handlertest.AssertStatus(t, rec, http.StatusNoContent)
						})
						t.Run("続けてプレイヤーAのバトル回数制限を取得すると、daily_battle_countが0になる", func(t *testing.T) {
							s, _ := revert(t)
							handlertest.AssertJSONField(t, s.GetBattleLimit(t, routertest.TokenA), "daily_battle_count", `0`)
						})
						t.Run("続けてプレイヤーBのバトル回数制限を取得すると、daily_battle_countが0になる", func(t *testing.T) {
							s, _ := revert(t)
							handlertest.AssertJSONField(t, s.GetBattleLimit(t, routertest.TokenB), "daily_battle_count", `0`)
						})
					})
				})
				t.Run("2回ずつで、停止で無効になった対戦の消費バトル回数の返却を、game_idがgame-1で2回続けるとき", func(t *testing.T) {
					revertTwice := func(t *testing.T) (*routertest.Server, *httptest.ResponseRecorder) {
						s := newFreeServer()
						consumeBattleCount(t, s, 2)
						body := buildBodyWith(t, revertBattleCountBase, map[string]any{"game_id": "game-1"})
						s.Call(t, http.MethodPost, revertBattleCountPath, routertest.NoAuth, body)
						return s, s.Call(t, http.MethodPost, revertBattleCountPath, routertest.NoAuth, body)
					}

					t.Run("2回目のステータスが204になる", func(t *testing.T) {
						_, rec := revertTwice(t)
						handlertest.AssertStatus(t, rec, http.StatusNoContent)
					})
					t.Run("続けてプレイヤーAのバトル回数制限を取得すると、daily_battle_countが1になる", func(t *testing.T) {
						s, _ := revertTwice(t)
						handlertest.AssertJSONField(t, s.GetBattleLimit(t, routertest.TokenA), "daily_battle_count", `1`)
					})
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("必須の文字列項目が指定されないとき", func(t *testing.T) {
				cases := []struct {
					name    string
					body    string
					message string
				}{
					{name: "game_idが空文字のとき、game_idの不足を示す400の応答になる", body: buildBodyWith(t, revertBattleCountBase, map[string]any{"game_id": ""}), message: "game_id is required"},
					{name: "game_idのキーが無いとき、game_idの不足を示す400の応答になる", body: buildBodyWithout(t, revertBattleCountBase, "game_id"), message: "game_id is required"},
					{name: "player1_idが空文字のとき、player1_idの不足を示す400の応答になる", body: buildBodyWith(t, revertBattleCountBase, map[string]any{"player1_id": ""}), message: "player1_id is required"},
					{name: "player1_idのキーが無いとき、player1_idの不足を示す400の応答になる", body: buildBodyWithout(t, revertBattleCountBase, "player1_id"), message: "player1_id is required"},
					{name: "player2_idが空文字のとき、player2_idの不足を示す400の応答になる", body: buildBodyWith(t, revertBattleCountBase, map[string]any{"player2_id": ""}), message: "player2_id is required"},
					{name: "player2_idのキーが無いとき、player2_idの不足を示す400の応答になる", body: buildBodyWithout(t, revertBattleCountBase, "player2_id"), message: "player2_id is required"},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						rec := newFreeServer().Call(t, http.MethodPost, revertBattleCountPath, routertest.NoAuth, tc.body)
						handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, tc.message)
					})
				}
			})
			t.Run("consumed_at_millisが正の整数でないとき", func(t *testing.T) {
				cases := []struct {
					name string
					body string
				}{
					{name: "0のとき、consumed_at_millisの不正を示す400の応答になる", body: buildBodyWith(t, revertBattleCountBase, map[string]any{"consumed_at_millis": 0})},
					{name: "キーが無いとき、consumed_at_millisの不正を示す400の応答になる", body: buildBodyWithout(t, revertBattleCountBase, "consumed_at_millis")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						rec := newFreeServer().Call(t, http.MethodPost, revertBattleCountPath, routertest.NoAuth, tc.body)
						handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "consumed_at_millis must be positive")
					})
				}
			})
			t.Run("player1_idまたはplayer2_idがnot-a-uuidのとき", func(t *testing.T) {
				cases := []struct {
					name    string
					body    string
					message string
				}{
					{name: "player1_idのとき、player1_idの不正を示す400の応答になる", body: buildBodyWith(t, revertBattleCountBase, map[string]any{"player1_id": "not-a-uuid"}), message: "player1_id is invalid"},
					{name: "player2_idのとき、player2_idの不正を示す400の応答になる", body: buildBodyWith(t, revertBattleCountBase, map[string]any{"player2_id": "not-a-uuid"}), message: "player2_id is invalid"},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						rec := newFreeServer().Call(t, http.MethodPost, revertBattleCountPath, routertest.NoAuth, tc.body)
						handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, tc.message)
					})
				}
			})
			t.Run("リクエストボディが無いとき、リクエストボディ不正を示す400の応答になる", func(t *testing.T) {
				rec := newFreeServer().Call(t, http.MethodPost, revertBattleCountPath, routertest.NoAuth, "")
				handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "invalid request body")
			})
		})
	})
}

func TestAddExp(t *testing.T) {
	t.Run("[経験値API]経験値の加算", func(t *testing.T) {
		addExpPath := "/api/v1/account/me/exp"
		newLevelOneServer := func() *routertest.Server {
			return routertest.NewSeededServer(map[string]int64{routertest.ConfigKeyExpCoefficient: 100}, routertest.NewPlayerSeedA())
		}
		t.Run("正常系", func(t *testing.T) {
			t.Run("設定した経験値係数が100で、経験値0のプレイヤーに経験値を加算するとき", func(t *testing.T) {
				addExp := func(t *testing.T, gain string) (*routertest.Server, *httptest.ResponseRecorder) {
					s := newLevelOneServer()
					return s, s.PostAs(t, routertest.TokenA, addExpPath, `{"exp_gain":`+gain+`}`)
				}

				t.Run("経験値150を加算すると、続けてプレイヤー情報を取得したときのexpが150になる", func(t *testing.T) {
					s, _ := addExp(t, "150")
					handlertest.AssertJSONField(t, s.GetPlayer(t, routertest.TokenA), "exp", `150`)
				})
				t.Run("経験値500を加算すると、続けてプレイヤー情報を取得したときのlevelが2になる", func(t *testing.T) {
					s, _ := addExp(t, "500")
					handlertest.AssertJSONField(t, s.GetPlayer(t, routertest.TokenA), "level", `2`)
				})
				t.Run("経験値0を加算すると、ステータスが204になる", func(t *testing.T) {
					_, rec := addExp(t, "0")
					handlertest.AssertStatus(t, rec, http.StatusNoContent)
				})
				t.Run("経験値0を加算すると、続けてプレイヤー情報を取得したときのexpが0のままになる", func(t *testing.T) {
					s, _ := addExp(t, "0")
					handlertest.AssertJSONField(t, s.GetPlayer(t, routertest.TokenA), "exp", `0`)
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("exp_gainのキーが無いとき、exp_gainの不足を示す400の応答になる", func(t *testing.T) {
				rec := newLevelOneServer().PostAs(t, routertest.TokenA, addExpPath, `{}`)
				handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "exp_gain is required")
			})
			t.Run("exp_gainが-1のとき、exp_gainの不正を示す400の応答になる", func(t *testing.T) {
				rec := newLevelOneServer().PostAs(t, routertest.TokenA, addExpPath, `{"exp_gain":-1}`)
				handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "exp_gain is invalid")
			})
			t.Run("リクエストボディが無いとき、リクエストボディ不正を示す400の応答になる", func(t *testing.T) {
				rec := newLevelOneServer().PostAs(t, routertest.TokenA, addExpPath, "")
				handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "invalid request body")
			})
		})
	})
}

func TestAwardGameExp(t *testing.T) {
	t.Run("[経験値API]ゲーム終了時の経験値付与", func(t *testing.T) {
		newPvpServer := func() *routertest.Server {
			return routertest.NewSeededServer(awardGameExpConfig(), routertest.NewPlayerSeedA(), routertest.NewPlayerSeedB())
		}
		award := func(t *testing.T, s *routertest.Server, body string) *httptest.ResponseRecorder {
			return s.Call(t, http.MethodPost, awardGameExpPath, routertest.NoAuth, body)
		}
		t.Run("正常系", func(t *testing.T) {
			t.Run("ゲーム終了時の経験値付与を、対戦種別がpvpで受け付けるとき", func(t *testing.T) {
				cases := []struct {
					name string
					body string
				}{
					{name: "winner_numが1、reasonがdraw以外のとき、ステータスが204になる", body: buildBodyWith(t, awardGameExpBase, nil)},
					{name: "winner_numが0のとき、ステータスが204になる", body: buildBodyWith(t, awardGameExpBase, map[string]any{"winner_num": 0})},
					{name: "winner_numが2、reasonがdraw以外のとき、ステータスが204になる", body: buildBodyWith(t, awardGameExpBase, map[string]any{"winner_num": 2})},
					{name: "winner_numが0、reasonがdisconnectのとき、ステータスが204になる", body: buildBodyWith(t, awardGameExpBase, map[string]any{"winner_num": 0, "reason": "disconnect"})},
					{name: "winner_numが0、reasonがdrawのとき、ステータスが204になる", body: buildBodyWith(t, awardGameExpBase, map[string]any{"winner_num": 0, "reason": "draw"})},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						handlertest.AssertStatus(t, award(t, newPvpServer(), tc.body), http.StatusNoContent)
					})
				}
			})
			t.Run("ゲーム終了時の経験値付与を、対戦種別がnpcでplayer2_idが空文字、winner_numが1、reasonがdraw以外で受け付けるとき", func(t *testing.T) {
				awardToNpc := func(t *testing.T) (*routertest.Server, *httptest.ResponseRecorder) {
					s := routertest.NewSeededServer(awardGameExpConfig(), routertest.NewPlayerSeedA())
					return s, award(t, s, buildBodyWith(t, awardGameExpBase, map[string]any{"match_type": "npc", "player2_id": ""}))
				}

				t.Run("ステータスが204になる", func(t *testing.T) {
					_, rec := awardToNpc(t)
					handlertest.AssertStatus(t, rec, http.StatusNoContent)
				})
				t.Run("勝利時の経験値を100に設定し、プレイヤーAの経験値が0のとき、続けてプレイヤーAのプレイヤー情報を取得すると、expが100になる", func(t *testing.T) {
					s, _ := awardToNpc(t)
					handlertest.AssertJSONField(t, s.GetPlayer(t, routertest.TokenA), "exp", `100`)
				})
			})
			t.Run("勝利時の経験値を100、敗北時の経験値を20に設定し、プレイヤーAとプレイヤーBの経験値が0で、対戦種別がpvp、winner_numが1、reasonがdraw以外のとき", func(t *testing.T) {
				run := func(t *testing.T) *routertest.Server {
					s := newPvpServer()
					award(t, s, buildBodyWith(t, awardGameExpBase, nil))
					return s
				}

				t.Run("続けてプレイヤーAのプレイヤー情報を取得すると、expが100になる", func(t *testing.T) {
					handlertest.AssertJSONField(t, run(t).GetPlayer(t, routertest.TokenA), "exp", `100`)
				})
				t.Run("続けてプレイヤーBのプレイヤー情報を取得すると、expが20になる", func(t *testing.T) {
					handlertest.AssertJSONField(t, run(t).GetPlayer(t, routertest.TokenB), "exp", `20`)
				})
			})
			t.Run("引き分け時の経験値を50に設定し、プレイヤーAとプレイヤーBの経験値が0で、対戦種別がpvp、winner_numが0、reasonがdisconnectのとき", func(t *testing.T) {
				run := func(t *testing.T) *routertest.Server {
					s := newPvpServer()
					award(t, s, buildBodyWith(t, awardGameExpBase, map[string]any{"winner_num": 0, "reason": "disconnect"}))
					return s
				}

				t.Run("続けてプレイヤーAのプレイヤー情報を取得すると、expが50になる", func(t *testing.T) {
					handlertest.AssertJSONField(t, run(t).GetPlayer(t, routertest.TokenA), "exp", `50`)
				})
				t.Run("続けてプレイヤーBのプレイヤー情報を取得すると、expが50になる", func(t *testing.T) {
					handlertest.AssertJSONField(t, run(t).GetPlayer(t, routertest.TokenB), "exp", `50`)
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("player1_idが指定されないとき", func(t *testing.T) {
				cases := []struct {
					name string
					body string
				}{
					{name: "空文字のとき、player1_idの不足を示す400の応答になる", body: buildBodyWith(t, awardGameExpBase, map[string]any{"player1_id": ""})},
					{name: "キーが無いとき、player1_idの不足を示す400の応答になる", body: buildBodyWithout(t, awardGameExpBase, "player1_id")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						handlertest.AssertErrorResponse(t, award(t, newPvpServer(), tc.body), http.StatusBadRequest, "player1_id is required")
					})
				}
			})
			t.Run("player1_idがnot-a-uuidのとき、player1_idの不正を示す400の応答になる", func(t *testing.T) {
				rec := award(t, newPvpServer(), buildBodyWith(t, awardGameExpBase, map[string]any{"player1_id": "not-a-uuid"}))
				handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "player1_id is invalid")
			})
			t.Run("対戦種別がpvpでplayer2_idが指定されないとき", func(t *testing.T) {
				cases := []struct {
					name string
					body string
				}{
					{name: "空文字のとき、player2_idの不足を示す400の応答になる", body: buildBodyWith(t, awardGameExpBase, map[string]any{"player2_id": ""})},
					{name: "キーが無いとき、player2_idの不足を示す400の応答になる", body: buildBodyWithout(t, awardGameExpBase, "player2_id")},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						handlertest.AssertErrorResponse(t, award(t, newPvpServer(), tc.body), http.StatusBadRequest, "player2_id is required")
					})
				}
			})
			t.Run("player2_idがnot-a-uuidのとき", func(t *testing.T) {
				cases := []struct {
					name string
					body string
				}{
					{name: "対戦種別がpvpのとき、player2_idの不正を示す400の応答になる", body: buildBodyWith(t, awardGameExpBase, map[string]any{"player2_id": "not-a-uuid"})},
					{name: "対戦種別がnpcのとき、player2_idの不正を示す400の応答になる", body: buildBodyWith(t, awardGameExpBase, map[string]any{"player2_id": "not-a-uuid", "match_type": "npc"})},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						handlertest.AssertErrorResponse(t, award(t, newPvpServer(), tc.body), http.StatusBadRequest, "player2_id is invalid")
					})
				}
			})
			t.Run("契約で必須の項目が指定されないとき", func(t *testing.T) {
				cases := []struct {
					name    string
					body    string
					message string
				}{
					{name: "winner_numのキーが無いとき、winner_numの不足を示す400の応答になる", body: buildBodyWithout(t, awardGameExpBase, "winner_num"), message: "winner_num is required"},
					{name: "reasonのキーが無いとき、reasonの不足を示す400の応答になる", body: buildBodyWithout(t, awardGameExpBase, "reason"), message: "reason is required"},
					{name: "reasonが空文字のとき、reasonの不足を示す400の応答になる", body: buildBodyWith(t, awardGameExpBase, map[string]any{"reason": ""}), message: "reason is required"},
					{name: "match_typeのキーが無いとき、match_typeの不足を示す400の応答になる", body: buildBodyWithout(t, awardGameExpBase, "match_type"), message: "match_type is required"},
					{name: "match_typeが空文字のとき、match_typeの不足を示す400の応答になる", body: buildBodyWith(t, awardGameExpBase, map[string]any{"match_type": ""}), message: "match_type is required"},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						handlertest.AssertErrorResponse(t, award(t, newPvpServer(), tc.body), http.StatusBadRequest, tc.message)
					})
				}
			})
			t.Run("winner_numの指定が許可されないとき", func(t *testing.T) {
				cases := []struct {
					name string
					body string
				}{
					{name: "3のとき、winner_numの不正を示す400の応答になる", body: buildBodyWith(t, awardGameExpBase, map[string]any{"winner_num": 3})},
					{name: "-1のとき、winner_numの不正を示す400の応答になる", body: buildBodyWith(t, awardGameExpBase, map[string]any{"winner_num": -1})},
					{name: "reasonがdrawで1のとき、winner_numの不正を示す400の応答になる", body: buildBodyWith(t, awardGameExpBase, map[string]any{"reason": "draw", "winner_num": 1})},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						handlertest.AssertErrorResponse(t, award(t, newPvpServer(), tc.body), http.StatusBadRequest, "winner_num is invalid")
					})
				}
			})
			t.Run("match_typeがpvpでもnpcでもないarenaのとき、match_typeの不正を示す400の応答になる", func(t *testing.T) {
				rec := award(t, newPvpServer(), buildBodyWith(t, awardGameExpBase, map[string]any{"match_type": "arena"}))
				handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "match_type is invalid")
			})
			t.Run("プレイヤーAは登録済みでプレイヤーBは存在せず、対戦種別がpvp、winner_numが1、reasonがdraw以外のとき", func(t *testing.T) {
				run := func(t *testing.T) (*routertest.Server, *httptest.ResponseRecorder) {
					s := routertest.NewSeededServer(awardGameExpConfig(), routertest.NewPlayerSeedA())
					return s, award(t, s, buildBodyWith(t, awardGameExpBase, nil))
				}

				t.Run("プレイヤー不在を示す404の応答になる", func(t *testing.T) {
					_, rec := run(t)
					handlertest.AssertErrorResponse(t, rec, http.StatusNotFound, "player not found")
				})
				t.Run("勝利時の経験値を100に設定し、プレイヤーAの経験値が0のとき、続けてプレイヤーAのプレイヤー情報を取得すると、expが0のままになる", func(t *testing.T) {
					s, _ := run(t)
					handlertest.AssertJSONField(t, s.GetPlayer(t, routertest.TokenA), "exp", `0`)
				})
			})
			t.Run("リクエストボディが無いとき、リクエストボディ不正を示す400の応答になる", func(t *testing.T) {
				handlertest.AssertErrorResponse(t, award(t, newPvpServer(), ""), http.StatusBadRequest, "invalid request body")
			})
		})
	})
}

func TestUpdatePremium(t *testing.T) {
	t.Run("[プレミアムAPI]プレミアム状態の更新", func(t *testing.T) {
		premiumPath := "/api/v1/account/me/premium"
		config := map[string]int64{routertest.ConfigKeyExpCoefficient: 100}
		expiry := time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)
		t.Run("正常系", func(t *testing.T) {
			t.Run("is_premiumをtrueにして、expires_at_millisを1にして更新すると、ステータスが204になる", func(t *testing.T) {
				s := routertest.NewSeededServer(config, routertest.NewPlayerSeedA())
				handlertest.AssertStatus(t, s.PutAs(t, routertest.TokenA, premiumPath, `{"is_premium":true,"expires_at_millis":1}`), http.StatusNoContent)
			})
			t.Run("is_premiumをtrueにして更新したあと、続けてプレイヤー情報を取得すると、is_premiumがtrueになる", func(t *testing.T) {
				s := routertest.NewSeededServer(config, routertest.NewPlayerSeedA())
				s.PutAs(t, routertest.TokenA, premiumPath, `{"is_premium":true}`)
				handlertest.AssertJSONField(t, s.GetPlayer(t, routertest.TokenA), "is_premium", `true`)
			})
			t.Run("is_premiumをtrue、expires_at_millisを1800000000000にして更新したあと、続けてプレイヤー情報を取得すると、premium_expires_atが2027-01-15T08:00:00Zの時点を表す", func(t *testing.T) {
				s := routertest.NewSeededServer(config, routertest.NewPlayerSeedA())
				s.PutAs(t, routertest.TokenA, premiumPath, `{"is_premium":true,"expires_at_millis":1800000000000}`)
				assertTimeField(t, s.GetPlayer(t, routertest.TokenA), "premium_expires_at", time.Date(2027, 1, 15, 8, 0, 0, 0, time.UTC))
			})
			t.Run("有効期限が設定済みのプレミアム中のプレイヤーが、expires_at_millisを省略して更新したあと、続けてプレイヤー情報を取得すると、premium_expires_atが含まれない", func(t *testing.T) {
				seed := routertest.NewPlayerSeedA()
				seed.IsPremium = true
				seed.PremiumExpiresAt = &expiry
				s := routertest.NewSeededServer(config, seed)
				s.PutAs(t, routertest.TokenA, premiumPath, `{"is_premium":true}`)
				assertFieldAbsent(t, s.GetPlayer(t, routertest.TokenA), "premium_expires_at")
			})
			t.Run("プレミアム中のプレイヤーが、is_premiumをfalseにして更新したあと、続けてプレイヤー情報を取得すると、is_premiumがfalseになる", func(t *testing.T) {
				seed := routertest.NewPlayerSeedA()
				seed.IsPremium = true
				s := routertest.NewSeededServer(config, seed)
				s.PutAs(t, routertest.TokenA, premiumPath, `{"is_premium":false}`)
				handlertest.AssertJSONField(t, s.GetPlayer(t, routertest.TokenA), "is_premium", `false`)
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("is_premiumのキーが無いとき、is_premiumの不足を示す400の応答になる", func(t *testing.T) {
				s := routertest.NewSeededServer(config, routertest.NewPlayerSeedA())
				handlertest.AssertErrorResponse(t, s.PutAs(t, routertest.TokenA, premiumPath, `{}`), http.StatusBadRequest, "is_premium is required")
			})
			t.Run("expires_at_millisが0のとき、expires_at_millisの不正を示す400の応答になる", func(t *testing.T) {
				s := routertest.NewSeededServer(config, routertest.NewPlayerSeedA())
				rec := s.PutAs(t, routertest.TokenA, premiumPath, `{"is_premium":true,"expires_at_millis":0}`)
				handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "expires_at_millis is invalid")
			})
			t.Run("リクエストボディが無いとき、リクエストボディ不正を示す400の応答になる", func(t *testing.T) {
				s := routertest.NewSeededServer(config, routertest.NewPlayerSeedA())
				handlertest.AssertErrorResponse(t, s.PutAs(t, routertest.TokenA, premiumPath, ""), http.StatusBadRequest, "invalid request body")
			})
		})
	})
}
