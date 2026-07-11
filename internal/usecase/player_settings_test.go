//go:build integration

package usecase

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

// newPlayerSettingsTestInteractor は実 PostgreSQL repository で PlayerSettingsInteractor を組む。
func newPlayerSettingsTestInteractor() *PlayerSettingsInteractor {
	_, _, _, playerSettingsRepo, _ := newRealRepos()
	return NewPlayerSettingsInteractor(playerSettingsRepo)
}

func TestGet(t *testing.T) {
	ctx := context.Background()

	t.Run("プレイヤー設定の取得", func(t *testing.T) {
		seededCases := []struct {
			name     string
			seedLang string
			seedBgm  int64
			seedSe   int64
			seedPush bool
		}{
			{
				name:     "言語=en / プッシュ無効のシード値のとき、そのまま返す",
				seedLang: "en",
				seedBgm:  20,
				seedSe:   30,
				seedPush: false,
			},
			{
				name:     "言語=ja / プッシュ有効のシード値のとき、そのまま返す",
				seedLang: "ja",
				seedBgm:  50,
				seedSe:   60,
				seedPush: true,
			},
		}
		for _, tc := range seededCases {
			t.Run(tc.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
				seedPlayerSettings(t, testPlayerID1, tc.seedLang, tc.seedBgm, tc.seedSe, tc.seedPush)

				svc := newPlayerSettingsTestInteractor()
				got, err := svc.Get(ctx, testPlayerID1)
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, testPlayerID1, got.PlayerID)
				assert.Equal(t, tc.seedLang, got.Language)
				assert.Equal(t, tc.seedBgm, got.BgmVolume)
				assert.Equal(t, tc.seedSe, got.SeVolume)
				assert.Equal(t, tc.seedPush, got.PushEnabled)
			})
		}

		t.Run("player_settings 行が無いとき、port.ErrNotFound になる", func(t *testing.T) {
			// Register と同一 Tx で必ず INSERT される契約なので、行が無いのは未実施または
			// 不整合の症状であり、デフォルト値で隠さずエラーにする。
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			svc := newPlayerSettingsTestInteractor()
			_, err := svc.Get(ctx, testPlayerID1)
			require.ErrorIs(t, err, port.ErrNotFound)
		})
	})
}

func TestUpdate(t *testing.T) {
	ctx := context.Background()

	t.Run("プレイヤー設定の更新", func(t *testing.T) {
		// 部分更新契約: patch の非 nil フィールドだけを更新し、nil フィールドは現状維持する。
		patchCases := []struct {
			name     string
			patch    *port.PlayerSettingsPatch
			wantLang string
			wantBgm  int64
			wantSe   int64
			wantPush bool
		}{
			{
				name:     "Language だけ指定するとき、他項目はシード値のまま維持される",
				patch:    &port.PlayerSettingsPatch{Language: ptr("en")},
				wantLang: "en",
				wantBgm:  50,
				wantSe:   50,
				wantPush: true,
			},
			{
				name: "全フィールド指定のとき、一括上書きされる",
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
		for _, tc := range patchCases {
			t.Run(tc.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
				seedPlayerSettings(t, testPlayerID1, "ja", 50, 50, true)

				svc := newPlayerSettingsTestInteractor()
				require.NoError(t, svc.Update(ctx, testPlayerID1, tc.patch))

				got, err := svc.Get(ctx, testPlayerID1)
				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, tc.wantLang, got.Language)
				assert.Equal(t, tc.wantBgm, got.BgmVolume)
				assert.Equal(t, tc.wantSe, got.SeVolume)
				assert.Equal(t, tc.wantPush, got.PushEnabled)
			})
		}

		t.Run("player_settings 行が未登録のとき、port.ErrNotFound になる", func(t *testing.T) {
			// 通常は Register で Insert されるため発生しないが、契約として担保する。
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			svc := newPlayerSettingsTestInteractor()
			err := svc.Update(ctx, testPlayerID1, &port.PlayerSettingsPatch{Language: ptr("en")})
			require.ErrorIs(t, err, port.ErrNotFound)
		})

		t.Run("設定を更新するとき、updated_at が前進する", func(t *testing.T) {
			// updated_at の前進は監査やキャッシュ無効化の基盤。実際の書き換えは
			// BEFORE UPDATE トリガー trg_player_settings_updated_at が now() で行う。
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			seedPlayerSettings(t, testPlayerID1, "ja", 50, 50, true)

			svc := newPlayerSettingsTestInteractor()
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
		})
	})
}
