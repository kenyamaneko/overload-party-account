//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres/postgrestest"
)

type playerViewGetter func(ctx context.Context, r *postgrestest.Repositories, playerID, firebaseUID string) (*domain.PlayerView, error)

func runPlayerViewLookupCases(t *testing.T, get playerViewGetter) {
	t.Helper()
	t.Run("表示名が「テスト太郎」のとき、取得結果の表示名が「テスト太郎」になる", func(t *testing.T) {
		r := postgrestest.NewRepositories(t, sharedPg)
		r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
			player.Name = postgrestest.NewPointer("テスト太郎")
		}))

		got, err := get(t.Context(), r, postgrestest.PlayerID1, postgrestest.FirebaseUID1)

		require.NoError(t, err)
		assert.Equal(t, postgrestest.NewPointer("テスト太郎"), got.Player.Name)
	})
	t.Run("装備アイコン番号が3のとき、取得結果の装備アイコン番号が3になる", func(t *testing.T) {
		r := postgrestest.NewRepositories(t, sharedPg)
		r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
			player.EquippedIconNo = postgrestest.NewPointer(int64(3))
		}))

		got, err := get(t.Context(), r, postgrestest.PlayerID1, postgrestest.FirebaseUID1)

		require.NoError(t, err)
		assert.Equal(t, postgrestest.NewPointer(int64(3)), got.Player.EquippedIconNo)
	})
	t.Run("プレミアム状態が有効のとき、取得結果のプレミアム状態が有効になる", func(t *testing.T) {
		r := postgrestest.NewRepositories(t, sharedPg)
		r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
			player.IsPremium = true
		}))

		got, err := get(t.Context(), r, postgrestest.PlayerID1, postgrestest.FirebaseUID1)

		require.NoError(t, err)
		assert.True(t, got.Player.IsPremium)
	})
	t.Run("プレミアムの有効期限が2026-12-31T23:59:59Zのとき、取得結果のプレミアムの有効期限が2026-12-31T23:59:59Zになる", func(t *testing.T) {
		r := postgrestest.NewRepositories(t, sharedPg)
		r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
			player.PremiumExpiresAt = postgrestest.NewPointer(postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59))
		}))

		got, err := get(t.Context(), r, postgrestest.PlayerID1, postgrestest.FirebaseUID1)

		require.NoError(t, err)
		assert.Equal(t, []time.Time{postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59)}, postgrestest.ToUTCTimes(got.Player.PremiumExpiresAt))
	})
	t.Run("オンボーディング進行状態がcompletedのとき、取得結果のオンボーディング進行状態がcompletedになる", func(t *testing.T) {
		r := postgrestest.NewRepositories(t, sharedPg)
		r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
			player.OnboardingStatus = "completed"
		}))

		got, err := get(t.Context(), r, postgrestest.PlayerID1, postgrestest.FirebaseUID1)

		require.NoError(t, err)
		assert.Equal(t, "completed", got.Player.OnboardingStatus)
	})
	t.Run("作成日時が2026-01-01T00:00:00Zのとき、取得結果の作成日時が2026-01-01T00:00:00Zになる", func(t *testing.T) {
		r := postgrestest.NewRepositories(t, sharedPg)
		r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
			player.CreatedAt = postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0)
		}))

		got, err := get(t.Context(), r, postgrestest.PlayerID1, postgrestest.FirebaseUID1)

		require.NoError(t, err)
		assert.Equal(t, postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0), got.Player.CreatedAt.UTC())
	})
	t.Run("更新日時が2026-01-01T00:00:00Zのとき、取得結果の更新日時が2026-01-01T00:00:00Zになる", func(t *testing.T) {
		r := postgrestest.NewRepositories(t, sharedPg)
		r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
			player.UpdatedAt = postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0)
		}))

		got, err := get(t.Context(), r, postgrestest.PlayerID1, postgrestest.FirebaseUID1)

		require.NoError(t, err)
		assert.Equal(t, postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0), got.Player.UpdatedAt.UTC())
	})
	t.Run("表示名が未設定のとき、取得結果の表示名も未設定になる", func(t *testing.T) {
		r := postgrestest.NewRepositories(t, sharedPg)
		r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))

		got, err := get(t.Context(), r, postgrestest.PlayerID1, postgrestest.FirebaseUID1)

		require.NoError(t, err)
		assert.Nil(t, got.Player.Name)
	})
	t.Run("装備アイコン番号が未設定のとき、取得結果の装備アイコン番号も未設定になる", func(t *testing.T) {
		r := postgrestest.NewRepositories(t, sharedPg)
		r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))

		got, err := get(t.Context(), r, postgrestest.PlayerID1, postgrestest.FirebaseUID1)

		require.NoError(t, err)
		assert.Nil(t, got.Player.EquippedIconNo)
	})
	t.Run("プレミアムの有効期限が未設定のとき、取得結果のプレミアムの有効期限も未設定になる", func(t *testing.T) {
		r := postgrestest.NewRepositories(t, sharedPg)
		r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))

		got, err := get(t.Context(), r, postgrestest.PlayerID1, postgrestest.FirebaseUID1)

		require.NoError(t, err)
		assert.Nil(t, got.Player.PremiumExpiresAt)
	})
	t.Run("経験値が120のとき、取得結果の経験値が120になる", func(t *testing.T) {
		r := postgrestest.NewRepositories(t, sharedPg)
		r.RegisterPlayerWithProgression(t,
			postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1),
			postgrestest.NewProgressionWithExp(postgrestest.PlayerID1, 120))

		got, err := get(t.Context(), r, postgrestest.PlayerID1, postgrestest.FirebaseUID1)

		require.NoError(t, err)
		assert.Equal(t, int64(120), got.Exp)
	})
	t.Run("初期陣営が未選択のとき", func(t *testing.T) {
		tests := []struct {
			name    string
			prepare func(t *testing.T, r *postgrestest.Repositories)
		}{
			{
				name:    "所持陣営が無いとき、取得結果の初期陣営は未設定になる",
				prepare: func(t *testing.T, r *postgrestest.Repositories) {},
			},
			{
				name: "初期陣営ではない「SHE」だけを所持しているとき、取得結果の初期陣営は未設定になる",
				prepare: func(t *testing.T, r *postgrestest.Repositories) {
					r.AddFaction(t, postgrestest.PlayerID1, "SHE")
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				tt.prepare(t, r)

				got, err := get(t.Context(), r, postgrestest.PlayerID1, postgrestest.FirebaseUID1)

				require.NoError(t, err)
				assert.Nil(t, got.InitialFaction)
			})
		}
	})
	t.Run("初期陣営に「Tenki」を選択済みのとき", func(t *testing.T) {
		tests := []struct {
			name    string
			prepare func(t *testing.T, r *postgrestest.Repositories)
		}{
			{
				name:    "ほかに所持陣営が無いとき、取得結果の初期陣営は「Tenki」になる",
				prepare: func(t *testing.T, r *postgrestest.Repositories) {},
			},
			{
				name: "ほかに「SHE」を所持しているとき、取得結果の初期陣営は「Tenki」になる",
				prepare: func(t *testing.T, r *postgrestest.Repositories) {
					r.AddFaction(t, postgrestest.PlayerID1, "SHE")
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				r.SetInitialFaction(t, postgrestest.PlayerID1, "Tenki")
				tt.prepare(t, r)

				got, err := get(t.Context(), r, postgrestest.PlayerID1, postgrestest.FirebaseUID1)

				require.NoError(t, err)
				assert.Equal(t, postgrestest.NewPointer("Tenki"), got.InitialFaction)
			})
		}
	})
}

