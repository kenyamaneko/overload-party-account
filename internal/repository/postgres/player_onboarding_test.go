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

func TestPlayerOnboardingStatusLookup(t *testing.T) {
	t.Run("[プレイヤー]オンボーディング進行状態の取得", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("取得すると", func(t *testing.T) {
				tests := []struct {
					name    string
					prepare func(t *testing.T, r *postgrestest.Repositories)
				}{
					{
						name: "オンボーディング進行状態がname_setのとき、name_setが返る",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
								player.OnboardingStatus = "name_set"
							}))
						},
					},
					{
						name: "指定したプレイヤーのオンボーディング進行状態がname_setで、別のプレイヤーのオンボーディング進行状態がfaction_setのとき、name_setが返る",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
								player.OnboardingStatus = "name_set"
							}))
							r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID2, postgrestest.FirebaseUID2, func(player *domain.Player) {
								player.OnboardingStatus = "faction_set"
							}))
						},
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						tt.prepare(t, r)

						got, err := r.Players.GetOnboardingStatus(t.Context(), postgrestest.PlayerID1)

						require.NoError(t, err)
						assert.Equal(t, "name_set", got)
					})
				}
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("取得すると", func(t *testing.T) {
				tests := []struct {
					name    string
					prepare func(t *testing.T, r *postgrestest.Repositories)
				}{
					{
						name:    "プレイヤーが1件も登録されていないとき、見つからないエラーになる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {},
					},
					{
						name: "別のプレイヤーだけが登録されているとき、見つからないエラーになる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2))
						},
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						tt.prepare(t, r)

						_, err := r.Players.GetOnboardingStatus(t.Context(), postgrestest.PlayerID1)

						assert.ErrorIs(t, err, port.ErrNotFound)
					})
				}
			})
		})
	})
}

func TestPlayerOnboardingStatusUpdate(t *testing.T) {
	t.Run("[プレイヤー]オンボーディング進行状態の更新", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("登録済みのプレイヤーのオンボーディング進行状態を更新してから取得すると", func(t *testing.T) {
				t.Run("name_setに更新すると、取得したオンボーディング進行状態がname_setになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
						player.OnboardingStatus = "not_started"
					}))

					require.NoError(t, r.Players.UpdateOnboardingStatus(t.Context(), postgrestest.PlayerID1, "name_set"))

					got, err := r.Players.GetOnboardingStatus(t.Context(), postgrestest.PlayerID1)
					require.NoError(t, err)
					assert.Equal(t, "name_set", got)
				})
				t.Run("faction_setに更新すると、取得したオンボーディング進行状態がfaction_setになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
						player.OnboardingStatus = "name_set"
					}))

					require.NoError(t, r.Players.UpdateOnboardingStatus(t.Context(), postgrestest.PlayerID1, "faction_set"))

					got, err := r.Players.GetOnboardingStatus(t.Context(), postgrestest.PlayerID1)
					require.NoError(t, err)
					assert.Equal(t, "faction_set", got)
				})
				t.Run("completedに更新すると、取得したオンボーディング進行状態がcompletedになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
						player.OnboardingStatus = "faction_set"
					}))

					require.NoError(t, r.Players.UpdateOnboardingStatus(t.Context(), postgrestest.PlayerID1, "completed"))

					got, err := r.Players.GetOnboardingStatus(t.Context(), postgrestest.PlayerID1)
					require.NoError(t, err)
					assert.Equal(t, "completed", got)
				})
			})
			t.Run("更新日時が2026-01-01T00:00:00Zのとき、オンボーディング進行状態を更新すると、取得結果の更新日時が2026-01-01T00:00:00Zより後になる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
					player.UpdatedAt = postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0)
				}))

				require.NoError(t, r.Players.UpdateOnboardingStatus(t.Context(), postgrestest.PlayerID1, "name_set"))

				assert.Greater(t, r.GetPlayer(t, postgrestest.PlayerID1).UpdatedAt, postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0))
			})
			t.Run("FirebaseUIDが「test-uid-1」のとき、オンボーディング進行状態を更新すると、取得結果のFirebaseUIDが「test-uid-1」のままになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, "test-uid-1"))

				require.NoError(t, r.Players.UpdateOnboardingStatus(t.Context(), postgrestest.PlayerID1, "name_set"))

				assert.Equal(t, "test-uid-1", r.GetPlayer(t, postgrestest.PlayerID1).FirebaseUID)
			})
			t.Run("表示名が「テスト太郎」のとき、オンボーディング進行状態を更新すると、取得結果の表示名が「テスト太郎」のままになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
					player.Name = postgrestest.NewPointer("テスト太郎")
				}))

				require.NoError(t, r.Players.UpdateOnboardingStatus(t.Context(), postgrestest.PlayerID1, "name_set"))

				assert.Equal(t, postgrestest.NewPointer("テスト太郎"), r.GetPlayer(t, postgrestest.PlayerID1).Name)
			})
			t.Run("装備アイコン番号が3のとき、オンボーディング進行状態を更新すると、取得結果の装備アイコン番号が3のままになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
					player.EquippedIconNo = postgrestest.NewPointer(int64(3))
				}))

				require.NoError(t, r.Players.UpdateOnboardingStatus(t.Context(), postgrestest.PlayerID1, "name_set"))

				assert.Equal(t, postgrestest.NewPointer(int64(3)), r.GetPlayer(t, postgrestest.PlayerID1).EquippedIconNo)
			})
			t.Run("プレミアム状態が有効のとき、オンボーディング進行状態を更新すると、取得結果のプレミアム状態が有効のままになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
					player.IsPremium = true
				}))

				require.NoError(t, r.Players.UpdateOnboardingStatus(t.Context(), postgrestest.PlayerID1, "name_set"))

				assert.True(t, r.GetPlayer(t, postgrestest.PlayerID1).IsPremium)
			})
			t.Run("プレミアムの有効期限が2026-12-31T23:59:59Zのとき、オンボーディング進行状態を更新すると、取得結果のプレミアムの有効期限が2026-12-31T23:59:59Zのままになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
					player.PremiumExpiresAt = postgrestest.NewPointer(postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59))
				}))

				require.NoError(t, r.Players.UpdateOnboardingStatus(t.Context(), postgrestest.PlayerID1, "name_set"))

				assert.Equal(t, []time.Time{postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59)},
					postgrestest.ToUTCTimes(r.GetPlayer(t, postgrestest.PlayerID1).PremiumExpiresAt))
			})
			t.Run("作成日時が2026-01-01T00:00:00Zのとき、オンボーディング進行状態を更新すると、取得結果の作成日時が2026-01-01T00:00:00Zのままになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
					player.CreatedAt = postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0)
				}))

				require.NoError(t, r.Players.UpdateOnboardingStatus(t.Context(), postgrestest.PlayerID1, "name_set"))

				assert.Equal(t, postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0), r.GetPlayer(t, postgrestest.PlayerID1).CreatedAt.UTC())
			})
			t.Run("別のプレイヤーのオンボーディング進行状態がnot_startedのとき、指定したプレイヤーのオンボーディング進行状態をname_setに更新すると、別のプレイヤーの取得結果のオンボーディング進行状態がnot_startedのままになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID2, postgrestest.FirebaseUID2, func(player *domain.Player) {
					player.OnboardingStatus = "not_started"
				}))

				require.NoError(t, r.Players.UpdateOnboardingStatus(t.Context(), postgrestest.PlayerID1, "name_set"))

				got, err := r.Players.GetOnboardingStatus(t.Context(), postgrestest.PlayerID2)
				require.NoError(t, err)
				assert.Equal(t, "not_started", got)
			})
		})
	})
}
