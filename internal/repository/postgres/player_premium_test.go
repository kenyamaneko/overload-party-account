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

type premiumUpdate func(t *testing.T, r *postgrestest.Repositories, isPremium bool, premiumExpiresAt *time.Time) error

func readLastPremiumEventOccurredAt(t *testing.T, r *postgrestest.Repositories, playerID string) (*time.Time, error) {
	t.Helper()
	return postgrestest.ReadInTx(t, r.TxManager, func(ctx context.Context) (*time.Time, error) {
		return r.Players.GetLastPremiumEventOccurredAtForUpdate(ctx, playerID)
	})
}

func runPremiumUpdateCases(t *testing.T, update premiumUpdate) {
	t.Run("更新前と更新するプレミアム状態を指定すると", func(t *testing.T) {
		tests := []struct {
			name   string
			before bool
			next   bool
			want   bool
		}{
			{name: "更新前が無効で、更新するプレミアム状態が有効のとき、取得結果のプレミアム状態が有効になる", before: false, next: true, want: true},
			{name: "更新前が有効で、更新するプレミアム状態が無効のとき、取得結果のプレミアム状態が無効になる", before: true, next: false, want: false},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
					player.IsPremium = tt.before
				}))

				require.NoError(t, update(t, r, tt.next, nil))

				assert.Equal(t, tt.want, r.GetPlayer(t, postgrestest.PlayerID1).IsPremium)
			})
		}
	})
	t.Run("更新前と更新するプレミアムの有効期限を指定すると", func(t *testing.T) {
		tests := []struct {
			name   string
			before *time.Time
			next   *time.Time
			want   *time.Time
		}{
			{
				name:   "更新前が未設定で、更新する有効期限が2026-12-31T23:59:59Zのとき、取得結果のプレミアムの有効期限が2026-12-31T23:59:59Zになる",
				before: nil,
				next:   postgrestest.NewPointer(postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59)),
				want:   postgrestest.NewPointer(postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59)),
			},
			{
				name:   "更新前が2026-12-31T23:59:59Zで、更新する有効期限が2027-01-31T23:59:59Zのとき、取得結果のプレミアムの有効期限が2027-01-31T23:59:59Zになる",
				before: postgrestest.NewPointer(postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59)),
				next:   postgrestest.NewPointer(postgrestest.NewUTCTime(2027, time.January, 31, 23, 59, 59)),
				want:   postgrestest.NewPointer(postgrestest.NewUTCTime(2027, time.January, 31, 23, 59, 59)),
			},
			{
				name:   "更新前が2026-12-31T23:59:59Zで、更新する有効期限が未設定のとき、取得結果のプレミアムの有効期限が未設定になる",
				before: postgrestest.NewPointer(postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59)),
				next:   nil,
				want:   nil,
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
					player.PremiumExpiresAt = tt.before
				}))

				require.NoError(t, update(t, r, true, tt.next))

				assert.Equal(t, postgrestest.ToUTCTimes(tt.want), postgrestest.ToUTCTimes(r.GetPlayer(t, postgrestest.PlayerID1).PremiumExpiresAt))
			})
		}
	})
	t.Run("更新日時が2026-01-01T00:00:00Zのとき、取得結果の更新日時が更新を実行した日時になる", func(t *testing.T) {
		r := postgrestest.NewRepositories(t, sharedPg)
		r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
			player.UpdatedAt = postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0)
		}))
		before := postgrestest.ReadDatabaseTime(t, r.Pool)

		require.NoError(t, update(t, r, true,
			postgrestest.NewPointer(postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59))))

		after := postgrestest.ReadDatabaseTime(t, r.Pool)
		updatedAt := r.GetPlayer(t, postgrestest.PlayerID1).UpdatedAt
		assert.GreaterOrEqual(t, updatedAt, before)
		assert.LessOrEqual(t, updatedAt, after)
	})
	t.Run("FirebaseUIDが「test-uid-1」のとき、取得結果のFirebaseUIDが「test-uid-1」のままになる", func(t *testing.T) {
		r := postgrestest.NewRepositories(t, sharedPg)
		r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, "test-uid-1"))

		require.NoError(t, update(t, r, true,
			postgrestest.NewPointer(postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59))))

		assert.Equal(t, "test-uid-1", r.GetPlayer(t, postgrestest.PlayerID1).FirebaseUID)
	})
	t.Run("表示名が「テスト太郎」のとき、取得結果の表示名が「テスト太郎」のままになる", func(t *testing.T) {
		r := postgrestest.NewRepositories(t, sharedPg)
		r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
			player.Name = postgrestest.NewPointer("テスト太郎")
		}))

		require.NoError(t, update(t, r, true,
			postgrestest.NewPointer(postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59))))

		assert.Equal(t, postgrestest.NewPointer("テスト太郎"), r.GetPlayer(t, postgrestest.PlayerID1).Name)
	})
	t.Run("装備アイコン番号が3のとき、取得結果の装備アイコン番号が3のままになる", func(t *testing.T) {
		r := postgrestest.NewRepositories(t, sharedPg)
		r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
			player.EquippedIconNo = postgrestest.NewPointer(int64(3))
		}))

		require.NoError(t, update(t, r, true,
			postgrestest.NewPointer(postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59))))

		assert.Equal(t, postgrestest.NewPointer(int64(3)), r.GetPlayer(t, postgrestest.PlayerID1).EquippedIconNo)
	})
	t.Run("オンボーディング進行状態がcompletedのとき、取得結果のオンボーディング進行状態がcompletedのままになる", func(t *testing.T) {
		r := postgrestest.NewRepositories(t, sharedPg)
		r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
			player.OnboardingStatus = "completed"
		}))

		require.NoError(t, update(t, r, true,
			postgrestest.NewPointer(postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59))))

		assert.Equal(t, "completed", r.GetPlayer(t, postgrestest.PlayerID1).OnboardingStatus)
	})
	t.Run("作成日時が2026-01-01T00:00:00Zのとき、取得結果の作成日時が2026-01-01T00:00:00Zのままになる", func(t *testing.T) {
		r := postgrestest.NewRepositories(t, sharedPg)
		r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
			player.CreatedAt = postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0)
		}))

		require.NoError(t, update(t, r, true,
			postgrestest.NewPointer(postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59))))

		assert.Equal(t, postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0), r.GetPlayer(t, postgrestest.PlayerID1).CreatedAt.UTC())
	})
	t.Run("別のプレイヤーのプレミアム状態が無効のとき、指定したプレイヤーのプレミアム状態を有効にすると、別のプレイヤーの取得結果のプレミアム状態が無効のままになる", func(t *testing.T) {
		r := postgrestest.NewRepositories(t, sharedPg)
		r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
		r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2))

		require.NoError(t, update(t, r, true,
			postgrestest.NewPointer(postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59))))

		assert.False(t, r.GetPlayer(t, postgrestest.PlayerID2).IsPremium)
	})
}