func TestPlayerViewLookup(t *testing.T) {
	t.Run("[プレイヤー]参照用情報の取得", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("プレイヤーIDで取得すると", func(t *testing.T) {
				get := func(ctx context.Context, r *postgrestest.Repositories, playerID, _ string) (*domain.PlayerView, error) {
					return r.Views.GetByID(ctx, playerID)
				}
				runPlayerViewLookupCases(t, get)
				t.Run("表示名が「テスト太郎」「テスト次郎」「テスト三郎」の3件のプレイヤーが登録されているとき、「テスト次郎」のプレイヤーIDで取得すると、取得結果の表示名が「テスト次郎」になる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterThreeNamedPlayers(t)

					got, err := get(t.Context(), r, postgrestest.PlayerID2, postgrestest.FirebaseUID2)

					require.NoError(t, err)
					assert.Equal(t, postgrestest.NewPointer("テスト次郎"), got.Player.Name)
				})
			})
			t.Run("FirebaseUIDで取得すると", func(t *testing.T) {
				get := func(ctx context.Context, r *postgrestest.Repositories, _, firebaseUID string) (*domain.PlayerView, error) {
					return r.Views.GetByFirebaseUID(ctx, firebaseUID)
				}
				runPlayerViewLookupCases(t, get)
				t.Run("表示名が「テスト太郎」「テスト次郎」「テスト三郎」の3件のプレイヤーが登録されているとき、「テスト次郎」のFirebaseUIDで取得すると、取得結果の表示名が「テスト次郎」になる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterThreeNamedPlayers(t)

					got, err := get(t.Context(), r, postgrestest.PlayerID2, postgrestest.FirebaseUID2)

					require.NoError(t, err)
					assert.Equal(t, postgrestest.NewPointer("テスト次郎"), got.Player.Name)
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("プレイヤーIDで取得すると", func(t *testing.T) {
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

						_, err := r.Views.GetByID(t.Context(), postgrestest.PlayerID1)

						assert.ErrorIs(t, err, port.ErrNotFound)
					})
				}
			})
			t.Run("FirebaseUIDで取得すると", func(t *testing.T) {
				tests := []struct {
					name        string
					prepare     func(t *testing.T, r *postgrestest.Repositories)
					firebaseUID string
				}{
					{
						name:        "プレイヤーが1件も登録されていないとき、見つからないエラーになる",
						prepare:     func(t *testing.T, r *postgrestest.Repositories) {},
						firebaseUID: postgrestest.FirebaseUID1,
					},
					{
						name: "別のFirebaseUIDのプレイヤーだけが登録されているとき、見つからないエラーになる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2))
						},
						firebaseUID: postgrestest.FirebaseUID1,
					},
					{
						name: "登録済みのFirebaseUIDが「test-firebase-uid-case」で、指定したFirebaseUIDが「TEST-FIREBASE-UID-CASE」のとき、見つからないエラーになる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, "test-firebase-uid-case"))
						},
						firebaseUID: "TEST-FIREBASE-UID-CASE",
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						tt.prepare(t, r)

						_, err := r.Views.GetByFirebaseUID(t.Context(), tt.firebaseUID)

						assert.ErrorIs(t, err, port.ErrNotFound)
					})
				}
			})
		})
	})
}
