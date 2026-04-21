package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/model"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// newUserSettingsTestService は実 PostgreSQL repository で UserSettingsService を組む。
func newUserSettingsTestService() *UserSettingsService {
	_, _, userSettingsRepo, _ := newRealRepos()
	return NewUserSettingsService(userSettingsRepo)
}

func TestUserSettingsService_Get_Seeded(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		seedLang string
		seedBgm  int64
		seedSe   int64
		seedPush bool
	}{
		{
			name:     "en / push=false",
			seedLang: "en",
			seedBgm:  20,
			seedSe:   30,
			seedPush: false,
		},
		{
			name:     "ja / push=true",
			seedLang: "ja",
			seedBgm:  50,
			seedSe:   60,
			seedPush: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			seedUserSettings(t, testPlayerID1, tt.seedLang, tt.seedBgm, tt.seedSe, tt.seedPush)

			svc := newUserSettingsTestService()
			got, err := svc.Get(ctx, testPlayerID1)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, testPlayerID1, got.PlayerID)
			assert.Equal(t, tt.seedLang, got.Language)
			assert.Equal(t, tt.seedBgm, got.BgmVolume)
			assert.Equal(t, tt.seedSe, got.SeVolume)
			assert.Equal(t, tt.seedPush, got.PushEnabled)
		})
	}
}

// TestUserSettingsService_Get_Unseeded_ReturnsDefaultsWithoutPersisting は
// 未登録プレイヤーに対して Get がデフォルト値を返しつつ DB に書き込まない契約を検証する
// （handler は明示的な Update でのみ永続化する）。
func TestUserSettingsService_Get_Unseeded_ReturnsDefaultsWithoutPersisting(t *testing.T) {
	ctx := context.Background()
	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	svc := newUserSettingsTestService()
	got, err := svc.Get(ctx, testPlayerID1)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, testPlayerID1, got.PlayerID)
	assert.Equal(t, model.DefaultLanguage, got.Language)
	assert.Equal(t, model.DefaultBgmVolume, got.BgmVolume)
	assert.Equal(t, model.DefaultSeVolume, got.SeVolume)
	assert.Equal(t, model.DefaultPushEnabled, got.PushEnabled)

	var count int
	require.NoError(t, sharedPg.Pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM account.user_settings WHERE player_id = $1`, testPlayerID1,
	).Scan(&count))
	assert.Equal(t, 0, count, "Get はデフォルト返却で永続化しない")
}

func TestUserSettingsService_Update(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		preSeed  bool
		payload  *apiaccount.UserSettings
		wantLang string
		wantBgm  int64
	}{
		{
			name:    "新規 INSERT (未登録プレイヤー)",
			preSeed: false,
			payload: &apiaccount.UserSettings{
				PlayerID:    testPlayerID1,
				Language:    "ja",
				BgmVolume:   40,
				SeVolume:    60,
				PushEnabled: true,
			},
			wantLang: "ja",
			wantBgm:  40,
		},
		{
			name:    "既存は UPDATE で上書き",
			preSeed: true,
			payload: &apiaccount.UserSettings{
				PlayerID:    testPlayerID1,
				Language:    "en",
				BgmVolume:   90,
				SeVolume:    80,
				PushEnabled: false,
			},
			wantLang: "en",
			wantBgm:  90,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			if tt.preSeed {
				seedUserSettings(t, testPlayerID1, "ja", 50, 50, true)
			}

			svc := newUserSettingsTestService()
			require.NoError(t, svc.Update(ctx, tt.payload))

			got, err := svc.Get(ctx, testPlayerID1)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantLang, got.Language)
			assert.Equal(t, tt.wantBgm, got.BgmVolume)
		})
	}
}

// Update は upsert で updated_at を現在時刻に書き換える。
// 既存行の updated_at が必ず前進することを検証する（監査やキャッシュ無効化の基盤）。
func TestUserSettingsService_Update_AdvancesUpdatedAt(t *testing.T) {
	ctx := context.Background()
	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
	seedUserSettings(t, testPlayerID1, "ja", 50, 50, true)

	svc := newUserSettingsTestService()
	before, err := svc.Get(ctx, testPlayerID1)
	require.NoError(t, err)

	// updated_at 解像度が 1ms 未満の場合に同値になることを防ぐため 2ms 待つ。
	time.Sleep(2 * time.Millisecond)

	require.NoError(t, svc.Update(ctx, &apiaccount.UserSettings{
		PlayerID:    testPlayerID1,
		Language:    "en",
		BgmVolume:   60,
		SeVolume:    70,
		PushEnabled: false,
	}))

	after, err := svc.Get(ctx, testPlayerID1)
	require.NoError(t, err)
	assert.True(t, after.UpdatedAt.After(before.UpdatedAt),
		"after.UpdatedAt (%v) should be strictly after before.UpdatedAt (%v)",
		after.UpdatedAt, before.UpdatedAt)
}

// Get は該当プレイヤーの設定が無い場合、デフォルト値の UpdatedAt に現在時刻を詰めて返す。
// 「常に non-nil で language/volume/push がデフォルト値」であることの契約テスト。
func TestUserSettingsService_Get_DefaultHasCurrentTimestamp(t *testing.T) {
	ctx := context.Background()
	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	svc := newUserSettingsTestService()
	before := time.Now()
	got, err := svc.Get(ctx, testPlayerID1)
	after := time.Now()
	require.NoError(t, err)
	require.NotNil(t, got)

	// デフォルト UpdatedAt は [before, after] の範囲内であるべき。
	assert.False(t, got.UpdatedAt.Before(before.Add(-time.Second)),
		"default UpdatedAt %v should be ~now (before=%v)", got.UpdatedAt, before)
	assert.False(t, got.UpdatedAt.After(after.Add(time.Second)),
		"default UpdatedAt %v should be ~now (after=%v)", got.UpdatedAt, after)
}
