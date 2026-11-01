//go:build integration

package postgres_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres/postgrestest"
)

func registerSettings(t *testing.T, r *postgrestest.Repositories, settings *domain.PlayerSettings) {
	t.Helper()
	require.NoError(t, r.Settings.InsertPlayerSettings(t.Context(), settings))
}

func getSettings(t *testing.T, r *postgrestest.Repositories, playerID string) *domain.PlayerSettings {
	t.Helper()
	settings, err := r.Settings.GetPlayerSettings(t.Context(), playerID)
	require.NoError(t, err)
	return settings
}

func TestPlayerSettingsRegistration(t *testing.T) {
	t.Run("[プレイヤー設定]プレイヤー設定の登録", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("登録後にプレイヤーIDで取得すると", func(t *testing.T) {
				registerAndGet := func(t *testing.T, settings *domain.PlayerSettings) *domain.PlayerSettings {
					t.Helper()
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
					registerSettings(t, r, settings)

					return getSettings(t, r, postgrestest.PlayerID1)
				}
				languageTests := []struct {
					name     string
					language string
				}{
					{name: "言語「ja」で登録したとき、取得結果の言語が「ja」になる", language: "ja"},
					{name: "言語「en」で登録したとき、取得結果の言語が「en」になる", language: "en"},
				}
				for _, tt := range languageTests {
					t.Run(tt.name, func(t *testing.T) {
						got := registerAndGet(t, postgrestest.NewSettingsWithValues(postgrestest.PlayerID1, tt.language, 50, 50, true))

						assert.Equal(t, tt.language, got.Language)
					})
				}
				bgmVolumeTests := []struct {
					name      string
					bgmVolume int64
				}{
					{name: "BGM音量50で登録したとき、取得結果のBGM音量が50になる", bgmVolume: 50},
					{name: "BGM音量30で登録したとき、取得結果のBGM音量が30になる", bgmVolume: 30},
				}
				for _, tt := range bgmVolumeTests {
					t.Run(tt.name, func(t *testing.T) {
						got := registerAndGet(t, postgrestest.NewSettingsWithValues(postgrestest.PlayerID1, "ja", tt.bgmVolume, 50, true))

						assert.Equal(t, tt.bgmVolume, got.BgmVolume)
					})
				}
				seVolumeTests := []struct {
					name     string
					seVolume int64
				}{
					{name: "SE音量50で登録したとき、取得結果のSE音量が50になる", seVolume: 50},
					{name: "SE音量70で登録したとき、取得結果のSE音量が70になる", seVolume: 70},
				}
				for _, tt := range seVolumeTests {
					t.Run(tt.name, func(t *testing.T) {
						got := registerAndGet(t, postgrestest.NewSettingsWithValues(postgrestest.PlayerID1, "ja", 50, tt.seVolume, true))

						assert.Equal(t, tt.seVolume, got.SeVolume)
					})
				}
				isPushEnabledTests := []struct {
					name          string
					isPushEnabled bool
				}{
					{name: "通知許可有効で登録したとき、取得結果の通知許可が有効になる", isPushEnabled: true},
					{name: "通知許可無効で登録したとき、取得結果の通知許可が無効になる", isPushEnabled: false},
				}
				for _, tt := range isPushEnabledTests {
					t.Run(tt.name, func(t *testing.T) {
						got := registerAndGet(t, postgrestest.NewSettingsWithValues(postgrestest.PlayerID1, "ja", 50, 50, tt.isPushEnabled))

						assert.Equal(t, tt.isPushEnabled, got.IsPushEnabled)
					})
				}
				t.Run("更新日時が2026-01-02T00:00:00Zのとき、取得結果の更新日時が2026-01-02T00:00:00Zになる", func(t *testing.T) {
					settings := postgrestest.NewSettings(postgrestest.PlayerID1)
					settings.UpdatedAt = postgrestest.NewUTCTime(2026, time.January, 2, 0, 0, 0)

					got := registerAndGet(t, settings)

					assert.Equal(t, postgrestest.NewUTCTime(2026, time.January, 2, 0, 0, 0), got.UpdatedAt.UTC())
				})
			})
		})
	})
}

