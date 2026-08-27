//go:build integration

package rest_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// createPlayerWithoutSettings は players / player_progression のみを作成し、
// player_settings 行を意図的に作らない。正規の登録フロー (POST /internal/v1/auth/register) では
// 発生し得ない状態のため、リポジトリを直接使って前提を組み立てる。
func createPlayerWithoutSettings(t *testing.T) string {
	t.Helper()
	playerID := uuid.NewString()
	now := time.Now().UTC()
	playerRepo := postgres.NewPlayerRepository(sharedPg.Pool)
	err := playerRepo.Create(context.Background(), &domain.Player{
		PlayerID:         playerID,
		FirebaseUID:      "firebase-no-settings-" + playerID,
		IsPremium:        false,
		OnboardingStatus: domain.OnboardingStatusNotStarted,
		CreatedAt:        now,
		UpdatedAt:        now,
	}, &domain.PlayerProgression{
		PlayerID:  playerID,
		Level:     1,
		Exp:       0,
		UpdatedAt: now,
	})
	require.NoError(t, err)
	return playerID
}

func TestPlayerSettingsHandler_GetSettings(t *testing.T) {
	t.Run("[プレイヤー設定API]GET /api/v1/account/me/settings", func(t *testing.T) {
		t.Run("プレイヤー設定を200で返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }

			w := doJSON(r, "GET", "/api/v1/account/me/settings", nil, authHeader())

			require.Equal(t, 200, w.Code)
			var resp apiaccount.PlayerSettingsResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, "ja", resp.Language)
		})

		t.Run("対象プレイヤーの設定行が存在しないとき、404を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			playerID := createPlayerWithoutSettings(t)
			verifier.VerifyFn = func(token string) (string, error) { return playerID, nil }

			w := doJSON(r, "GET", "/api/v1/account/me/settings", nil, authHeader())

			assert.Equal(t, 404, w.Code)
		})
	})
}

func TestPlayerSettingsHandler_UpdateSettings(t *testing.T) {
	t.Run("[プレイヤー設定API]PUT /api/v1/account/me/settings", func(t *testing.T) {
		t.Run("更新対象フィールドが1つも指定されていないとき、400を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }

			w := doJSON(r, "PUT", "/api/v1/account/me/settings", apiaccount.UpdateSettingsRequest{}, authHeader())

			assert.Equal(t, 400, w.Code)
		})

		t.Run("1つ以上のフィールドが指定されているとき、指定されたフィールドのみを更新し、200と更新後の設定情報を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			player := registerPlayer(t, r)
			verifier.VerifyFn = func(token string) (string, error) { return player.PlayerID, nil }
			lang := "en"

			w := doJSON(r, "PUT", "/api/v1/account/me/settings", apiaccount.UpdateSettingsRequest{Language: &lang}, authHeader())

			require.Equal(t, 200, w.Code)
			var resp apiaccount.PlayerSettingsResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, "en", resp.Language)
			assert.Equal(t, int64(50), resp.BgmVolume)
		})

		t.Run("対象プレイヤーが存在しないとき、404を返す", func(t *testing.T) {
			r, verifier := newTestRouter(t, validGameConfigValues())
			verifier.VerifyFn = func(token string) (string, error) { return uuid.NewString(), nil }
			lang := "en"

			w := doJSON(r, "PUT", "/api/v1/account/me/settings", apiaccount.UpdateSettingsRequest{Language: &lang}, authHeader())

			assert.Equal(t, 404, w.Code)
		})
	})
}
