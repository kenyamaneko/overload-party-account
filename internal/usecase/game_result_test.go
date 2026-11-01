package usecase_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	"github.com/kenyamaneko/overload-party-account/internal/usecase/usecasetest"
)

type gameExpFixture struct {
	env       *usecasetest.UnitEnv
	player1ID string
	player2ID string
}

func newGameExpFixture(t *testing.T) *gameExpFixture {
	t.Helper()
	env := usecasetest.NewUnitEnv(t)
	return &gameExpFixture{
		env:       env,
		player1ID: usecasetest.RegisterPlayer(t, env.Auth),
		player2ID: usecasetest.RegisterPlayer(t, env.Auth),
	}
}

func newGameExpFixtureWithPlayer1Only(t *testing.T) *gameExpFixture {
	t.Helper()
	env := usecasetest.NewUnitEnv(t)
	return &gameExpFixture{env: env, player1ID: usecasetest.RegisterPlayer(t, env.Auth)}
}

func (f *gameExpFixture) award(t *testing.T, player2ID string, winner domain.WinnerNumber, reason domain.WinReason, matchType domain.MatchType) error {
	t.Helper()
	return f.env.Player.AwardGameExp(t.Context(), f.player1ID, player2ID, winner, reason, matchType)
}

func (f *gameExpFixture) storedExp(t *testing.T, playerID string) int64 {
	t.Helper()
	return usecasetest.ReadStoredExp(t, f.env, playerID)
}