func TestPlayerSettingsLookup(t *testing.T) {
	t.Run("[プレイヤー設定]プレイヤー設定の取得", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("言語が「ja」「en」「fr」の3件のプレイヤーの設定が登録されているとき、言語が「en」の設定のプレイヤーIDで取得すると、取得結果の言語が「en」になる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2))
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID3, postgrestest.FirebaseUID3))
				registerSettings(t, r, postgrestest.NewSettingsWithValues(postgrestest.PlayerID1, "ja", 10, 20, true))
				registerSettings(t, r, postgrestest.NewSettingsWithValues(postgrestest.PlayerID2, "en", 30, 40, false))
				registerSettings(t, r, postgrestest.NewSettingsWithValues(postgrestest.PlayerID3, "fr", 60, 70, true))

				got := getSettings(t, r, postgrestest.PlayerID2)

				assert.Equal(t, "en", got.Language)
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("取得すると", func(t *testing.T) {
				tests := []struct {
					name    string
					prepare func(t *testing.T, r *postgrestest.Repositories)
				}{
					{
						name:    "設定が1件も登録されていないとき、見つからないエラーになる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {},
					},
					{
						name: "別のプレイヤーの設定だけが登録されているとき、見つからないエラーになる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2))
							registerSettings(t, r, postgrestest.NewSettings(postgrestest.PlayerID2))
						},
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						tt.prepare(t, r)

						_, err := r.Settings.GetPlayerSettings(t.Context(), postgrestest.PlayerID1)

						assert.ErrorIs(t, err, port.ErrNotFound)
					})
				}
			})
		})
	})
}

