//go:build integration

package postgres_test

import (
	"context"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres/postgrestest"
)

func gameDate(year int, month time.Month, day int) civil.Date {
	return civil.Date{Year: year, Month: month, Day: day}
}

func getDailyBattle(t *testing.T, r *postgrestest.Repositories, playerID string, date civil.Date) *domain.PlayerDailyBattle {
	t.Helper()
	battle, err := r.Players.GetDailyBattle(t.Context(), playerID, date)
	require.NoError(t, err)
	return battle
}

func incrementDailyBattle(t *testing.T, r *postgrestest.Repositories, playerID string, date civil.Date, times int) {
	t.Helper()
	for range times {
		_, err := r.Players.IncrementDailyBattleCount(t.Context(), playerID, date)
		require.NoError(t, err)
	}
}

func TestPlayerDailyBattleLookup(t *testing.T) {
	t.Run("[プレイヤー]日次バトル回数の取得", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("ゲーム日2026-04-01の日次バトル回数が3のとき、ゲーム日2026-04-01で取得すると、日次バトル回数3が返る", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				incrementDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1), 3)

				got := getDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1))

				require.NotNil(t, got)
				assert.Equal(t, int64(3), got.DailyBattleCount)
			})
			t.Run("ゲーム日2026-04-01の日次バトル回数があるとき、ゲーム日2026-04-01で取得すると、返るゲーム日が2026-04-01になる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				incrementDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1), 1)

				got := getDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1))

				require.NotNil(t, got)
				assert.Equal(t, gameDate(2026, time.April, 1), got.GameDate)
			})
			t.Run("ゲーム日2026-04-01の日次バトル回数があり、ゲーム日2026-04-02の日次バトル回数が5のとき、ゲーム日2026-04-02で取得すると、日次バトル回数5が返る", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				incrementDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1), 1)
				incrementDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 2), 5)

				got := getDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 2))

				require.NotNil(t, got)
				assert.Equal(t, int64(5), got.DailyBattleCount)
			})
			t.Run("ゲーム日2026-04-01を指定して取得すると", func(t *testing.T) {
				tests := []struct {
					name    string
					prepare func(t *testing.T, r *postgrestest.Repositories)
				}{
					{
						name: "ゲーム日2026-04-02の日次バトル回数だけがあるとき、該当なしになる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							incrementDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 2), 1)
						},
					},
					{
						name: "別のプレイヤーの日次バトル回数だけがあるとき、該当なしになる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2))
							incrementDailyBattle(t, r, postgrestest.PlayerID2, gameDate(2026, time.April, 1), 1)
						},
					},
					{
						name:    "日次バトル回数が1件も無いとき、該当なしになる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {},
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
						tt.prepare(t, r)

						got := getDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1))

						assert.Nil(t, got)
					})
				}
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("呼び出しが取り消し済みのとき、取得すると、取り消しを示すエラーになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				incrementDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1), 1)
				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				_, err := r.Players.GetDailyBattle(ctx, postgrestest.PlayerID1, gameDate(2026, time.April, 1))

				assert.ErrorIs(t, err, context.Canceled)
			})
		})
	})
}

