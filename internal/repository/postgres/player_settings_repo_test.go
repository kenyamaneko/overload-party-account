//go:build integration

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

// Insert は Register 用プリミティブ。全フィールドを受け取りそのまま書き込む。
func TestPlayerSettingsRepository_Insert(t *testing.T) {
	repo := postgres.NewPlayerSettingsRepository(sharedPg.Pool)
	ctx := context.Background()

	sharedPg.Truncate(t)
	seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

	payload := &apiaccount.PlayerSettings{
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

// Get の契約: シード済みなら永続層の値を全フィールドそのまま返し、
// 未シードなら ErrNotFound (FindByFirebaseUID と揃え、未存在は常にエラーで表現)。
func TestPlayerSettingsRepository_Get(t *testing.T) {
	repo := postgres.NewPlayerSettingsRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name    string
		seeds   []apiaccount.PlayerSettings // 0 件 = 未シード, 1 件 = その値をシード
		want    *settingsSnapshot
		wantErr error
	}{
		{
			name: "シード済みなら永続層の値をそのまま返す",
			seeds: []apiaccount.PlayerSettings{
				{Language: "ja", BgmVolume: 50, SeVolume: 60, PushEnabled: true},
			},
			want: &settingsSnapshot{Language: "ja", BgmVolume: 50, SeVolume: 60, PushEnabled: true},
		},
		{
			name:    "未シードなら ErrNotFound",
			seeds:   nil,
			want:    nil,
			wantErr: port.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			for _, s := range tt.seeds {
				seedPlayerSettings(t, testPlayerID1, s.Language, s.BgmVolume, s.SeVolume, s.PushEnabled)
			}

			got, err := repo.Get(ctx, testPlayerID1)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, snapshotSettings(got))
		})
	}
}

// UpdatePartial の契約:
//   - 非 nil フィールドだけが書き換わり、nil フィールドは COALESCE で現状維持
//   - 複数フィールド同時指定、単一フィールドのみ指定、どちらも動作する
//   - 行が無い (Register 未実施) なら ErrNotFound
//
// シード値は (ja, 50, 60, true)。want は patch 適用後の期待スナップショット。
// 未シードケースは want=nil で「行が存在しない (= UpdatePartial が ErrNotFound を返す)」を
// 表現する。
func TestPlayerSettingsRepository_UpdatePartial(t *testing.T) {
	repo := postgres.NewPlayerSettingsRepository(sharedPg.Pool)
	ctx := context.Background()

	tests := []struct {
		name    string
		seeds   []apiaccount.PlayerSettings // 0 件 = 未シード
		patch   *port.PlayerSettingsPatch
		want    *settingsSnapshot
		wantErr error
	}{
		{
			name: "言語だけ更新すると他のフィールドは現状維持される",
			seeds: []apiaccount.PlayerSettings{
				{Language: "ja", BgmVolume: 50, SeVolume: 60, PushEnabled: true},
			},
			patch: &port.PlayerSettingsPatch{Language: ptr("en")},
			want:  &settingsSnapshot{Language: "en", BgmVolume: 50, SeVolume: 60, PushEnabled: true},
		},
		{
			name: "BGM 音量だけ更新しても言語はシード値のまま",
			seeds: []apiaccount.PlayerSettings{
				{Language: "ja", BgmVolume: 50, SeVolume: 60, PushEnabled: true},
			},
			patch: &port.PlayerSettingsPatch{BgmVolume: ptr(int64(80))},
			want:  &settingsSnapshot{Language: "ja", BgmVolume: 80, SeVolume: 60, PushEnabled: true},
		},
		{
			name: "複数フィールド同時指定でまとめて更新される",
			seeds: []apiaccount.PlayerSettings{
				{Language: "ja", BgmVolume: 50, SeVolume: 60, PushEnabled: true},
			},
			patch: &port.PlayerSettingsPatch{
				Language:    ptr("en"),
				BgmVolume:   ptr(int64(0)),
				SeVolume:    ptr(int64(0)),
				PushEnabled: ptr(false),
			},
			want: &settingsSnapshot{Language: "en", BgmVolume: 0, SeVolume: 0, PushEnabled: false},
		},
		{
			name:    "未シードなら ErrNotFound (行が無いまま)",
			seeds:   nil,
			patch:   &port.PlayerSettingsPatch{Language: ptr("en")},
			want:    nil,
			wantErr: port.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			for _, s := range tt.seeds {
				seedPlayerSettings(t, testPlayerID1, s.Language, s.BgmVolume, s.SeVolume, s.PushEnabled)
			}

			err := repo.UpdatePartial(ctx, testPlayerID1, tt.patch)
			require.ErrorIs(t, err, tt.wantErr)

			// Get は「シード済みケースは行を返す」「未シードケースは ErrNotFound を返す」と
			// シード状況に連動するため、UpdatePartial と同じ wantErr で判定する。
			got, err := repo.Get(ctx, testPlayerID1)
			require.ErrorIs(t, err, tt.wantErr)
			assert.Equal(t, tt.want, snapshotSettings(got))
		})
	}
}