func TestPlayerPremiumUpdate(t *testing.T) {
	t.Run("[プレイヤー]プレミアム状態更新", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("プレミアム状態を更新してプレイヤーIDで取得すると", func(t *testing.T) {
				runPremiumUpdateCases(t, func(t *testing.T, r *postgrestest.Repositories, isPremium bool, premiumExpiresAt *time.Time) error {
					return r.Players.UpdatePremium(t.Context(), postgrestest.PlayerID1, isPremium, premiumExpiresAt)
				})
			})
			t.Run("発生時刻を指定してプレミアム状態を更新してプレイヤーIDで取得すると", func(t *testing.T) {
				runPremiumUpdateCases(t, func(t *testing.T, r *postgrestest.Repositories, isPremium bool, premiumExpiresAt *time.Time) error {
					return r.Players.UpdatePremiumFromEvent(t.Context(), postgrestest.PlayerID1, isPremium, premiumExpiresAt,
						postgrestest.NewUTCTime(2026, time.February, 1, 0, 0, 0))
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("未登録のプレイヤーIDのとき、プレミアム状態を更新すると、見つからないエラーになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)

				err := r.Players.UpdatePremium(t.Context(), postgrestest.UnregisteredPlayerID, true, nil)

				assert.ErrorIs(t, err, port.ErrNotFound)
			})
			t.Run("未登録のプレイヤーIDのとき、発生時刻を指定してプレミアム状態を更新すると、見つからないエラーになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)

				err := r.Players.UpdatePremiumFromEvent(t.Context(), postgrestest.UnregisteredPlayerID, true, nil,
					postgrestest.NewUTCTime(2026, time.February, 1, 0, 0, 0))

				assert.ErrorIs(t, err, port.ErrNotFound)
			})
		})
	})
}