func TestPlayerDailyBattleExclusiveLookup(t *testing.T) {
	t.Run("[プレイヤー]日次バトル回数の排他取得", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("ゲーム日2026-04-01の日次バトル回数が1件も無いとき、トランザクション内でゲーム日2026-04-01を取得すると、該当なしになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))

				got, err := postgrestest.ReadInTx(t, r.TxManager, func(ctx context.Context) (*domain.PlayerDailyBattle, error) {
					return r.Players.GetDailyBattleForUpdate(ctx, postgrestest.PlayerID1, gameDate(2026, time.April, 1))
				})

				require.NoError(t, err)
				assert.Nil(t, got)
			})
			t.Run("ゲーム日2026-04-01の日次バトル回数が3のとき、トランザクション内でゲーム日2026-04-01を取得すると、日次バトル回数3が返る", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				incrementDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1), 3)

				got, err := postgrestest.ReadInTx(t, r.TxManager, func(ctx context.Context) (*domain.PlayerDailyBattle, error) {
					return r.Players.GetDailyBattleForUpdate(ctx, postgrestest.PlayerID1, gameDate(2026, time.April, 1))
				})

				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, int64(3), got.DailyBattleCount)
			})
			t.Run("別のトランザクションが同じゲーム日の日次バトル回数を加算している途中のとき、トランザクション内でゲーム日2026-04-01を取得すると", func(t *testing.T) {
				start := func(t *testing.T) (*postgrestest.Repositories, *postgrestest.UncommittedUpdate, <-chan postgrestest.ReadResult[*domain.PlayerDailyBattle]) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
					incrementDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1), 3)
					update := postgrestest.HoldUncommittedUpdate(t, r.TxManager, func(ctx context.Context) error {
						_, err := r.Players.IncrementDailyBattleCount(ctx, postgrestest.PlayerID1, gameDate(2026, time.April, 1))
						return err
					})
					results := postgrestest.StartExclusiveRead(r.TxManager, func(ctx context.Context) (*domain.PlayerDailyBattle, error) {
						return r.Players.GetDailyBattleForUpdate(ctx, postgrestest.PlayerID1, gameDate(2026, time.April, 1))
					})
					return r, update, results
				}
				t.Run("加算しているトランザクションが確定するまで、取得結果は返らない", func(t *testing.T) {
					r, update, results := start(t)
					postgrestest.RequireLockWaiter(t, r.Pool)

					assert.Empty(t, results)

					update.Commit(t)
					<-results
				})
				t.Run("ゲーム日2026-04-01の日次バトル回数が3のとき、加算しているトランザクションが確定した後は、日次バトル回数4が返る", func(t *testing.T) {
					r, update, results := start(t)
					postgrestest.RequireLockWaiter(t, r.Pool)

					update.Commit(t)
					result := <-results

					require.NoError(t, result.Err)
					require.NotNil(t, result.Value)
					assert.Equal(t, int64(4), result.Value.DailyBattleCount)
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("呼び出しが取り消し済みのとき、トランザクション内で取得すると、取り消しを示すエラーになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				incrementDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1), 1)
				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				err := r.TxManager.RunInTx(ctx, func(ctx context.Context) error {
					_, err := r.Players.GetDailyBattleForUpdate(ctx, postgrestest.PlayerID1, gameDate(2026, time.April, 1))
					return err
				})

				assert.ErrorIs(t, err, context.Canceled)
			})
		})
	})
}

func TestPlayerDailyBattleIncrement(t *testing.T) {
	t.Run("[プレイヤー]日次バトル回数の加算", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("加算すると", func(t *testing.T) {
				tests := []struct {
					name     string
					prepare  func(t *testing.T, r *postgrestest.Repositories)
					gameDate civil.Date
				}{
					{
						name:     "日次バトル回数が1件も無いとき、加算結果が1になる",
						prepare:  func(t *testing.T, r *postgrestest.Repositories) {},
						gameDate: gameDate(2026, time.April, 1),
					},
					{
						name: "ゲーム日2026-04-01の日次バトル回数があり、加算するゲーム日が2026-04-02のとき、加算結果が1になる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							incrementDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1), 1)
						},
						gameDate: gameDate(2026, time.April, 2),
					},
					{
						name: "別のプレイヤーの日次バトル回数があるとき、加算結果が1になる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2))
							incrementDailyBattle(t, r, postgrestest.PlayerID2, gameDate(2026, time.April, 1), 1)
						},
						gameDate: gameDate(2026, time.April, 1),
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
						tt.prepare(t, r)

						got, err := r.Players.IncrementDailyBattleCount(t.Context(), postgrestest.PlayerID1, tt.gameDate)

						require.NoError(t, err)
						assert.Equal(t, int64(1), got)
					})
				}
			})
			t.Run("日次バトル回数が1件も無いとき、加算すると、その後の取得結果の日次バトル回数が1になる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				_, err := r.Players.IncrementDailyBattleCount(t.Context(), postgrestest.PlayerID1, gameDate(2026, time.April, 1))
				require.NoError(t, err)

				got := getDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1))

				require.NotNil(t, got)
				assert.Equal(t, int64(1), got.DailyBattleCount)
			})
			t.Run("日次バトル回数が1のとき、加算すると、加算結果が2になる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				incrementDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1), 1)

				got, err := r.Players.IncrementDailyBattleCount(t.Context(), postgrestest.PlayerID1, gameDate(2026, time.April, 1))

				require.NoError(t, err)
				assert.Equal(t, int64(2), got)
			})
			t.Run("ゲーム日2026-04-01の日次バトル回数が3のとき、ゲーム日2026-04-02を加算すると、その後のゲーム日2026-04-01の取得結果の日次バトル回数は3のままになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				incrementDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1), 3)
				_, err := r.Players.IncrementDailyBattleCount(t.Context(), postgrestest.PlayerID1, gameDate(2026, time.April, 2))
				require.NoError(t, err)

				got := getDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1))

				require.NotNil(t, got)
				assert.Equal(t, int64(3), got.DailyBattleCount)
			})
			t.Run("別のプレイヤーの日次バトル回数が3のとき、加算すると、その後の別のプレイヤーの取得結果の日次バトル回数は3のままになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2))
				incrementDailyBattle(t, r, postgrestest.PlayerID2, gameDate(2026, time.April, 1), 3)
				_, err := r.Players.IncrementDailyBattleCount(t.Context(), postgrestest.PlayerID1, gameDate(2026, time.April, 1))
				require.NoError(t, err)

				got := getDailyBattle(t, r, postgrestest.PlayerID2, gameDate(2026, time.April, 1))

				require.NotNil(t, got)
				assert.Equal(t, int64(3), got.DailyBattleCount)
			})
			t.Run("ゲーム日2026-04-01の日次バトル回数が1件も無いとき、ゲーム日2026-04-01に10回を同時に加算すると、加算結果は1から10までの値が重複なく1回ずつになる", func(t *testing.T) {
				type incrementResult struct {
					count int64
					err   error
				}
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))

				results := postgrestest.RunConcurrently(10, func(int) incrementResult {
					count, err := r.Players.IncrementDailyBattleCount(t.Context(), postgrestest.PlayerID1, gameDate(2026, time.April, 1))
					return incrementResult{count: count, err: err}
				})

				counts := make([]int64, 0, len(results))
				for _, result := range results {
					require.NoError(t, result.err)
					counts = append(counts, result.count)
				}
				assert.ElementsMatch(t, []int64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, counts)
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("未登録のプレイヤーIDのとき、加算すると、存在しないプレイヤーを指すエラーになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)

				_, err := r.Players.IncrementDailyBattleCount(t.Context(), postgrestest.UnregisteredPlayerID, gameDate(2026, time.April, 1))

				postgrestest.RequirePgError(t, err, postgrestest.SQLStateForeignKeyViolation)
			})
		})
	})
}

