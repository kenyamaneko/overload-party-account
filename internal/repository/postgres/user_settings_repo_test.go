package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

func TestUserSettingsRepository_Get(t *testing.T) {
	repo := postgres.NewUserSettingsRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name     string
		seed     bool
		wantNil  bool
		wantLang string
	}{
		{"シード済みなら取得成功", true, false, "ja"},
		{"未登録は (nil, nil)", false, true, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			if tt.seed {
				seedUserSettings(t, testPlayerID1, "ja", 50, 60, true)
			}

			got, err := repo.Get(ctx, testPlayerID1)
			require.NoError(t, err)
			if tt.wantNil {
				assert.Nil(t, got)
				return
			}
			require.NotNil(t, got)
			assert.Equal(t, tt.wantLang, got.Language)
		})
	}
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
			name:     "新規 INSERT",
			preSeed:  false,
			payload:  &apiaccount.UserSettings{PlayerID: testPlayerID1, Language: "ja", BgmVolume: 30, SeVolume: 40, PushEnabled: true},
			wantLang: "ja",
			wantBgm:  30,
		},
		{
			name:     "既存は UPDATE で上書き",
			preSeed:  true,
			payload:  &apiaccount.UserSettings{PlayerID: testPlayerID1, Language: "en", BgmVolume: 80, SeVolume: 90, PushEnabled: false},
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
