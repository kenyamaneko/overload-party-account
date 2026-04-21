package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

func TestUserSettingsRepository_Get_Seeded(t *testing.T) {
	repo := postgres.NewUserSettingsRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name     string
		seedLang string
		seedBgm  int64
		seedSe   int64
		seedPush bool
	}{
		{
			name:     "ja / push=true",
			seedLang: "ja",
			seedBgm:  50,
			seedSe:   60,
			seedPush: true,
		},
		{
			name:     "en / push=false",
			seedLang: "en",
			seedBgm:  10,
			seedSe:   20,
			seedPush: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			seedUserSettings(t, testPlayerID1, tt.seedLang, tt.seedBgm, tt.seedSe, tt.seedPush)

			got, err := repo.Get(ctx, testPlayerID1)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.seedLang, got.Language)
			assert.Equal(t, tt.seedBgm, got.BgmVolume)
			assert.Equal(t, tt.seedSe, got.SeVolume)
			assert.Equal(t, tt.seedPush, got.PushEnabled)
		})
	}
}

func TestUserSettingsRepository_Get_Unseeded_ReturnsNil(t *testing.T) {
	repo := postgres.NewUserSettingsRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	got, err := repo.Get(ctx, testPlayerID1)
	require.NoError(t, err)
	assert.Nil(t, got)
}

func TestUserSettingsRepository_Upsert(t *testing.T) {
	repo := postgres.NewUserSettingsRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name     string
		preSeed  bool
		payload  *apiaccount.UserSettings
		wantLang string
		wantBgm  int64
	}{
		{
			name:    "新規 INSERT",
			preSeed: false,
			payload: &apiaccount.UserSettings{
				PlayerID:    testPlayerID1,
				Language:    "ja",
				BgmVolume:   30,
				SeVolume:    40,
				PushEnabled: true,
			},
			wantLang: "ja",
			wantBgm:  30,
		},
		{
			name:    "既存は UPDATE で上書き",
			preSeed: true,
			payload: &apiaccount.UserSettings{
				PlayerID:    testPlayerID1,
				Language:    "en",
				BgmVolume:   80,
				SeVolume:    90,
				PushEnabled: false,
			},
			wantLang: "en",
			wantBgm:  80,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			if tt.preSeed {
				seedUserSettings(t, testPlayerID1, "ja", 50, 60, true)
			}

			require.NoError(t, repo.Upsert(ctx, tt.payload))

			got, err := repo.Get(ctx, testPlayerID1)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantLang, got.Language)
			assert.Equal(t, tt.wantBgm, got.BgmVolume)
		})
	}
}
