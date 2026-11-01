//go:build integration

package usecase_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres/postgrestest"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	"github.com/kenyamaneko/overload-party-account/internal/usecase/usecasetest"
)

func countSucceeded(errs []error) int {
	return len(slices.DeleteFunc(slices.Clone(errs), func(err error) bool { return err != nil }))
}

func countFailedWith(errs []error, target error) int {
	return len(slices.DeleteFunc(slices.Clone(errs), func(err error) bool { return !errors.Is(err, target) }))
}

func groupFactionsBySucceeded(factions []string, errs []error) map[bool][]string {
	grouped := map[bool][]string{}
	for index, err := range errs {
		isSucceeded := err == nil
		grouped[isSucceeded] = append(grouped[isSucceeded], factions[index])
	}
	return grouped
}

func incrementBattleCountConcurrently(t *testing.T, env *usecasetest.IntegrationEnv, playerID string, count int) []error {
	t.Helper()
	return postgrestest.RunConcurrently(count, func(int) error {
		return env.Player.IncrementBattleCount(t.Context(), playerID)
	})
}

func selectInitialFactionsConcurrently(t *testing.T, env *usecasetest.IntegrationEnv, playerID string, factions []string) []error {
	t.Helper()
	return postgrestest.RunConcurrently(len(factions), func(index int) error {
		return env.Faction.SelectInitialFaction(t.Context(), playerID, factions[index])
	})
}

func newCancelledContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	return ctx
}

func newEnvWithPlayer1BattleCountOfOne(t *testing.T) (env *usecasetest.IntegrationEnv, player1ID, player2ID string) {
	t.Helper()
	env = usecasetest.NewIntegrationEnv(t, sharedPg)
	player1ID = usecasetest.RegisterPlayer(t, env.Auth)
	player2ID = usecasetest.RegisterPlayer(t, env.Auth)
	require.NoError(t, env.Player.IncrementBattleCount(t.Context(), player1ID))
	return env, player1ID, player2ID
}

func revertBattleCountConcurrently(ctx context.Context, env *usecasetest.IntegrationEnv, player1ID, player2ID string) []error {
	consumedAt := time.Date(2026, 1, 10, 12, 0, 0, 0, usecasetest.JapanTimeZone)
	return postgrestest.RunConcurrently(5, func(index int) error {
		return env.Player.RevertBattleCount(ctx, fmt.Sprintf("game-%d", index), consumedAt, player1ID, player2ID)
	})
}

func awardExpConcurrently(ctx context.Context, env *usecasetest.IntegrationEnv, playerID string) []error {
	return postgrestest.RunConcurrently(20, func(int) error {
		return env.Player.AwardExp(ctx, playerID, 10)
	})
}

func TestIncrementBattleCountConcurrently(t *testing.T) {
	t.Run("[バトル回数制限]1日のバトル回数の同時加算", func(t *testing.T) {
		t.Run("異常系", func(t *testing.T) {
			t.Run("無料プレイヤーの設定した上限が3回で当日のバトル回数が0回のとき、加算を5件同時に行うと", func(t *testing.T) {
				t.Run("成功は3件になる", func(t *testing.T) {
					env := usecasetest.NewIntegrationEnv(t, sharedPg)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					errs := incrementBattleCountConcurrently(t, env, playerID, 5)
					assert.Equal(t, 3, countSucceeded(errs))
				})

				t.Run("1日のバトル回数が上限に達していることを示すエラーは2件になる", func(t *testing.T) {
					env := usecasetest.NewIntegrationEnv(t, sharedPg)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					errs := incrementBattleCountConcurrently(t, env, playerID, 5)
					assert.Equal(t, 2, countFailedWith(errs, usecase.ErrBattleLimitExceeded))
				})

				t.Run("照会したdaily_battle_countは3になる", func(t *testing.T) {
					env := usecasetest.NewIntegrationEnv(t, sharedPg)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					incrementBattleCountConcurrently(t, env, playerID, 5)
					limit, err := env.Player.GetBattleLimit(t.Context(), playerID)
					require.NoError(t, err)
					assert.Equal(t, int64(3), limit.DailyBattleCount)
				})

				t.Run("上限に達して加算に失敗したあと、バトル回数を1回返却してから加算すると、照会したdaily_battle_countは3になる", func(t *testing.T) {
					env := usecasetest.NewIntegrationEnv(t, sharedPg)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					otherPlayerID := usecasetest.RegisterPlayer(t, env.Auth)
					incrementBattleCountConcurrently(t, env, playerID, 5)
					consumedAt := time.Date(2026, 1, 10, 12, 0, 0, 0, usecasetest.JapanTimeZone)
					require.NoError(t, env.Player.RevertBattleCount(t.Context(), "game-1", consumedAt, playerID, otherPlayerID))
					require.NoError(t, env.Player.IncrementBattleCount(t.Context(), playerID))
					limit, err := env.Player.GetBattleLimit(t.Context(), playerID)
					require.NoError(t, err)
					assert.Equal(t, int64(3), limit.DailyBattleCount)
				})
			})
		})
	})
}