func TestPlayerSettingsPartialUpdate(t *testing.T) {
	t.Run("[プレイヤー設定]プレイヤー設定の部分更新", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("初期値が言語「ja」、BGM音量50、SE音量50、通知許可有効の設定を更新してから取得すると", func(t *testing.T) {
				updateInitialSettings := func(t *testing.T, patch *port.PlayerSettingsPatch) *domain.PlayerSettings {
					t.Helper()
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
					registerSettings(t, r, postgrestest.NewSettingsWithValues(postgrestest.PlayerID1, "ja", 50, 50, true))
					require.NoError(t, r.Settings.UpdatePlayerSettings(t.Context(), postgrestest.PlayerID1, patch))
					return getSettings(t, r, postgrestest.PlayerID1)
				}
				languageEnOnly := &port.PlayerSettingsPatch{Language: postgrestest.NewPointer("en")}
				languageTenCharactersOnly := &port.PlayerSettingsPatch{Language: postgrestest.NewPointer("abcdefghij")}
				bgmVolume80Only := &port.PlayerSettingsPatch{BgmVolume: postgrestest.NewPointer(int64(80))}
				bgmVolume0Only := &port.PlayerSettingsPatch{BgmVolume: postgrestest.NewPointer(int64(0))}
				seVolume20Only := &port.PlayerSettingsPatch{SeVolume: postgrestest.NewPointer(int64(20))}
				seVolume0Only := &port.PlayerSettingsPatch{SeVolume: postgrestest.NewPointer(int64(0))}
				pushDisabledOnly := &port.PlayerSettingsPatch{IsPushEnabled: postgrestest.NewPointer(false)}
				allItems := &port.PlayerSettingsPatch{
					Language:      postgrestest.NewPointer("en"),
					BgmVolume:     postgrestest.NewPointer(int64(80)),
					SeVolume:      postgrestest.NewPointer(int64(20)),
					IsPushEnabled: postgrestest.NewPointer(false),
				}

				languageEnTests := []struct {
					name  string
					patch *port.PlayerSettingsPatch
				}{
					{name: "更新する項目が言語「en」だけのとき、言語が「en」に変わる", patch: languageEnOnly},
					{name: "更新する項目が言語・BGM音量・SE音量・通知許可のすべてで、言語が「en」のとき、言語が「en」に変わる", patch: allItems},
				}
				for _, tt := range languageEnTests {
					t.Run(tt.name, func(t *testing.T) {
						got := updateInitialSettings(t, tt.patch)

						assert.Equal(t, "en", got.Language)
					})
				}
				t.Run("更新する項目が10文字の言語「abcdefghij」だけのとき、言語が「abcdefghij」に変わる", func(t *testing.T) {
					got := updateInitialSettings(t, languageTenCharactersOnly)

					assert.Equal(t, "abcdefghij", got.Language)
				})
				bgmVolume80Tests := []struct {
					name  string
					patch *port.PlayerSettingsPatch
				}{
					{name: "更新する項目がBGM音量80だけのとき、BGM音量が80に変わる", patch: bgmVolume80Only},
					{name: "更新する項目が言語・BGM音量・SE音量・通知許可のすべてで、BGM音量が80のとき、BGM音量が80に変わる", patch: allItems},
				}
				for _, tt := range bgmVolume80Tests {
					t.Run(tt.name, func(t *testing.T) {
						got := updateInitialSettings(t, tt.patch)

						assert.Equal(t, int64(80), got.BgmVolume)
					})
				}
				t.Run("更新する項目がBGM音量0だけのとき、BGM音量が0に変わる", func(t *testing.T) {
					got := updateInitialSettings(t, bgmVolume0Only)

					assert.Equal(t, int64(0), got.BgmVolume)
				})
				seVolume20Tests := []struct {
					name  string
					patch *port.PlayerSettingsPatch
				}{
					{name: "更新する項目がSE音量20だけのとき、SE音量が20に変わる", patch: seVolume20Only},
					{name: "更新する項目が言語・BGM音量・SE音量・通知許可のすべてで、SE音量が20のとき、SE音量が20に変わる", patch: allItems},
				}
				for _, tt := range seVolume20Tests {
					t.Run(tt.name, func(t *testing.T) {
						got := updateInitialSettings(t, tt.patch)

						assert.Equal(t, int64(20), got.SeVolume)
					})
				}
				t.Run("更新する項目がSE音量0だけのとき、SE音量が0に変わる", func(t *testing.T) {
					got := updateInitialSettings(t, seVolume0Only)

					assert.Equal(t, int64(0), got.SeVolume)
				})
				pushDisabledTests := []struct {
					name  string
					patch *port.PlayerSettingsPatch
				}{
					{name: "更新する項目が通知許可無効だけのとき、通知許可が無効に変わる", patch: pushDisabledOnly},
					{name: "更新する項目が言語・BGM音量・SE音量・通知許可のすべてで、通知許可が無効のとき、通知許可が無効に変わる", patch: allItems},
				}
				for _, tt := range pushDisabledTests {
					t.Run(tt.name, func(t *testing.T) {
						got := updateInitialSettings(t, tt.patch)

						assert.False(t, got.IsPushEnabled)
					})
				}
				languageUnchangedTests := []struct {
					name  string
					patch *port.PlayerSettingsPatch
				}{
					{name: "更新する項目がBGM音量80だけのとき、言語が「ja」のままになる", patch: bgmVolume80Only},
					{name: "更新する項目がBGM音量0だけのとき、言語が「ja」のままになる", patch: bgmVolume0Only},
					{name: "更新する項目がSE音量20だけのとき、言語が「ja」のままになる", patch: seVolume20Only},
					{name: "更新する項目がSE音量0だけのとき、言語が「ja」のままになる", patch: seVolume0Only},
					{name: "更新する項目が通知許可無効だけのとき、言語が「ja」のままになる", patch: pushDisabledOnly},
				}
				for _, tt := range languageUnchangedTests {
					t.Run(tt.name, func(t *testing.T) {
						got := updateInitialSettings(t, tt.patch)

						assert.Equal(t, "ja", got.Language)
					})
				}
				bgmVolumeUnchangedTests := []struct {
					name  string
					patch *port.PlayerSettingsPatch
				}{
					{name: "更新する項目が言語「en」だけのとき、BGM音量が50のままになる", patch: languageEnOnly},
					{name: "更新する項目が10文字の言語「abcdefghij」だけのとき、BGM音量が50のままになる", patch: languageTenCharactersOnly},
					{name: "更新する項目がSE音量20だけのとき、BGM音量が50のままになる", patch: seVolume20Only},
					{name: "更新する項目がSE音量0だけのとき、BGM音量が50のままになる", patch: seVolume0Only},
					{name: "更新する項目が通知許可無効だけのとき、BGM音量が50のままになる", patch: pushDisabledOnly},
				}
				for _, tt := range bgmVolumeUnchangedTests {
					t.Run(tt.name, func(t *testing.T) {
						got := updateInitialSettings(t, tt.patch)

						assert.Equal(t, int64(50), got.BgmVolume)
					})
				}
				seVolumeUnchangedTests := []struct {
					name  string
					patch *port.PlayerSettingsPatch
				}{
					{name: "更新する項目が言語「en」だけのとき、SE音量が50のままになる", patch: languageEnOnly},
					{name: "更新する項目が10文字の言語「abcdefghij」だけのとき、SE音量が50のままになる", patch: languageTenCharactersOnly},
					{name: "更新する項目がBGM音量80だけのとき、SE音量が50のままになる", patch: bgmVolume80Only},
					{name: "更新する項目がBGM音量0だけのとき、SE音量が50のままになる", patch: bgmVolume0Only},
					{name: "更新する項目が通知許可無効だけのとき、SE音量が50のままになる", patch: pushDisabledOnly},
				}
				for _, tt := range seVolumeUnchangedTests {
					t.Run(tt.name, func(t *testing.T) {
						got := updateInitialSettings(t, tt.patch)

						assert.Equal(t, int64(50), got.SeVolume)
					})
				}
				isPushEnabledUnchangedTests := []struct {
					name  string
					patch *port.PlayerSettingsPatch
				}{
					{name: "更新する項目が言語「en」だけのとき、通知許可が有効のままになる", patch: languageEnOnly},
					{name: "更新する項目が10文字の言語「abcdefghij」だけのとき、通知許可が有効のままになる", patch: languageTenCharactersOnly},
					{name: "更新する項目がBGM音量80だけのとき、通知許可が有効のままになる", patch: bgmVolume80Only},
					{name: "更新する項目がBGM音量0だけのとき、通知許可が有効のままになる", patch: bgmVolume0Only},
					{name: "更新する項目がSE音量20だけのとき、通知許可が有効のままになる", patch: seVolume20Only},
					{name: "更新する項目がSE音量0だけのとき、通知許可が有効のままになる", patch: seVolume0Only},
				}
				for _, tt := range isPushEnabledUnchangedTests {
					t.Run(tt.name, func(t *testing.T) {
						got := updateInitialSettings(t, tt.patch)

						assert.True(t, got.IsPushEnabled)
					})
				}
			})
			t.Run("更新日時が2026-01-01T00:00:00Zの設定の言語を更新すると、取得結果の更新日時が2026-01-01T00:00:00Zより後になる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				settings := postgrestest.NewSettings(postgrestest.PlayerID1)
				settings.UpdatedAt = postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0)
				registerSettings(t, r, settings)

				require.NoError(t, r.Settings.UpdatePlayerSettings(t.Context(), postgrestest.PlayerID1,
					&port.PlayerSettingsPatch{Language: postgrestest.NewPointer("en")}))

				assert.Greater(t, getSettings(t, r, postgrestest.PlayerID1).UpdatedAt, postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0))
			})
			t.Run("別のプレイヤーの言語が「fr」のとき、指定したプレイヤーの言語を「en」に更新すると、別のプレイヤーの取得結果の言語が「fr」のままになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2))
				registerSettings(t, r, postgrestest.NewSettingsWithValues(postgrestest.PlayerID1, "ja", 50, 50, true))
				registerSettings(t, r, postgrestest.NewSettingsWithValues(postgrestest.PlayerID2, "fr", 10, 20, false))

				require.NoError(t, r.Settings.UpdatePlayerSettings(t.Context(), postgrestest.PlayerID1,
					&port.PlayerSettingsPatch{Language: postgrestest.NewPointer("en")}))

				assert.Equal(t, "fr", getSettings(t, r, postgrestest.PlayerID2).Language)
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("設定が登録されていないプレイヤーIDのとき、言語「en」で更新すると、見つからないエラーになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))

				err := r.Settings.UpdatePlayerSettings(t.Context(), postgrestest.PlayerID1,
					&port.PlayerSettingsPatch{Language: postgrestest.NewPointer("en")})

				assert.ErrorIs(t, err, port.ErrNotFound)
			})
		})
	})
}
