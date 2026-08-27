//go:build integration

package rest_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gamedesign "github.com/kenyamaneko/overload-party-common/packages/game-design-constants"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

func TestPlayerHandler_AwardGameExp(t *testing.T) {
	t.Run("POST /internal/v1/players/award-game-exp", func(t *testing.T) {
		t.Run("リクエストボディが不正なJSONのとき、400を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())

			w := doRaw(r, "POST", "/internal/v1/players/award-game-exp", []byte("not-json"), nil)

			assert.Equal(t, 400, w.Code)
		})

		t.Run("成功したとき、204を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())
			p1 := registerPlayer(t, r)
			p2 := registerPlayer(t, r)

			w := doJSON(r, "POST", "/internal/v1/players/award-game-exp", apiaccount.AwardGameExpRequest{
				Player1ID: p1.PlayerID, Player2ID: p2.PlayerID, WinnerNum: 1, Reason: "normal", MatchType: gamedesign.MatchTypePvp,
			}, nil)

			assert.Equal(t, 204, w.Code)
		})

		t.Run("player1_idが空文字のとき、400を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())
			p2 := registerPlayer(t, r)

			w := doJSON(r, "POST", "/internal/v1/players/award-game-exp", apiaccount.AwardGameExpRequest{
				Player1ID: "", Player2ID: p2.PlayerID, WinnerNum: 1, Reason: "normal", MatchType: gamedesign.MatchTypePvp,
			}, nil)

			assert.Equal(t, 400, w.Code)
		})

		t.Run("match_typeがnpcでないとき、player2_idが空文字だと400を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())
			p1 := registerPlayer(t, r)

			w := doJSON(r, "POST", "/internal/v1/players/award-game-exp", apiaccount.AwardGameExpRequest{
				Player1ID: p1.PlayerID, Player2ID: "", WinnerNum: 1, Reason: "normal", MatchType: gamedesign.MatchTypePvp,
			}, nil)

			assert.Equal(t, 400, w.Code)
		})

		t.Run("match_typeがnpcのとき、player2_idが空文字であっても204を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())
			p1 := registerPlayer(t, r)

			w := doJSON(r, "POST", "/internal/v1/players/award-game-exp", apiaccount.AwardGameExpRequest{
				Player1ID: p1.PlayerID, Player2ID: "", WinnerNum: 1, Reason: "normal", MatchType: gamedesign.MatchTypeNpc,
			}, nil)

			assert.Equal(t, 204, w.Code)
		})
	})
}

func TestPlayerHandler_RevertBattleCount(t *testing.T) {
	t.Run("POST /internal/v1/players/revert-battle-count", func(t *testing.T) {
		t.Run("game_idが空のとき、400を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())
			p1 := registerPlayer(t, r)
			p2 := registerPlayer(t, r)

			w := doJSON(r, "POST", "/internal/v1/players/revert-battle-count", apiaccount.RevertBattleCountRequest{
				GameID: "", Player1ID: p1.PlayerID, Player2ID: p2.PlayerID, ConsumedAtMillis: time.Now().UnixMilli(),
			}, nil)

			assert.Equal(t, 400, w.Code)
		})

		t.Run("player1_idが空のとき、400を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())
			p2 := registerPlayer(t, r)

			w := doJSON(r, "POST", "/internal/v1/players/revert-battle-count", apiaccount.RevertBattleCountRequest{
				GameID: "game-1", Player1ID: "", Player2ID: p2.PlayerID, ConsumedAtMillis: time.Now().UnixMilli(),
			}, nil)

			assert.Equal(t, 400, w.Code)
		})

		t.Run("player2_idが空のとき、400を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())
			p1 := registerPlayer(t, r)

			w := doJSON(r, "POST", "/internal/v1/players/revert-battle-count", apiaccount.RevertBattleCountRequest{
				GameID: "game-1", Player1ID: p1.PlayerID, Player2ID: "", ConsumedAtMillis: time.Now().UnixMilli(),
			}, nil)

			assert.Equal(t, 400, w.Code)
		})

		t.Run("consumed_at_millisが0以下のとき、400を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())
			p1 := registerPlayer(t, r)
			p2 := registerPlayer(t, r)

			w := doJSON(r, "POST", "/internal/v1/players/revert-battle-count", apiaccount.RevertBattleCountRequest{
				GameID: "game-1", Player1ID: p1.PlayerID, Player2ID: p2.PlayerID, ConsumedAtMillis: 0,
			}, nil)

			assert.Equal(t, 400, w.Code)
		})

		t.Run("リクエストが正しいとき、204を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())
			p1 := registerPlayer(t, r)
			p2 := registerPlayer(t, r)

			w := doJSON(r, "POST", "/internal/v1/players/revert-battle-count", apiaccount.RevertBattleCountRequest{
				GameID: "game-1", Player1ID: p1.PlayerID, Player2ID: p2.PlayerID, ConsumedAtMillis: time.Now().UnixMilli(),
			}, nil)

			assert.Equal(t, 204, w.Code)
		})
	})
}

