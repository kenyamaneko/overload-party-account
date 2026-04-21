package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// ptr はテスト内でポインタリテラルを書きやすくするヘルパ。
func ptr[T any](v T) *T { return &v }

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
			name:     "言語=ja / プッシュ有効のシード値をそのまま返す",
			seedLang: "ja",
			seedBgm:  50,
			seedSe:   60,
			seedPush: true,
		},
		{
			name:     "言語=en / プッシュ無効のシード値をそのまま返す",
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

// Insert は Register 用プリミティブ。全フィールドを受け取りそのまま書き込む。
func TestUserSettingsRepository_Insert(t *testing.T) {
	repo := postgres.NewUserSettingsRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	payload := &apiaccount.UserSettings{
		PlayerID:    testPlayerID1,
		Language:    "ja",
		BgmVolume:   30,
		SeVolume:    40,
		PushEnabled: true,
	}
	require.NoError(t, repo.Insert(ctx, payload))

	got, err := repo.Get(ctx, testPlayerID1)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "ja", got.Language)
	assert.Equal(t, int64(30), got.BgmVolume)
	assert.Equal(t, int64(40), got.SeVolume)
	assert.True(t, got.PushEnabled)
}

// UpdatePartial の仕様:
//   - 非 nil フィールドだけが書き換わり、nil フィールドは COALESCE で現状維持
//   - 複数フィールド同時指定、単一フィールドのみ指定、どちらも動作する
func TestUserSettingsRepository_UpdatePartial(t *testing.T) {
	repo := postgres.NewUserSettingsRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name     string
		patch    *port.UserSettingsPatch
		wantLang string
		wantBgm  int64
		wantSe   int64
		wantPush bool
	}{
		{
			name: "言語だけ更新すると他のフィールドは現状維持される",
			patch: &port.UserSettingsPatch{
				Language: ptr("en"),
			},
			wantLang: "en",
			wantBgm:  50, // seed 値
			wantSe:   60, // seed 値
			wantPush: true,
		},
		{
			name: "BGM 音量だけ更新しても言語はシード値のまま",
			patch: &port.UserSettingsPatch{
				BgmVolume: ptr(int64(80)),
			},
			wantLang: "ja", // seed 値
			wantBgm:  80,
			wantSe:   60,
			wantPush: true,
		},
		{
			name: "複数フィールド同時指定でまとめて更新される",
			patch: &port.UserSettingsPatch{
				Language:    ptr("en"),
				BgmVolume:   ptr(int64(0)),
				SeVolume:    ptr(int64(0)),
				PushEnabled: ptr(false),
			},
			wantLang: "en",
			wantBgm:  0,
			wantSe:   0,
			wantPush: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			seedUserSettings(t, testPlayerID1, "ja", 50, 60, true)

			require.NoError(t, repo.UpdatePartial(ctx, testPlayerID1, tt.patch))

			got, err := repo.Get(ctx, testPlayerID1)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantLang, got.Language)
			assert.Equal(t, tt.wantBgm, got.BgmVolume)
			assert.Equal(t, tt.wantSe, got.SeVolume)
			assert.Equal(t, tt.wantPush, got.PushEnabled)
		})
	}
}

func TestUserSettingsRepository_UpdatePartial_NotFound(t *testing.T) {
	repo := postgres.NewUserSettingsRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	err := repo.UpdatePartial(ctx, testPlayerID1, &port.UserSettingsPatch{Language: ptr("en")})
	assert.ErrorIs(t, err, port.ErrNotFound)
}