func TestRevertBattleCountConcurrently(t *testing.T) {
	t.Run("[消費バトル回数返却]停止で無効になった対戦の消費バトル回数の同時返却", func(t *testing.T) {
		t.Run("player1_idのプレイヤーの2026-01-10のバトル回数が1回のとき", func(t *testing.T) {
			t.Run("正常系", func(t *testing.T) {
				t.Run("異なるgame_idの返却を5件同時に行うと、照会したdaily_battle_countは0になる", func(t *testing.T) {
					env, player1ID, player2ID := newEnvWithPlayer1BattleCountOfOne(t)

					errs := revertBattleCountConcurrently(t.Context(), env, player1ID, player2ID)

					require.NoError(t, errors.Join(errs...))
					limit, err := env.Player.GetBattleLimit(t.Context(), player1ID)
					require.NoError(t, err)
					assert.Equal(t, int64(0), limit.DailyBattleCount)
				})
			})

			t.Run("異常系", func(t *testing.T) {
				t.Run("呼び出しが取り消し済みで、異なるgame_idの返却を5件同時に行うと", func(t *testing.T) {
					t.Run("取り消しを示すエラーは5件になる", func(t *testing.T) {
						env, player1ID, player2ID := newEnvWithPlayer1BattleCountOfOne(t)

						errs := revertBattleCountConcurrently(newCancelledContext(t), env, player1ID, player2ID)

						assert.Equal(t, 5, countFailedWith(errs, context.Canceled))
					})

					t.Run("照会したdaily_battle_countは1のままになる", func(t *testing.T) {
						env, player1ID, player2ID := newEnvWithPlayer1BattleCountOfOne(t)
						revertBattleCountConcurrently(newCancelledContext(t), env, player1ID, player2ID)

						limit, err := env.Player.GetBattleLimit(t.Context(), player1ID)

						require.NoError(t, err)
						assert.Equal(t, int64(1), limit.DailyBattleCount)
					})

					t.Run("返却に失敗したあと、取り消されていない呼び出しで、失敗したときと同じ5つのgame_idの返却を同時に行い直すと、照会したdaily_battle_countは0になる", func(t *testing.T) {
						env, player1ID, player2ID := newEnvWithPlayer1BattleCountOfOne(t)
						failedErrs := revertBattleCountConcurrently(newCancelledContext(t), env, player1ID, player2ID)
						require.Equal(t, 5, countFailedWith(failedErrs, context.Canceled))

						errs := revertBattleCountConcurrently(t.Context(), env, player1ID, player2ID)

						require.NoError(t, errors.Join(errs...))
						limit, err := env.Player.GetBattleLimit(t.Context(), player1ID)
						require.NoError(t, err)
						assert.Equal(t, int64(0), limit.DailyBattleCount)
					})
				})
			})
		})
	})
}