func TestPlayerHandler_GetPlayer(t *testing.T) {
	t.Run("GET /api/v1/account/me", func(t *testing.T) {
		t.Run("認証済みのプレイヤーの情報を200で返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }

			w := doJSON(r, "GET", "/api/v1/account/me", nil, authHeader())

			require.Equal(t, 200, w.Code)
			var resp apiaccount.PlayerResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, player.PlayerID, resp.PlayerID)
		})

		t.Run("認証で解決したplayer_idに対応するプレイヤーが存在しないとき、404を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			verifier.VerifyFn = func(token string) (string, error) { return uuid.NewString(), nil }

			w := doJSON(r, "GET", "/api/v1/account/me", nil, authHeader())

			assert.Equal(t, 404, w.Code)
		})
	})
}

func TestPlayerHandler_UpdateName(t *testing.T) {
	t.Run("PUT /api/v1/account/me/name", func(t *testing.T) {
		t.Run("表示名が空文字のとき、対象プレイヤーが存在しても400を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }

			w := doJSON(r, "PUT", "/api/v1/account/me/name", apiaccount.UpdateNameRequest{Name: ""}, authHeader())

			assert.Equal(t, 400, w.Code)
		})

		t.Run("表示名が空文字のとき、対象プレイヤーが存在しなくても400を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			verifier.VerifyFn = func(token string) (string, error) { return uuid.NewString(), nil }

			w := doJSON(r, "PUT", "/api/v1/account/me/name", apiaccount.UpdateNameRequest{Name: ""}, authHeader())

			assert.Equal(t, 400, w.Code)
		})

		t.Run("表示名が有効で、対象プレイヤーが存在するとき、更新し200と更新後のプレイヤー情報を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }

			w := doJSON(r, "PUT", "/api/v1/account/me/name", apiaccount.UpdateNameRequest{Name: "新しい名前"}, authHeader())

			require.Equal(t, 200, w.Code)
			var resp apiaccount.PlayerResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			require.NotNil(t, resp.Name)
			assert.Equal(t, "新しい名前", *resp.Name)
		})

		t.Run("表示名が有効で、対象プレイヤーが存在しないとき、404を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			verifier.VerifyFn = func(token string) (string, error) { return uuid.NewString(), nil }

			w := doJSON(r, "PUT", "/api/v1/account/me/name", apiaccount.UpdateNameRequest{Name: "新しい名前"}, authHeader())

			assert.Equal(t, 404, w.Code)
		})
	})
}

func TestPlayerHandler_ValidateNameForOnboarding(t *testing.T) {
	t.Run("POST /api/v1/account/me/onboarding/name/validate", func(t *testing.T) {
		t.Run("対象プレイヤーが存在しないとき、表示名の内容に関わらず404を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			verifier.VerifyFn = func(token string) (string, error) { return uuid.NewString(), nil }

			w := doJSON(r, "POST", "/api/v1/account/me/onboarding/name/validate", apiaccount.ValidateNameForOnboardingRequest{Name: "有効な名前"}, authHeader())

			assert.Equal(t, 404, w.Code)
		})

		t.Run("対象プレイヤーが存在し、表示名が空文字のとき、400を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }

			w := doJSON(r, "POST", "/api/v1/account/me/onboarding/name/validate", apiaccount.ValidateNameForOnboardingRequest{Name: ""}, authHeader())

			assert.Equal(t, 400, w.Code)
		})

		t.Run("対象プレイヤーが存在し、呼び出し前に保存されている表示名とは異なる有効な表示名を指定したとき、204を返し、対象プレイヤーの表示名は呼び出し前の値のまま変わらない", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }
			updateW := doJSON(r, "PUT", "/api/v1/account/me/name", apiaccount.UpdateNameRequest{Name: "呼び出し前の名前"}, authHeader())
			require.Equal(t, 200, updateW.Code)

			w := doJSON(r, "POST", "/api/v1/account/me/onboarding/name/validate", apiaccount.ValidateNameForOnboardingRequest{Name: "別の有効な名前"}, authHeader())

			require.Equal(t, 204, w.Code)
			getW := doJSON(r, "GET", "/api/v1/account/me", nil, authHeader())
			require.Equal(t, 200, getW.Code)
			var resp apiaccount.PlayerResponse
			require.NoError(t, json.Unmarshal(getW.Body.Bytes(), &resp))
			require.NotNil(t, resp.Name)
			assert.Equal(t, "呼び出し前の名前", *resp.Name)
		})
	})
}

