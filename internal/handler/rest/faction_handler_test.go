//go:build integration

package rest_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	gamedesign "github.com/kenyamaneko/overload-party-common/packages/game-design-constants"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

func TestFactionHandler_ListPlayerFactions(t *testing.T) {
	t.Run("GET /internal/v1/players/{playerID}/factions", func(t *testing.T) {
		t.Run("指定したplayerIDの所持ファクション一覧を200で返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }
			grantW := doJSON(r, "POST", "/api/v1/account/me/factions", apiaccount.FactionGrantRequest{Faction: gamedesign.FactionSHE}, authHeader())
			require.Equal(t, 204, grantW.Code)

			w := doJSON(r, "GET", "/internal/v1/players/"+player.PlayerID+"/factions", nil, nil)

			require.Equal(t, 200, w.Code)
			var listing apiaccount.FactionListing
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listing))
			assert.Equal(t, []string{gamedesign.FactionSHE}, listing.Factions)
		})

		t.Run("対象プレイヤーが存在しないplayerIDを指定しても、エラーにはならず空の一覧を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())

			w := doJSON(r, "GET", "/internal/v1/players/"+uuid.NewString()+"/factions", nil, nil)

			assert.Equal(t, 200, w.Code)
		})

		t.Run("所持ファクションが0件のとき、レスポンスJSONのfactionsはnullになる", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)

			w := doJSON(r, "GET", "/internal/v1/players/"+player.PlayerID+"/factions", nil, nil)

			require.Equal(t, 200, w.Code)
			assert.Contains(t, w.Body.String(), `"factions":null`)
		})
	})
}

func TestFactionHandler_ListFactions(t *testing.T) {
	t.Run("GET /api/v1/account/me/factions", func(t *testing.T) {
		t.Run("所持ファクション一覧を200で返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }
			grantW := doJSON(r, "POST", "/api/v1/account/me/factions", apiaccount.FactionGrantRequest{Faction: gamedesign.FactionTenki}, authHeader())
			require.Equal(t, 204, grantW.Code)

			w := doJSON(r, "GET", "/api/v1/account/me/factions", nil, authHeader())

			require.Equal(t, 200, w.Code)
			var listing apiaccount.FactionListing
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &listing))
			assert.Equal(t, []string{gamedesign.FactionTenki}, listing.Factions)
		})

		t.Run("対象プレイヤーが存在しなくても、エラーにはならず空の一覧を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			verifier.VerifyFn = func(token string) (string, error) { return uuid.NewString(), nil }

			w := doJSON(r, "GET", "/api/v1/account/me/factions", nil, authHeader())

			assert.Equal(t, 200, w.Code)
		})

		t.Run("所持ファクションが0件のとき、レスポンスJSONのfactionsはnullになる", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }

			w := doJSON(r, "GET", "/api/v1/account/me/factions", nil, authHeader())

			require.Equal(t, 200, w.Code)
			assert.Contains(t, w.Body.String(), `"factions":null`)
		})
	})
}

func TestFactionHandler_GrantFaction(t *testing.T) {
	t.Run("POST /api/v1/account/me/factions", func(t *testing.T) {
		t.Run("factionが空のとき、400を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }

			w := doJSON(r, "POST", "/api/v1/account/me/factions", apiaccount.FactionGrantRequest{Faction: ""}, authHeader())

			assert.Equal(t, 400, w.Code)
		})

		t.Run("存在するplayer_idに対して、成功したとき204を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }

			w := doJSON(r, "POST", "/api/v1/account/me/factions", apiaccount.FactionGrantRequest{Faction: gamedesign.FactionSugar}, authHeader())

			assert.Equal(t, 204, w.Code)
		})

		t.Run("存在しないplayer_idに対しては、外部キー制約違反により500を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			verifier.VerifyFn = func(token string) (string, error) { return uuid.NewString(), nil }

			w := doJSON(r, "POST", "/api/v1/account/me/factions", apiaccount.FactionGrantRequest{Faction: gamedesign.FactionSugar}, authHeader())

			assert.Equal(t, 500, w.Code)
		})
	})
}

func TestFactionHandler_SelectInitialFaction(t *testing.T) {
	t.Run("POST /api/v1/account/me/factions/select", func(t *testing.T) {
		t.Run("faction_idが空のとき、400を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }

			w := doJSON(r, "POST", "/api/v1/account/me/factions/select", apiaccount.SelectInitialFactionRequest{FactionID: ""}, authHeader())

			assert.Equal(t, 400, w.Code)
		})

		t.Run("指定したファクションが選択可能なファクションでないとき、400を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }

			w := doJSON(r, "POST", "/api/v1/account/me/factions/select", apiaccount.SelectInitialFactionRequest{FactionID: gamedesign.FactionNeutral}, authHeader())

			assert.Equal(t, 400, w.Code)
		})

		t.Run("対象プレイヤーが存在しないとき、404を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			verifier.VerifyFn = func(token string) (string, error) { return uuid.NewString(), nil }

			w := doJSON(r, "POST", "/api/v1/account/me/factions/select", apiaccount.SelectInitialFactionRequest{FactionID: gamedesign.FactionSHE}, authHeader())

			assert.Equal(t, 404, w.Code)
		})

		t.Run("初期ファクション未選択のとき、選択を確定し200を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }

			w := doJSON(r, "POST", "/api/v1/account/me/factions/select", apiaccount.SelectInitialFactionRequest{FactionID: gamedesign.FactionSHE}, authHeader())

			assert.Equal(t, 200, w.Code)
		})

		t.Run("初期ファクション選択済みのとき、409を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }
			w1 := doJSON(r, "POST", "/api/v1/account/me/factions/select", apiaccount.SelectInitialFactionRequest{FactionID: gamedesign.FactionSHE}, authHeader())
			require.Equal(t, 200, w1.Code)

			w2 := doJSON(r, "POST", "/api/v1/account/me/factions/select", apiaccount.SelectInitialFactionRequest{FactionID: gamedesign.FactionTenki}, authHeader())

			assert.Equal(t, 409, w2.Code)
		})
	})
}