func TestPlayerDailyBattleDecrement(t *testing.T) {
	t.Run("[プレイヤー]日次バトル回数の減算", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("ゲーム日2026-04-01の日次バトル回数を減算してから取得すると", func(t *testing.T) {
				tests := []struct {
					name  string
					count int
					want  int64
				}{
					{name: "減算前の日次バトル回数が3のとき、取得結果の日次バトル回数が2になる", count: 3, want: 2},
					{name: "減算前の日次バトル回数が1のとき、取得結果の日次バトル回数が0になる", count: 1, want: 0},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
						incrementDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1), tt.count)

						require.NoError(t, r.Players.DecrementDailyBattleCount(t.Context(), postgrestest.PlayerID1, gameDate(2026, time.April, 1)))

						got := getDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1))
						require.NotNil(t, got)
						assert.Equal(t, tt.want, got.DailyBattleCount)
					})
				}
			})
			t.Run("ゲーム日2026-04-01の日次バトル回数が3で、ゲーム日2026-04-02の日次バトル回数があるとき、ゲーム日2026-04-02を減算すると、ゲーム日2026-04-01の取得結果の日次バトル回数は3のままになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				incrementDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1), 3)
				incrementDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 2), 1)

				require.NoError(t, r.Players.DecrementDailyBattleCount(t.Context(), postgrestest.PlayerID1, gameDate(2026, time.April, 2)))

				got := getDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1))
				require.NotNil(t, got)
				assert.Equal(t, int64(3), got.DailyBattleCount)
			})
			t.Run("指定したプレイヤーの日次バトル回数があり、別のプレイヤーの日次バトル回数が3のとき、指定したプレイヤーを減算すると、別のプレイヤーの取得結果の日次バトル回数は3のままになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2))
				incrementDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1), 2)
				incrementDailyBattle(t, r, postgrestest.PlayerID2, gameDate(2026, time.April, 1), 3)

				require.NoError(t, r.Players.DecrementDailyBattleCount(t.Context(), postgrestest.PlayerID1, gameDate(2026, time.April, 1)))

				got := getDailyBattle(t, r, postgrestest.PlayerID2, gameDate(2026, time.April, 1))
				require.NotNil(t, got)
				assert.Equal(t, int64(3), got.DailyBattleCount)
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("ゲーム日2026-04-01の日次バトル回数が1件も無いとき、ゲーム日2026-04-01を減算して見つからないエラーになった後に", func(t *testing.T) {
				t.Run("ゲーム日2026-04-01を指定して取得すると、該当なしのままになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
					require.ErrorIs(t, r.Players.DecrementDailyBattleCount(t.Context(), postgrestest.PlayerID1, gameDate(2026, time.April, 1)), port.ErrNotFound)

					got := getDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 1))

					assert.Nil(t, got)
				})
				t.Run("ゲーム日2026-04-01を加算すると、加算結果が1になる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
					require.ErrorIs(t, r.Players.DecrementDailyBattleCount(t.Context(), postgrestest.PlayerID1, gameDate(2026, time.April, 1)), port.ErrNotFound)

					incremented, err := r.Players.IncrementDailyBattleCount(t.Context(), postgrestest.PlayerID1, gameDate(2026, time.April, 1))

					require.NoError(t, err)
					assert.Equal(t, int64(1), incremented)
				})
			})
			t.Run("ゲーム日2026-04-01を減算すると", func(t *testing.T) {
				tests := []struct {
					name    string
					prepare func(t *testing.T, r *postgrestest.Repositories)
				}{
					{
						name:    "日次バトル回数が1件も無いとき、見つからないエラーになる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {},
					},
					{
						name: "ゲーム日2026-04-02の日次バトル回数だけがあるとき、見つからないエラーになる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							incrementDailyBattle(t, r, postgrestest.PlayerID1, gameDate(2026, time.April, 2), 1)
						},
					},
					{
						name: "別のプレイヤーの日次バトル回数だけがあるとき、見つからないエラーになる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2))
							incrementDailyBattle(t, r, postgrestest.PlayerID2, gameDate(2026, time.April, 1), 1)
						},
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
						tt.prepare(t, r)

						err := r.Players.DecrementDailyBattleCount(t.Context(), postgrestest.PlayerID1, gameDate(2026, time.April, 1))

						assert.ErrorIs(t, err, port.ErrNotFound)
					})
				}
			})
		})
	})
}