func TestPlayerLastPremiumEventOccurredAtLookup(t *testing.T) {
	t.Run("[プレイヤー]最後に反映したプレミアム状態更新イベントの発生時刻の取得", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("発生時刻2026-02-01T00:00:00Zを指定してプレミアム状態を更新してから取得すると、取得結果が2026-02-01T00:00:00Zになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				require.NoError(t, r.Players.UpdatePremiumFromEvent(t.Context(), postgrestest.PlayerID1, true, nil,
					postgrestest.NewUTCTime(2026, time.February, 1, 0, 0, 0)))

				got, err := readLastPremiumEventOccurredAt(t, r, postgrestest.PlayerID1)

				require.NoError(t, err)
				assert.Equal(t, []time.Time{postgrestest.NewUTCTime(2026, time.February, 1, 0, 0, 0)}, postgrestest.ToUTCTimes(got))
			})
			t.Run("発生時刻2026-02-01T00:00:00Zで更新済みのプレイヤーを更新してから取得すると", func(t *testing.T) {
				t.Run("発生時刻2026-03-01T00:00:00Zで更新したとき、取得結果が2026-03-01T00:00:00Zになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
					require.NoError(t, r.Players.UpdatePremiumFromEvent(t.Context(), postgrestest.PlayerID1, true, nil,
						postgrestest.NewUTCTime(2026, time.February, 1, 0, 0, 0)))
					require.NoError(t, r.Players.UpdatePremiumFromEvent(t.Context(), postgrestest.PlayerID1, true, nil,
						postgrestest.NewUTCTime(2026, time.March, 1, 0, 0, 0)))

					got, err := readLastPremiumEventOccurredAt(t, r, postgrestest.PlayerID1)

					require.NoError(t, err)
					assert.Equal(t, []time.Time{postgrestest.NewUTCTime(2026, time.March, 1, 0, 0, 0)}, postgrestest.ToUTCTimes(got))
				})
				t.Run("発生時刻を伴わずに更新したとき、取得結果が2026-02-01T00:00:00Zのままになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
					require.NoError(t, r.Players.UpdatePremiumFromEvent(t.Context(), postgrestest.PlayerID1, true, nil,
						postgrestest.NewUTCTime(2026, time.February, 1, 0, 0, 0)))
					require.NoError(t, r.Players.UpdatePremium(t.Context(), postgrestest.PlayerID1, true,
						postgrestest.NewPointer(postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59))))

					got, err := readLastPremiumEventOccurredAt(t, r, postgrestest.PlayerID1)

					require.NoError(t, err)
					assert.Equal(t, []time.Time{postgrestest.NewUTCTime(2026, time.February, 1, 0, 0, 0)}, postgrestest.ToUTCTimes(got))
				})
			})
			t.Run("まだプレミアム状態を更新していないプレイヤーを取得すると", func(t *testing.T) {
				tests := []struct {
					name    string
					prepare func(t *testing.T, r *postgrestest.Repositories)
				}{
					{
						name:    "他のプレイヤーが登録されていないとき、取得結果は未設定になる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {},
					},
					{
						name: "別のプレイヤーが発生時刻を指定してプレミアム状態を更新済みのとき、取得結果は未設定になる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2))
							require.NoError(t, r.Players.UpdatePremiumFromEvent(t.Context(), postgrestest.PlayerID2, true, nil,
								postgrestest.NewUTCTime(2026, time.February, 1, 0, 0, 0)))
						},
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
						tt.prepare(t, r)

						got, err := readLastPremiumEventOccurredAt(t, r, postgrestest.PlayerID1)

						require.NoError(t, err)
						assert.Nil(t, got)
					})
				}
			})
			t.Run("別のプレイヤーが発生時刻2026-02-01T00:00:00Zで更新済みのとき、指定したプレイヤーを発生時刻2026-03-01T00:00:00Zで更新すると、別のプレイヤーの取得結果は2026-02-01T00:00:00Zのままになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2))
				require.NoError(t, r.Players.UpdatePremiumFromEvent(t.Context(), postgrestest.PlayerID2, true, nil,
					postgrestest.NewUTCTime(2026, time.February, 1, 0, 0, 0)))
				require.NoError(t, r.Players.UpdatePremiumFromEvent(t.Context(), postgrestest.PlayerID1, true, nil,
					postgrestest.NewUTCTime(2026, time.March, 1, 0, 0, 0)))

				got, err := readLastPremiumEventOccurredAt(t, r, postgrestest.PlayerID2)

				require.NoError(t, err)
				assert.Equal(t, []time.Time{postgrestest.NewUTCTime(2026, time.February, 1, 0, 0, 0)}, postgrestest.ToUTCTimes(got))
			})
			t.Run("別のトランザクションが同じプレイヤーのプレミアム状態を更新している途中のとき、取得すると", func(t *testing.T) {
				start := func(t *testing.T) (*postgrestest.Repositories, *postgrestest.UncommittedUpdate, <-chan postgrestest.ReadResult[*time.Time]) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
					update := postgrestest.HoldUncommittedUpdate(t, r.TxManager, func(ctx context.Context) error {
						return r.Players.UpdatePremiumFromEvent(ctx, postgrestest.PlayerID1, true, nil,
							postgrestest.NewUTCTime(2026, time.March, 1, 0, 0, 0))
					})
					results := postgrestest.StartExclusiveRead(r.TxManager, func(ctx context.Context) (*time.Time, error) {
						return r.Players.GetLastPremiumEventOccurredAtForUpdate(ctx, postgrestest.PlayerID1)
					})
					return r, update, results
				}
				t.Run("更新しているトランザクションが確定するまで、取得結果は返らない", func(t *testing.T) {
					r, update, results := start(t)
					postgrestest.RequireLockWaiter(t, r.Pool)

					assert.Empty(t, results)

					update.Commit(t)
					<-results
				})
				t.Run("発生時刻2026-03-01T00:00:00Zで更新しているとき、更新しているトランザクションが確定した後は、取得結果が2026-03-01T00:00:00Zになる", func(t *testing.T) {
					r, update, results := start(t)
					postgrestest.RequireLockWaiter(t, r.Pool)

					update.Commit(t)
					result := <-results

					require.NoError(t, result.Err)
					assert.Equal(t, []time.Time{postgrestest.NewUTCTime(2026, time.March, 1, 0, 0, 0)}, postgrestest.ToUTCTimes(result.Value))
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("未登録のプレイヤーIDのとき、取得すると、見つからないエラーになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)

				_, err := readLastPremiumEventOccurredAt(t, r, postgrestest.UnregisteredPlayerID)

				assert.ErrorIs(t, err, port.ErrNotFound)
			})
		})
	})
}