func TestAwardGameExp(t *testing.T) {
	t.Run("[経験値]対戦結果による付与", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("勝利の経験値が100、敗北の経験値が30、引き分けの経験値が50に設定されているとき", func(t *testing.T) {
				t.Run("付与したあとにplayer1_idのプレイヤーの累計経験値を取得すると", func(t *testing.T) {
					registeredPlayer2 := func(f *gameExpFixture) string { return f.player2ID }
					noPlayer2 := func(*gameExpFixture) string { return "" }
					tests := []struct {
						name      string
						player2ID func(f *gameExpFixture) string
						winner    domain.WinnerNumber
						reason    domain.WinReason
						matchType domain.MatchType
						wantExp   int64
					}{
						{"対人戦で勝者番号が1のとき、累計経験値は100増える", registeredPlayer2, domain.WinnerNumberPlayer1, usecasetest.DecisiveWinReason, domain.MatchTypePvp, 100},
						{"対人戦で勝者番号が2のとき、累計経験値は30増える", registeredPlayer2, domain.WinnerNumberPlayer2, usecasetest.DecisiveWinReason, domain.MatchTypePvp, 30},
						{"対人戦で勝者番号が0で理由がdrawのとき、累計経験値は50増える", registeredPlayer2, domain.WinnerNumberNone, domain.WinReasonDraw, domain.MatchTypePvp, 50},
						{"対人戦で勝者番号が0で理由がdisconnectのとき、累計経験値は50増える", registeredPlayer2, domain.WinnerNumberNone, domain.WinReason("disconnect"), domain.MatchTypePvp, 50},
						{"NPC戦で勝者番号が1のとき、累計経験値は100増える", noPlayer2, domain.WinnerNumberPlayer1, usecasetest.DecisiveWinReason, domain.MatchTypeNpc, 100},
						{"NPC戦で勝者番号が2のとき、累計経験値は30増える", noPlayer2, domain.WinnerNumberPlayer2, usecasetest.DecisiveWinReason, domain.MatchTypeNpc, 30},
						{"NPC戦で勝者番号が0で理由がdrawのとき、累計経験値は50増える", noPlayer2, domain.WinnerNumberNone, domain.WinReasonDraw, domain.MatchTypeNpc, 50},
					}
					for _, tt := range tests {
						t.Run(tt.name, func(t *testing.T) {
							f := newGameExpFixture(t)
							require.NoError(t, f.award(t, tt.player2ID(f), tt.winner, tt.reason, tt.matchType))
							assert.Equal(t, tt.wantExp, f.storedExp(t, f.player1ID))
						})
					}
				})

				t.Run("対人戦で付与したあとにplayer2_idのプレイヤーの累計経験値を取得すると", func(t *testing.T) {
					tests := []struct {
						name    string
						winner  domain.WinnerNumber
						reason  domain.WinReason
						wantExp int64
					}{
						{"勝者番号が1のとき、累計経験値は30増える", domain.WinnerNumberPlayer1, usecasetest.DecisiveWinReason, 30},
						{"勝者番号が2のとき、累計経験値は100増える", domain.WinnerNumberPlayer2, usecasetest.DecisiveWinReason, 100},
						{"勝者番号が0で理由がdrawのとき、累計経験値は50増える", domain.WinnerNumberNone, domain.WinReasonDraw, 50},
						{"勝者番号が0で理由がdisconnectのとき、累計経験値は50増える", domain.WinnerNumberNone, domain.WinReason("disconnect"), 50},
					}
					for _, tt := range tests {
						t.Run(tt.name, func(t *testing.T) {
							f := newGameExpFixture(t)
							require.NoError(t, f.award(t, f.player2ID, tt.winner, tt.reason, domain.MatchTypePvp))
							assert.Equal(t, tt.wantExp, f.storedExp(t, f.player2ID))
						})
					}
				})
			})

			t.Run("敗北の経験値が0に設定されているとき、対人戦で勝者番号が1の付与をすると、付与したあとに取得したplayer2_idのプレイヤーの累計経験値は0のままになる", func(t *testing.T) {
				f := newGameExpFixture(t)
				f.env.Store.SetGameConfig("exp_loss", 0)
				require.NoError(t, f.award(t, f.player2ID, domain.WinnerNumberPlayer1, usecasetest.DecisiveWinReason, domain.MatchTypePvp))
				assert.Equal(t, int64(0), f.storedExp(t, f.player2ID))
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("NPC戦でplayer2_idに登録済みのプレイヤーのIDが指定されたとき、付与をしても、付与したあとに取得したplayer2_idのプレイヤーの累計経験値は0のままになる", func(t *testing.T) {
				f := newGameExpFixture(t)
				require.NoError(t, f.award(t, f.player2ID, domain.WinnerNumberPlayer2, usecasetest.DecisiveWinReason, domain.MatchTypeNpc))
				assert.Equal(t, int64(0), f.storedExp(t, f.player2ID))
			})

			t.Run("ゲーム設定が不足しているとき、付与すると", func(t *testing.T) {
				tests := []struct {
					name       string
					missingKey string
				}{
					{"勝利の経験値が存在しないとき、ゲーム設定が見つからないことを示すエラーになる", "exp_win"},
					{"敗北の経験値が存在しないとき、ゲーム設定が見つからないことを示すエラーになる", "exp_loss"},
					{"引き分けの経験値が存在しないとき、ゲーム設定が見つからないことを示すエラーになる", "exp_draw"},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						f := newGameExpFixture(t)
						f.env.Store.RemoveGameConfig(tt.missingKey)
						err := f.award(t, f.player2ID, domain.WinnerNumberPlayer1, usecasetest.DecisiveWinReason, domain.MatchTypePvp)
						assert.ErrorIs(t, err, usecase.ErrGameConfigNotFound)
					})
				}
			})

			t.Run("対戦結果が不正なとき、付与すると", func(t *testing.T) {
				tests := []struct {
					name      string
					winner    domain.WinnerNumber
					reason    domain.WinReason
					matchType domain.MatchType
					wantErr   error
				}{
					{"勝者番号が3のとき、勝者番号が不正であることを示すエラーになる", 3, usecasetest.DecisiveWinReason, domain.MatchTypePvp, usecase.ErrInvalidWinnerNumber},
					{"勝者番号が-1のとき、勝者番号が不正であることを示すエラーになる", -1, usecasetest.DecisiveWinReason, domain.MatchTypePvp, usecase.ErrInvalidWinnerNumber},
					{"勝者番号が1で理由がdrawのとき、勝者番号が不正であることを示すエラーになる", domain.WinnerNumberPlayer1, domain.WinReasonDraw, domain.MatchTypePvp, usecase.ErrInvalidWinnerNumber},
					{"対戦種別がpvpでもnpcでもないarenaのとき、対戦種別が不正であることを示すエラーになる", domain.WinnerNumberPlayer1, usecasetest.DecisiveWinReason, domain.MatchType("arena"), usecase.ErrInvalidMatchType},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						f := newGameExpFixture(t)
						err := f.award(t, f.player2ID, tt.winner, tt.reason, tt.matchType)
						assert.ErrorIs(t, err, tt.wantErr)
					})
				}
			})

			t.Run("対戦結果が不正で付与に失敗したあと", func(t *testing.T) {
				targets := []struct {
					name     string
					playerID func(f *gameExpFixture) string
				}{
					{"player1_idのプレイヤーの累計経験値を取得すると", func(f *gameExpFixture) string { return f.player1ID }},
					{"player2_idのプレイヤーの累計経験値を取得すると", func(f *gameExpFixture) string { return f.player2ID }},
				}
				tests := []struct {
					name      string
					winner    domain.WinnerNumber
					reason    domain.WinReason
					matchType domain.MatchType
				}{
					{"勝者番号が3のとき、累計経験値は0のままになる", 3, usecasetest.DecisiveWinReason, domain.MatchTypePvp},
					{"勝者番号が-1のとき、累計経験値は0のままになる", -1, usecasetest.DecisiveWinReason, domain.MatchTypePvp},
					{"勝者番号が1で理由がdrawのとき、累計経験値は0のままになる", domain.WinnerNumberPlayer1, domain.WinReasonDraw, domain.MatchTypePvp},
					{"対戦種別がpvpでもnpcでもないarenaのとき、累計経験値は0のままになる", domain.WinnerNumberPlayer1, usecasetest.DecisiveWinReason, domain.MatchType("arena")},
				}
				for _, target := range targets {
					t.Run(target.name, func(t *testing.T) {
						for _, tt := range tests {
							t.Run(tt.name, func(t *testing.T) {
								f := newGameExpFixture(t)
								require.Error(t, f.award(t, f.player2ID, tt.winner, tt.reason, tt.matchType))
								assert.Equal(t, int64(0), f.storedExp(t, target.playerID(f)))
							})
						}
					})
				}
			})

			t.Run("対人戦で2人目のプレイヤーIDが空のとき", func(t *testing.T) {
				t.Run("付与すると、2人目のプレイヤーIDが指定されていないことを示すエラーになる", func(t *testing.T) {
					f := newGameExpFixtureWithPlayer1Only(t)
					err := f.award(t, "", domain.WinnerNumberPlayer1, usecasetest.DecisiveWinReason, domain.MatchTypePvp)
					assert.ErrorIs(t, err, usecase.ErrPlayer2IDRequired)
				})

				t.Run("付与に失敗したあとに取得したplayer1_idのプレイヤーの累計経験値は0のままになる", func(t *testing.T) {
					f := newGameExpFixtureWithPlayer1Only(t)
					require.Error(t, f.award(t, "", domain.WinnerNumberPlayer1, usecasetest.DecisiveWinReason, domain.MatchTypePvp))
					assert.Equal(t, int64(0), f.storedExp(t, f.player1ID))
				})
			})
		})
	})
}