func TestBattleCountReversalRecording(t *testing.T) {
	t.Run("[対戦]バトル回数返却の記録", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("記録すると", func(t *testing.T) {
				tests := []struct {
					name    string
					prepare func(t *testing.T, r *postgrestest.Repositories)
					gameID  string
				}{
					{
						name:    "記録が無い対戦ID「game-a」のとき、新規に記録されたという結果が返る",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {},
						gameID:  "game-a",
					},
					{
						name: "対戦ID「game-a」が記録済みで、記録する対戦IDが「game-b」のとき、新規に記録されたという結果が返る",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							_, err := r.Reversals.MarkReverted(t.Context(), "game-a")
							require.NoError(t, err)
						},
						gameID: "game-b",
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						tt.prepare(t, r)

						got, err := r.Reversals.MarkReverted(t.Context(), tt.gameID)

						require.NoError(t, err)
						assert.True(t, got)
					})
				}
			})
			t.Run("対戦ID「game-a」が記録済みのとき、同じ「game-a」を記録すると", func(t *testing.T) {
				recordAgain := func(t *testing.T) (*postgrestest.Repositories, bool) {
					t.Helper()
					r := postgrestest.NewRepositories(t, sharedPg)
					_, err := r.Reversals.MarkReverted(t.Context(), "game-a")
					require.NoError(t, err)

					got, err := r.Reversals.MarkReverted(t.Context(), "game-a")

					require.NoError(t, err)
					return r, got
				}
				t.Run("既に記録済みという結果が返る", func(t *testing.T) {
					_, got := recordAgain(t)

					assert.False(t, got)
				})
				t.Run("データベースに保存されたバトル回数返却の記録は1件のままになる", func(t *testing.T) {
					r, _ := recordAgain(t)

					assert.Equal(t, 1, r.CountBattleCountReversals(t))
				})
			})
			t.Run("対戦ID「game-a」が記録されていないとき、「game-a」を2回同時に記録すると", func(t *testing.T) {
				type markResult struct {
					recorded bool
					err      error
				}
				recordConcurrently := func(t *testing.T) (*postgrestest.Repositories, []markResult) {
					t.Helper()
					r := postgrestest.NewRepositories(t, sharedPg)

					results := postgrestest.RunConcurrently(2, func(int) markResult {
						recorded, err := r.Reversals.MarkReverted(t.Context(), "game-a")
						return markResult{recorded: recorded, err: err}
					})
					return r, results
				}
				t.Run("片方だけが新規に記録されたという結果になり、もう片方は既に記録済みという結果になる", func(t *testing.T) {
					_, results := recordConcurrently(t)

					recorded := make([]bool, 0, len(results))
					for _, result := range results {
						require.NoError(t, result.err)
						recorded = append(recorded, result.recorded)
					}
					assert.ElementsMatch(t, []bool{true, false}, recorded)
				})
				t.Run("データベースに保存されたバトル回数返却の記録は1件になる", func(t *testing.T) {
					r, results := recordConcurrently(t)

					for _, result := range results {
						require.NoError(t, result.err)
					}
					assert.Equal(t, 1, r.CountBattleCountReversals(t))
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("呼び出しが取り消し済みのとき、記録すると、取り消しを示すエラーになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				_, err := r.Reversals.MarkReverted(ctx, "game-a")

				assert.ErrorIs(t, err, context.Canceled)
			})
			t.Run("呼び出しが取り消し済みで対戦ID「game-a」の記録がエラーになった後に、取り消されていない呼び出しで対戦ID「game-a」を記録すると、新規に記録されたという結果が返る", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				_, err := r.Reversals.MarkReverted(ctx, "game-a")
				require.Error(t, err)

				got, err := r.Reversals.MarkReverted(t.Context(), "game-a")

				require.NoError(t, err)
				assert.True(t, got)
			})
		})
	})
}