func TestAwardExpConcurrently(t *testing.T) {
	t.Run("[経験値]累計経験値の同時加算", func(t *testing.T) {
		t.Run("登録済みで累計経験値が0のプレイヤーのとき", func(t *testing.T) {
			t.Run("正常系", func(t *testing.T) {
				t.Run("経験値10の加算を20件同時に行うと、続けて取得したプレイヤー情報の累計経験値は200になる", func(t *testing.T) {
					env := usecasetest.NewIntegrationEnv(t, sharedPg)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)

					errs := awardExpConcurrently(t.Context(), env, playerID)

					require.NoError(t, errors.Join(errs...))
					info, err := env.Player.GetPlayer(t.Context(), playerID)
					require.NoError(t, err)
					assert.Equal(t, int64(200), info.View.Exp)
				})
			})

			t.Run("異常系", func(t *testing.T) {
				t.Run("呼び出しが取り消し済みで、経験値10の加算を20件同時に行うと", func(t *testing.T) {
					t.Run("取り消しを示すエラーは20件になる", func(t *testing.T) {
						env := usecasetest.NewIntegrationEnv(t, sharedPg)
						playerID := usecasetest.RegisterPlayer(t, env.Auth)

						errs := awardExpConcurrently(newCancelledContext(t), env, playerID)

						assert.Equal(t, 20, countFailedWith(errs, context.Canceled))
					})

					t.Run("続けて取得したプレイヤー情報の累計経験値は0のままになる", func(t *testing.T) {
						env := usecasetest.NewIntegrationEnv(t, sharedPg)
						playerID := usecasetest.RegisterPlayer(t, env.Auth)
						awardExpConcurrently(newCancelledContext(t), env, playerID)

						info, err := env.Player.GetPlayer(t.Context(), playerID)

						require.NoError(t, err)
						assert.Equal(t, int64(0), info.View.Exp)
					})

					t.Run("加算に失敗したあと、取り消されていない呼び出しで経験値10の加算を20件同時に行うと、続けて取得したプレイヤー情報の累計経験値は200になる", func(t *testing.T) {
						env := usecasetest.NewIntegrationEnv(t, sharedPg)
						playerID := usecasetest.RegisterPlayer(t, env.Auth)
						awardExpConcurrently(newCancelledContext(t), env, playerID)

						errs := awardExpConcurrently(t.Context(), env, playerID)

						require.NoError(t, errors.Join(errs...))
						info, err := env.Player.GetPlayer(t.Context(), playerID)
						require.NoError(t, err)
						assert.Equal(t, int64(200), info.View.Exp)
					})
				})
			})
		})
	})
}

func TestAwardGameExpForTwoPlayers(t *testing.T) {
	t.Run("[経験値]対戦結果による2人分の付与", func(t *testing.T) {
		t.Run("異常系", func(t *testing.T) {
			t.Run("対人戦で勝者番号が1、player2_idのプレイヤーが未登録のとき", func(t *testing.T) {
				t.Run("付与すると、プレイヤーが存在しないことを示すエラーになる", func(t *testing.T) {
					env := usecasetest.NewIntegrationEnv(t, sharedPg)
					player1ID := usecasetest.RegisterPlayer(t, env.Auth)
					err := env.Player.AwardGameExp(t.Context(), player1ID, uuid.NewString(), domain.WinnerNumberPlayer1, usecasetest.DecisiveWinReason, domain.MatchTypePvp)
					assert.ErrorIs(t, err, port.ErrNotFound)
				})

				t.Run("付与に失敗したあとにプレイヤー情報を取得すると、player1_idのプレイヤーの累計経験値は0のままになる", func(t *testing.T) {
					env := usecasetest.NewIntegrationEnv(t, sharedPg)
					player1ID := usecasetest.RegisterPlayer(t, env.Auth)
					err := env.Player.AwardGameExp(t.Context(), player1ID, uuid.NewString(), domain.WinnerNumberPlayer1, usecasetest.DecisiveWinReason, domain.MatchTypePvp)
					require.ErrorIs(t, err, port.ErrNotFound)
					info, err := env.Player.GetPlayer(t.Context(), player1ID)
					require.NoError(t, err)
					assert.Equal(t, int64(0), info.View.Exp)
				})

				t.Run("勝利の経験値が100に設定されているとき、付与に失敗したあと、player2_idを登録済みのプレイヤーのIDにして付与し直したあとにプレイヤー情報を取得すると、player1_idのプレイヤーの累計経験値は100増える", func(t *testing.T) {
					env := usecasetest.NewIntegrationEnv(t, sharedPg)
					player1ID := usecasetest.RegisterPlayer(t, env.Auth)
					player2ID := usecasetest.RegisterPlayer(t, env.Auth)
					err := env.Player.AwardGameExp(t.Context(), player1ID, uuid.NewString(), domain.WinnerNumberPlayer1, usecasetest.DecisiveWinReason, domain.MatchTypePvp)
					require.ErrorIs(t, err, port.ErrNotFound)
					require.NoError(t, env.Player.AwardGameExp(t.Context(), player1ID, player2ID, domain.WinnerNumberPlayer1, usecasetest.DecisiveWinReason, domain.MatchTypePvp))
					info, err := env.Player.GetPlayer(t.Context(), player1ID)
					require.NoError(t, err)
					assert.Equal(t, int64(100), info.View.Exp)
				})
			})
		})
	})
}