func TestPlayerHandler_GetBattleLimit(t *testing.T) {
	t.Run("GET /api/v1/account/me/battle-limit", func(t *testing.T) {
		t.Run("バトル制限情報を200で返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }

			w := doJSON(r, "GET", "/api/v1/account/me/battle-limit", nil, authHeader())

			require.Equal(t, 200, w.Code)
			var resp apiaccount.BattleLimitResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, int64(5), resp.DailyBattleLimit)
		})

		t.Run("対象プレイヤーが存在しないとき、404を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			verifier.VerifyFn = func(token string) (string, error) { return uuid.NewString(), nil }

			w := doJSON(r, "GET", "/api/v1/account/me/battle-limit", nil, authHeader())

			assert.Equal(t, 404, w.Code)
		})
	})
}

func TestPlayerHandler_IncrementBattleCount(t *testing.T) {
	t.Run("POST /api/v1/account/me/battle-limit/increment", func(t *testing.T) {
		t.Run("成功したとき、204を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }

			w := doJSON(r, "POST", "/api/v1/account/me/battle-limit/increment", nil, authHeader())

			assert.Equal(t, 204, w.Code)
		})

		t.Run("対戦回数が上限に達しているとき、429を返す", func(t *testing.T) {
			values := validGameConfigValues()
			values["free_daily_battle_limit"] = 1
			r, verifier := newTestRouter(t, values)
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }
			first := doJSON(r, "POST", "/api/v1/account/me/battle-limit/increment", nil, authHeader())
			require.Equal(t, 204, first.Code)

			w := doJSON(r, "POST", "/api/v1/account/me/battle-limit/increment", nil, authHeader())

			assert.Equal(t, 429, w.Code)
		})

		t.Run("対象プレイヤーが存在しないとき、404を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			verifier.VerifyFn = func(token string) (string, error) { return uuid.NewString(), nil }

			w := doJSON(r, "POST", "/api/v1/account/me/battle-limit/increment", nil, authHeader())

			assert.Equal(t, 404, w.Code)
		})
	})
}

func TestPlayerHandler_UpdatePremium(t *testing.T) {
	t.Run("PUT /api/v1/account/me/premium", func(t *testing.T) {
		t.Run("リクエストボディが不正なJSONのとき、400を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }

			w := doRaw(r, "PUT", "/api/v1/account/me/premium", []byte("not-json"), authHeader())

			assert.Equal(t, 400, w.Code)
		})

		t.Run("成功したとき、204を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }

			w := doJSON(r, "PUT", "/api/v1/account/me/premium", apiaccount.UpdatePremiumRequest{IsPremium: true}, authHeader())

			assert.Equal(t, 204, w.Code)
		})

		t.Run("対象プレイヤーが存在しないとき、404を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			verifier.VerifyFn = func(token string) (string, error) { return uuid.NewString(), nil }

			w := doJSON(r, "PUT", "/api/v1/account/me/premium", apiaccount.UpdatePremiumRequest{IsPremium: true}, authHeader())

			assert.Equal(t, 404, w.Code)
		})
	})
}

func TestPlayerHandler_AddExp(t *testing.T) {
	t.Run("POST /api/v1/account/me/exp", func(t *testing.T) {
		t.Run("リクエストボディが不正なJSONのとき、400を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }

			w := doRaw(r, "POST", "/api/v1/account/me/exp", []byte("not-json"), authHeader())

			assert.Equal(t, 400, w.Code)
		})

		t.Run("成功したとき、204を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }

			w := doJSON(r, "POST", "/api/v1/account/me/exp", apiaccount.AddExpRequest{ExpGain: 10}, authHeader())

			assert.Equal(t, 204, w.Code)
		})

		t.Run("対象プレイヤーが存在しないとき、404を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			verifier.VerifyFn = func(token string) (string, error) { return uuid.NewString(), nil }

			w := doJSON(r, "POST", "/api/v1/account/me/exp", apiaccount.AddExpRequest{ExpGain: 10}, authHeader())

			assert.Equal(t, 404, w.Code)
		})
	})
}
