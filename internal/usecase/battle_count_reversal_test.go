package usecase_test

import (
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/logging/loggingtest"
	"github.com/kenyamaneko/overload-party-account/internal/port/porttest"
	"github.com/kenyamaneko/overload-party-account/internal/usecase/usecasetest"
)

var (
	gameDateJan9  = civil.Date{Year: 2026, Month: time.January, Day: 9}
	gameDateJan10 = civil.Date{Year: 2026, Month: time.January, Day: 10}
	gameDateJan11 = civil.Date{Year: 2026, Month: time.January, Day: 11}
	gameDateJan12 = civil.Date{Year: 2026, Month: time.January, Day: 12}
)

var defaultConsumedAt = time.Date(2026, 1, 10, 12, 0, 0, 0, usecasetest.JapanTimeZone)

const (
	revertPlayer1ID = "3c8e1f6a-5b2d-4a7c-8e19-6d0f4b2a7c31"
	revertPlayer2ID = "9a4d7e20-1c6b-4f58-b3a2-0e5c8d1f6a94"
)

type revertFixture struct {
	env       *usecasetest.UnitEnv
	player1ID string
	player2ID string
}

func newRevertFixture(t *testing.T) *revertFixture {
	t.Helper()
	env := usecasetest.NewUnitEnv(t)
	env.Store.AddPlayer(porttest.NewPlayerSeed(revertPlayer1ID, "firebase-uid-1", defaultConsumedAt))
	env.Store.AddPlayer(porttest.NewPlayerSeed(revertPlayer2ID, "firebase-uid-2", defaultConsumedAt))
	for _, gameDate := range []civil.Date{gameDateJan9, gameDateJan10, gameDateJan11} {
		env.Store.SetDailyBattleCount(revertPlayer1ID, gameDate, 3)
		env.Store.SetDailyBattleCount(revertPlayer2ID, gameDate, 2)
	}
	return &revertFixture{env: env, player1ID: revertPlayer1ID, player2ID: revertPlayer2ID}
}

func (f *revertFixture) revert(t *testing.T, gameID string, consumedAt time.Time) error {
	t.Helper()
	return f.env.Player.RevertBattleCount(t.Context(), gameID, consumedAt, f.player1ID, f.player2ID)
}

func (f *revertFixture) revertConsumedAt(t *testing.T, consumedAt string) error {
	t.Helper()
	parsed, err := time.Parse(time.RFC3339Nano, consumedAt)
	require.NoError(t, err)
	return f.revert(t, "game-1", parsed)
}

func (f *revertFixture) storedCount(t *testing.T, playerID string, gameDate civil.Date) int64 {
	t.Helper()
	record, err := f.env.Store.PlayerBattleRepo().GetDailyBattle(t.Context(), playerID, gameDate)
	require.NoError(t, err)
	require.NotNil(t, record)
	return record.DailyBattleCount
}