func TestSelectInitialFactionConcurrently(t *testing.T) {
	t.Run("[初期陣営]初期陣営の同時選択", func(t *testing.T) {
		t.Run("異常系", func(t *testing.T) {
			t.Run("所持陣営が無い登録済みのプレイヤーが、2件の陣営の選択を同時に行うと", func(t *testing.T) {
				t.Run("成功した件数を数えると", func(t *testing.T) {
					tests := []struct {
						name     string
						factions []string
					}{
						{name: "選ぶ陣営がTenkiとSugarのとき、1件になる", factions: []string{"Tenki", "Sugar"}},
						{name: "選ぶ陣営がTenkiの2件のとき、1件になる", factions: []string{"Tenki", "Tenki"}},
					}
					for _, tt := range tests {
						t.Run(tt.name, func(t *testing.T) {
							env := usecasetest.NewIntegrationEnv(t, sharedPg)
							playerID := usecasetest.RegisterPlayer(t, env.Auth)

							errs := selectInitialFactionsConcurrently(t, env, playerID, tt.factions)

							assert.Equal(t, 1, countSucceeded(errs))
						})
					}
				})

				t.Run("選択済みを示すエラーの件数を数えると", func(t *testing.T) {
					tests := []struct {
						name     string
						factions []string
					}{
						{name: "選ぶ陣営がTenkiとSugarのとき、1件になる", factions: []string{"Tenki", "Sugar"}},
						{name: "選ぶ陣営がTenkiの2件のとき、1件になる", factions: []string{"Tenki", "Tenki"}},
					}
					for _, tt := range tests {
						t.Run(tt.name, func(t *testing.T) {
							env := usecasetest.NewIntegrationEnv(t, sharedPg)
							playerID := usecasetest.RegisterPlayer(t, env.Auth)

							errs := selectInitialFactionsConcurrently(t, env, playerID, tt.factions)

							assert.Equal(t, 1, countFailedWith(errs, usecase.ErrFactionAlreadySelected))
						})
					}
				})

				t.Run("選択したあとに所持陣営一覧を取得すると", func(t *testing.T) {
					tests := []struct {
						name     string
						factions []string
					}{
						{name: "選ぶ陣営がTenkiとSugarのとき、成功した側の陣営だけになる", factions: []string{"Tenki", "Sugar"}},
						{name: "選ぶ陣営がTenkiの2件のとき、Tenkiだけになる", factions: []string{"Tenki", "Tenki"}},
					}
					for _, tt := range tests {
						t.Run(tt.name, func(t *testing.T) {
							env := usecasetest.NewIntegrationEnv(t, sharedPg)
							playerID := usecasetest.RegisterPlayer(t, env.Auth)
							errs := selectInitialFactionsConcurrently(t, env, playerID, tt.factions)
							succeeded := groupFactionsBySucceeded(tt.factions, errs)[true]
							require.Len(t, succeeded, 1)

							listed, err := env.Faction.ListFactions(t.Context(), playerID)

							require.NoError(t, err)
							assert.Equal(t, succeeded, listed)
						})
					}
				})

				t.Run("選ぶ陣営がTenkiとSugarのとき、選択に失敗した側の陣営を付与すると、取得した所持陣営一覧はTenkiとSugarの2件になる", func(t *testing.T) {
					factions := []string{"Tenki", "Sugar"}
					env := usecasetest.NewIntegrationEnv(t, sharedPg)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					errs := selectInitialFactionsConcurrently(t, env, playerID, factions)
					failed := groupFactionsBySucceeded(factions, errs)[false]
					require.Len(t, failed, 1)
					require.NoError(t, env.Faction.GrantFaction(t.Context(), playerID, failed[0]))
					listed, err := env.Faction.ListFactions(t.Context(), playerID)
					require.NoError(t, err)
					assert.ElementsMatch(t, []string{"Tenki", "Sugar"}, listed)
				})

				t.Run("選ぶ陣営がTenkiの2件のとき、プレイヤー情報を取得すると、initial_factionはTenkiになる", func(t *testing.T) {
					env := usecasetest.NewIntegrationEnv(t, sharedPg)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					selectInitialFactionsConcurrently(t, env, playerID, []string{"Tenki", "Tenki"})
					info, err := env.Player.GetPlayer(t.Context(), playerID)
					require.NoError(t, err)
					assert.Equal(t, usecasetest.ToPointer("Tenki"), info.View.InitialFaction)
				})
			})
		})
	})
}
