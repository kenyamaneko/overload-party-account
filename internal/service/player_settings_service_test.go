//go:build integration

package service

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// ptr はテスト内でポインタリテラルを書きやすくするヘルパ。
func ptr[T any](v T) *T { return &v }

// newPlayerSettingsTestService は実 PostgreSQL repository で PlayerSettingsService を組む。
func newPlayerSettingsTestService() *PlayerSettingsService {
	_, _, playerSettingsRepo, _ := newRealRepos()
	return NewPlayerSettingsService(playerSettingsRepo)
}

func TestPlayerSettingsService_Get_Seeded(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		seedLang string
		seedBgm  int64
		seedSe   int64
		seedPush bool
	}{
		{
			name:     "言語=en / プッシュ無効のシード値をそのまま返す",
			seedLang: "en",
			seedBgm:  20,
			seedSe:   30,
			seedPush: false,
		},
		{
			name:     "言語=ja / プッシュ有効のシード値をそのまま返す",
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
			seedPlayerSettings(t, testPlayerID1, tt.seedLang, tt.seedBgm, tt.seedSe, tt.seedPush)

			svc := newPlayerSettingsTestService()
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

// player_settings 行が存在しない場合 Get は port.ErrNotFound を返し、
// デフォルト値での隠蔽は行わない（Register と同一 Tx で必ず INSERT されている契約のため、
// 行が無いのは Register 未実施または不整合の症状であり、エラーとして扱う）。
func TestPlayerSettingsService_Get_Unseeded_ReturnsErrNotFound(t *testing.T) {
	ctx := context.Background()
	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	svc := newPlayerSettingsTestService()
	_, err := svc.Get(ctx, testPlayerID1)
	require.ErrorIs(t, err, port.ErrNotFound)
}

// Update は patch の非 nil フィールドだけを更新する（部分更新契約）。
// nil フィールドは現状維持、複数指定は同時に書き換え。
func TestPlayerSettingsService_Update(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name     string
		patch    *port.PlayerSettingsPatch
		wantLang string
		wantBgm  int64
		wantSe   int64
		wantPush bool
	}{
		{
			name:     "言語だけ指定すると他の項目はシード値のまま維持される",
			patch:    &port.PlayerSettingsPatch{Language: ptr("en")},
			wantLang: "en",
			wantBgm:  50,
			wantSe:   50,
			wantPush: true,
		},
		{
			name: "全フィールド指定で一括上書きされる",
			patch: &port.PlayerSettingsPatch{
				Language:    ptr("en"),
				BgmVolume:   ptr(int64(90)),
				SeVolume:    ptr(int64(80)),
				PushEnabled: ptr(false),
			},
			wantLang: "en",
			wantBgm:  90,
			wantSe:   80,
			wantPush: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			seedPlayerSettings(t, testPlayerID1, "ja", 50, 50, true)

			svc := newPlayerSettingsTestService()
			require.NoError(t, svc.Update(ctx, testPlayerID1, tt.patch))

			got, err := svc.Get(ctx, testPlayerID1)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, tt.wantLang, got.Language)
			assert.Equal(t, tt.wantBgm, got.BgmVolume)
			assert.Equal(t, tt.wantSe, got.SeVolume)
			assert.Equal(t, tt.wantPush, got.PushEnabled)
		})
	}
}

// player_settings 行が未登録なら Update は ErrNotFound を返す。
// 通常は Register で Insert されているので発生しないが、契約として担保する。
func TestPlayerSettingsService_Update_NotFound(t *testing.T) {
	ctx := context.Background()
	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	svc := newPlayerSettingsTestService()
	err := svc.Update(ctx, testPlayerID1, &port.PlayerSettingsPatch{Language: ptr("en")})
	require.ErrorIs(t, err, port.ErrNotFound)
}

// 既存行の updated_at が Update で必ず前進することを検証する（監査やキャッシュ無効化の基盤）。
// 実際の更新は BEFORE UPDATE トリガー trg_player_settings_updated_at が now() に書き換える。
func TestPlayerSettingsService_Update_AdvancesUpdatedAt(t *testing.T) {
	ctx := context.Background()
	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
	seedPlayerSettings(t, testPlayerID1, "ja", 50, 50, true)

	svc := newPlayerSettingsTestService()
	before, err := svc.Get(ctx, testPlayerID1)
	require.NoError(t, err)

	// updated_at 解像度が 1ms 未満の場合に同値になることを防ぐため 2ms 待つ。
	time.Sleep(2 * time.Millisecond)

	require.NoError(t, svc.Update(ctx, testPlayerID1, &port.PlayerSettingsPatch{Language: ptr("en")}))

	after, err := svc.Get(ctx, testPlayerID1)
	require.NoError(t, err)
	assert.True(t, after.UpdatedAt.After(before.UpdatedAt),
		"after.UpdatedAt (%v) should be strictly after before.UpdatedAt (%v)",
		after.UpdatedAt, before.UpdatedAt)
}