func TestRevertBattleCount(t *testing.T) {
	t.Run("[消費バトル回数返却]停止で無効になった対戦の消費バトル回数の返却", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("停止で無効になった対戦で2026-01-10T12:00:00+09:00に消費したバトル回数を返却するとき", func(t *testing.T) {
				t.Run("player2_idのプレイヤーの2026-01-10のバトル回数が2回のとき、返却したあとに取得した2026-01-10のバトル回数は1回になる", func(t *testing.T) {
					f := newRevertFixture(t)
					require.NoError(t, f.revert(t, "game-1", defaultConsumedAt))
					assert.Equal(t, int64(1), f.storedCount(t, f.player2ID, gameDateJan10))
				})

				t.Run("player1_idのプレイヤーの2026-01-10のバトル回数が1回のとき、返却したあとに取得した2026-01-10のバトル回数は0回になる", func(t *testing.T) {
					f := newRevertFixture(t)
					f.env.Store.SetDailyBattleCount(f.player1ID, gameDateJan10, 1)
					require.NoError(t, f.revert(t, "game-1", defaultConsumedAt))
					assert.Equal(t, int64(0), f.storedCount(t, f.player1ID, gameDateJan10))
				})

				t.Run("player1_idとplayer2_idの両プレイヤーに2026-01-10のバトル記録があるとき、返却すると警告ログは出ない", func(t *testing.T) {
					f := newRevertFixture(t)
					logs := loggingtest.Capture(t)
					require.NoError(t, f.revert(t, "game-1", defaultConsumedAt))
					assert.Empty(t, usecasetest.SelectWarnings(logs))
				})

				t.Run("返却したあとにplayer1_idのプレイヤーのバトル回数を取得すると", func(t *testing.T) {
					tests := []struct {
						name     string
						gameDate civil.Date
					}{
						{"2026-01-09のバトル回数は3回のままになる", gameDateJan9},
						{"2026-01-11のバトル回数は3回のままになる", gameDateJan11},
					}
					for _, tt := range tests {
						t.Run(tt.name, func(t *testing.T) {
							f := newRevertFixture(t)
							require.NoError(t, f.revert(t, "game-1", defaultConsumedAt))
							assert.Equal(t, int64(3), f.storedCount(t, f.player1ID, tt.gameDate))
						})
					}
				})

				t.Run("返却したあとにplayer2_idのプレイヤーのバトル回数を取得すると", func(t *testing.T) {
					tests := []struct {
						name     string
						gameDate civil.Date
					}{
						{"2026-01-09のバトル回数は2回のままになる", gameDateJan9},
						{"2026-01-11のバトル回数は2回のままになる", gameDateJan11},
					}
					for _, tt := range tests {
						t.Run(tt.name, func(t *testing.T) {
							f := newRevertFixture(t)
							require.NoError(t, f.revert(t, "game-1", defaultConsumedAt))
							assert.Equal(t, int64(2), f.storedCount(t, f.player2ID, tt.gameDate))
						})
					}
				})
			})

			t.Run("player1_idのプレイヤーのバトル回数が3回ある日のバトル回数を返却したあとに取得すると", func(t *testing.T) {
				tests := []struct {
					name       string
					consumedAt string
					gameDate   civil.Date
				}{
					{"消費時刻が2026-01-10T05:00:00+09:00のとき、2026-01-10のバトル回数は2回になる", "2026-01-10T05:00:00+09:00", gameDateJan10},
					{"消費時刻が2026-01-11T04:59:59.999999999+09:00のとき、2026-01-10のバトル回数は2回になる", "2026-01-11T04:59:59.999999999+09:00", gameDateJan10},
					{"消費時刻が2026-01-11T05:00:00+09:00のとき、2026-01-11のバトル回数は2回になる", "2026-01-11T05:00:00+09:00", gameDateJan11},
					{"消費時刻が2026-01-10T04:59:59.999999999+09:00のとき、2026-01-09のバトル回数は2回になる", "2026-01-10T04:59:59.999999999+09:00", gameDateJan9},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						f := newRevertFixture(t)
						require.NoError(t, f.revertConsumedAt(t, tt.consumedAt))
						assert.Equal(t, int64(2), f.storedCount(t, f.player1ID, tt.gameDate))
					})
				}
			})

			t.Run("停止で無効になった対戦のバトル回数の返却を繰り返すとき", func(t *testing.T) {
				t.Run("game_idがgame-1の返却を2回行うと、2回目の返却は成功する", func(t *testing.T) {
					f := newRevertFixture(t)
					require.NoError(t, f.revert(t, "game-1", defaultConsumedAt))
					assert.NoError(t, f.revert(t, "game-1", defaultConsumedAt))
				})

				t.Run("player1_idのプレイヤーのバトル回数が3回のとき", func(t *testing.T) {
					t.Run("game_idがgame-1の返却を2回行っても、取得した2026-01-10のバトル回数は2回になる", func(t *testing.T) {
						f := newRevertFixture(t)
						require.NoError(t, f.revert(t, "game-1", defaultConsumedAt))
						require.NoError(t, f.revert(t, "game-1", defaultConsumedAt))
						assert.Equal(t, int64(2), f.storedCount(t, f.player1ID, gameDateJan10))
					})

					t.Run("game_idがgame-1の返却とgame-2の返却を1回ずつ行うと、取得した2026-01-10のバトル回数は1回になる", func(t *testing.T) {
						f := newRevertFixture(t)
						require.NoError(t, f.revert(t, "game-1", defaultConsumedAt))
						require.NoError(t, f.revert(t, "game-2", defaultConsumedAt))
						assert.Equal(t, int64(1), f.storedCount(t, f.player1ID, gameDateJan10))
					})
				})
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("停止で無効になった対戦のバトル回数を返却するとき", func(t *testing.T) {
				t.Run("消費時刻が2026-01-10T12:00:00+09:00で、player1_idのプレイヤーの2026-01-10のバトル回数が0回のとき", func(t *testing.T) {
					t.Run("返却したあとに取得した2026-01-10のバトル回数は0回のままになる", func(t *testing.T) {
						f := newRevertFixture(t)
						f.env.Store.SetDailyBattleCount(f.player1ID, gameDateJan10, 0)
						require.NoError(t, f.revert(t, "game-1", defaultConsumedAt))
						assert.Equal(t, int64(0), f.storedCount(t, f.player1ID, gameDateJan10))
					})

					t.Run("返却すると警告ログは出ない", func(t *testing.T) {
						f := newRevertFixture(t)
						f.env.Store.SetDailyBattleCount(f.player1ID, gameDateJan10, 0)
						logs := loggingtest.Capture(t)
						require.NoError(t, f.revert(t, "game-1", defaultConsumedAt))
						assert.Empty(t, usecasetest.SelectWarnings(logs))
					})
				})

				t.Run("消費時刻が2026-01-10T12:00:00+09:00で、player2_idのプレイヤーの2026-01-10のバトル記録が無いとき", func(t *testing.T) {
					t.Run("player1_idのプレイヤーの2026-01-10のバトル回数が3回のとき、返却したあとに取得したplayer1_idのプレイヤーの2026-01-10のバトル回数は2回になる", func(t *testing.T) {
						f := newRevertFixture(t)
						f.env.Store.RemoveDailyBattle(f.player2ID, gameDateJan10)
						require.NoError(t, f.revert(t, "game-1", defaultConsumedAt))
						assert.Equal(t, int64(2), f.storedCount(t, f.player1ID, gameDateJan10))
					})

					t.Run("返却すると警告ログが1件出る", func(t *testing.T) {
						f := newRevertFixture(t)
						f.env.Store.RemoveDailyBattle(f.player2ID, gameDateJan10)
						logs := loggingtest.Capture(t)
						require.NoError(t, f.revert(t, "game-1", defaultConsumedAt))
						assert.Len(t, usecasetest.SelectWarnings(logs), 1)
					})

					t.Run("player2_idが9a4d7e20-1c6b-4f58-b3a2-0e5c8d1f6a94のとき、返却すると警告ログのplayer_idは9a4d7e20-1c6b-4f58-b3a2-0e5c8d1f6a94になる", func(t *testing.T) {
						f := newRevertFixture(t)
						f.env.Store.RemoveDailyBattle(f.player2ID, gameDateJan10)
						logs := loggingtest.Capture(t)
						require.NoError(t, f.revert(t, "game-1", defaultConsumedAt))
						assert.Equal(t, []string{"9a4d7e20-1c6b-4f58-b3a2-0e5c8d1f6a94"}, usecasetest.SelectWarningValues(logs, "player_id"))
					})

					t.Run("game_idがgame-1のとき、返却すると警告ログのgame_idはgame-1になる", func(t *testing.T) {
						f := newRevertFixture(t)
						f.env.Store.RemoveDailyBattle(f.player2ID, gameDateJan10)
						logs := loggingtest.Capture(t)
						require.NoError(t, f.revert(t, "game-1", defaultConsumedAt))
						assert.Equal(t, []string{"game-1"}, usecasetest.SelectWarningValues(logs, "game_id"))
					})
				})

				t.Run("消費時刻が2026-01-12T12:00:00+09:00で、player1_idのプレイヤーの2026-01-12のバトル回数が3回、player2_idのプレイヤーの2026-01-12のバトル記録が無いとき、返却すると警告ログのgame_dateは2026-01-12になる", func(t *testing.T) {
					f := newRevertFixture(t)
					f.env.Store.SetDailyBattleCount(f.player1ID, gameDateJan12, 3)
					logs := loggingtest.Capture(t)
					require.NoError(t, f.revertConsumedAt(t, "2026-01-12T12:00:00+09:00"))
					assert.Equal(t, []string{"2026-01-12"}, usecasetest.SelectWarningValues(logs, "game_date"))
				})
			})
		})
	})
}
