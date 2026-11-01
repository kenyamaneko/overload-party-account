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

func TestPlayerProgressionLookup(t *testing.T) {
	t.Run("[プレイヤー]累計経験値の取得", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			registerTargetOnly := func(t *testing.T, r *postgrestest.Repositories) {
				r.RegisterPlayerWithProgression(t,
					postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1),
					postgrestest.NewProgressionWithExp(postgrestest.PlayerID1, 120))
			}
			registerTargetAndOther := func(t *testing.T, r *postgrestest.Repositories) {
				registerTargetOnly(t, r)
				r.RegisterPlayerWithProgression(t,
					postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2),
					postgrestest.NewProgressionWithExp(postgrestest.PlayerID2, 900))
			}
			lookup := func(t *testing.T, prepare func(t *testing.T, r *postgrestest.Repositories)) *domain.PlayerProgression {
				t.Helper()
				r := postgrestest.NewRepositories(t, sharedPg)
				prepare(t, r)

				got, err := postgrestest.ReadInTx(t, r.TxManager, func(ctx context.Context) (*domain.PlayerProgression, error) {
					return r.Players.GetProgressionForUpdate(ctx, postgrestest.PlayerID1)
				})

				require.NoError(t, err)
				return got
			}
			t.Run("経験値を取得すると", func(t *testing.T) {
				tests := []struct {
					name    string
					prepare func(t *testing.T, r *postgrestest.Repositories)
				}{
					{
						name:    "経験値が120のとき、取得結果の経験値が120になる",
						prepare: registerTargetOnly,
					},
					{
						name:    "指定したプレイヤーの経験値が120で、別のプレイヤーの経験値が900のとき、取得結果の経験値が120になる",
						prepare: registerTargetAndOther,
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						got := lookup(t, tt.prepare)

						assert.Equal(t, int64(120), got.Exp)
					})
				}
			})
			t.Run("別のトランザクションが同じプレイヤーの経験値を更新している途中のとき、取得すると", func(t *testing.T) {
				start := func(t *testing.T) (*postgrestest.Repositories, *postgrestest.UncommittedUpdate, <-chan postgrestest.ReadResult[*domain.PlayerProgression]) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayerWithProgression(t,
						postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1),
						postgrestest.NewProgressionWithExp(postgrestest.PlayerID1, 120))
					update := postgrestest.HoldUncommittedUpdate(t, r.TxManager, func(ctx context.Context) error {
						return r.Players.UpdateProgression(ctx, postgrestest.PlayerID1, 200)
					})
					results := postgrestest.StartExclusiveRead(r.TxManager, func(ctx context.Context) (*domain.PlayerProgression, error) {
						return r.Players.GetProgressionForUpdate(ctx, postgrestest.PlayerID1)
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
				t.Run("経験値を200に更新しているとき、更新しているトランザクションが確定した後は、取得結果の経験値が200になる", func(t *testing.T) {
					r, update, results := start(t)
					postgrestest.RequireLockWaiter(t, r.Pool)

					update.Commit(t)
					result := <-results

					require.NoError(t, result.Err)
					assert.Equal(t, int64(200), result.Value.Exp)
				})
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
							r.RegisterPlayerWithProgression(t,
								postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2),
								postgrestest.NewProgressionWithExp(postgrestest.PlayerID2, 900))
						},
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						tt.prepare(t, r)

						_, err := postgrestest.ReadInTx(t, r.TxManager, func(ctx context.Context) (*domain.PlayerProgression, error) {
							return r.Players.GetProgressionForUpdate(ctx, postgrestest.PlayerID1)
						})

						assert.ErrorIs(t, err, port.ErrNotFound)
					})
				}
			})
		})
	})
}

func TestPlayerProgressionUpdate(t *testing.T) {
	t.Run("[プレイヤー]累計経験値の更新", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("登録済みのプレイヤーの経験値を120に更新して取得すると、取得結果の経験値が120になる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))

				require.NoError(t, r.Players.UpdateProgression(t.Context(), postgrestest.PlayerID1, 120))

				assert.Equal(t, int64(120), r.GetProgression(t, postgrestest.PlayerID1).Exp)
			})
			t.Run("更新日時が2026-01-01T00:00:00Zのとき、更新すると、取得結果の更新日時が2026-01-01T00:00:00Zより後になる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))

				require.NoError(t, r.Players.UpdateProgression(t.Context(), postgrestest.PlayerID1, 120))

				assert.Greater(t, r.GetProgression(t, postgrestest.PlayerID1).UpdatedAt, postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0))
			})
			t.Run("別のプレイヤーの経験値が900のとき、指定したプレイヤーの経験値を120に更新すると、別のプレイヤーの取得結果の経験値が900のままになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				r.RegisterPlayerWithProgression(t,
					postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2),
					postgrestest.NewProgressionWithExp(postgrestest.PlayerID2, 900))

				require.NoError(t, r.Players.UpdateProgression(t.Context(), postgrestest.PlayerID1, 120))

				assert.Equal(t, int64(900), r.GetProgression(t, postgrestest.PlayerID2).Exp)
			})
		})
	})
}
