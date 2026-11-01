package usecase_test

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	"github.com/kenyamaneko/overload-party-account/internal/usecase/usecasetest"
)

func selectInitialFactionAndGetInitialFaction(t *testing.T, faction string) *string {
	t.Helper()
	env := usecasetest.NewUnitEnv(t)
	playerID := usecasetest.RegisterPlayer(t, env.Auth)
	require.NoError(t, env.Faction.SelectInitialFaction(t.Context(), playerID, faction))
	return playerInfoOf(t, env, playerID).View.InitialFaction
}

func TestSelectInitialFactionValidation(t *testing.T) {
	t.Run("[初期陣営]選択できる陣営の判定", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("所持陣営が無い登録済みのプレイヤーが選択するとき", func(t *testing.T) {
				t.Run("陣営がSHEのとき、選択したあとに取得したプレイヤー情報のinitial_factionはSHEになる", func(t *testing.T) {
					assert.Equal(t, usecasetest.ToPointer("SHE"), selectInitialFactionAndGetInitialFaction(t, "SHE"))
				})

				t.Run("陣営がTenkiのとき、選択したあとに取得したプレイヤー情報のinitial_factionはTenkiになる", func(t *testing.T) {
					assert.Equal(t, usecasetest.ToPointer("Tenki"), selectInitialFactionAndGetInitialFaction(t, "Tenki"))
				})

				t.Run("陣営がSugarのとき、選択したあとに取得したプレイヤー情報のinitial_factionはSugarになる", func(t *testing.T) {
					assert.Equal(t, usecasetest.ToPointer("Sugar"), selectInitialFactionAndGetInitialFaction(t, "Sugar"))
				})

				t.Run("陣営がTunersのとき、選択したあとに取得したプレイヤー情報のinitial_factionはTunersになる", func(t *testing.T) {
					assert.Equal(t, usecasetest.ToPointer("Tuners"), selectInitialFactionAndGetInitialFaction(t, "Tuners"))
				})
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("所持陣営が無い登録済みのプレイヤーが選択するとき", func(t *testing.T) {
				tests := []struct {
					name    string
					faction string
				}{
					{"陣営がNeutralのとき、不正な陣営を示すエラーになる", "Neutral"},
					{"陣営がUnknownのとき、不正な陣営を示すエラーになる", "Unknown"},
					{"陣営が小文字のsheのとき、不正な陣営を示すエラーになる", "she"},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						env := usecasetest.NewUnitEnv(t)
						playerID := usecasetest.RegisterPlayer(t, env.Auth)
						err := env.Faction.SelectInitialFaction(t.Context(), playerID, tt.faction)
						assert.ErrorIs(t, err, usecase.ErrInvalidFaction)
					})
				}

				t.Run("陣営がNeutralのとき、選択に失敗したあとにプレイヤー情報を取得すると、initial_factionは含まれない", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					require.Error(t, env.Faction.SelectInitialFaction(t.Context(), playerID, "Neutral"))
					assert.Nil(t, playerInfoOf(t, env, playerID).View.InitialFaction)
				})
			})
		})
	})
}

func TestSelectInitialFaction(t *testing.T) {
	t.Run("[初期陣営]初期陣営の選択", func(t *testing.T) {
		t.Run("異常系", func(t *testing.T) {
			t.Run("初期陣営にSHEを選択済みのプレイヤーが選択するとき", func(t *testing.T) {
				tests := []struct {
					name    string
					faction string
				}{
					{"陣営がSHEのとき、選択済みを示すエラーになる", "SHE"},
					{"陣営がTenkiのとき、選択済みを示すエラーになる", "Tenki"},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						env := usecasetest.NewUnitEnv(t)
						playerID := usecasetest.RegisterPlayer(t, env.Auth)
						require.NoError(t, env.Faction.SelectInitialFaction(t.Context(), playerID, "SHE"))
						err := env.Faction.SelectInitialFaction(t.Context(), playerID, tt.faction)
						assert.ErrorIs(t, err, usecase.ErrFactionAlreadySelected)
					})
				}

				t.Run("陣営がTenkiのとき、選択に失敗したあとにプレイヤー情報を取得すると、initial_factionはSHEのままになる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					require.NoError(t, env.Faction.SelectInitialFaction(t.Context(), playerID, "SHE"))
					require.Error(t, env.Faction.SelectInitialFaction(t.Context(), playerID, "Tenki"))
					assert.Equal(t, usecasetest.ToPointer("SHE"), playerInfoOf(t, env, playerID).View.InitialFaction)
				})
			})

			t.Run("未登録のプレイヤーがSHEを選択するとき", func(t *testing.T) {
				t.Run("プレイヤーが存在しないことを示すエラーになる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					err := env.Faction.SelectInitialFaction(t.Context(), uuid.NewString(), "SHE")
					assert.ErrorIs(t, err, port.ErrNotFound)
				})

				t.Run("選択に失敗したあとに所持陣営一覧を取得すると、一覧は空のままになる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := uuid.NewString()
					require.Error(t, env.Faction.SelectInitialFaction(t.Context(), playerID, "SHE"))
					factions, err := env.Store.FactionRepo().GetPlayerFactions(t.Context(), playerID)
					require.NoError(t, err)
					assert.Empty(t, factions)
				})
			})
		})
	})
}

func TestListFactions(t *testing.T) {
	t.Run("[所持陣営]所持陣営一覧の取得", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("登録済みで所持陣営が無いプレイヤーの所持陣営一覧を取得すると、一覧は空になる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				factions, err := env.Faction.ListFactions(t.Context(), playerID)
				require.NoError(t, err)
				assert.Empty(t, factions)
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("未登録のプレイヤーの所持陣営一覧を取得すると、プレイヤーが存在しないことを示すエラーになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				_, err := env.Faction.ListFactions(t.Context(), uuid.NewString())
				assert.ErrorIs(t, err, port.ErrNotFound)
			})
		})
	})
}

func TestApplyFactionAcquired(t *testing.T) {
	t.Run("[所持陣営]陣営獲得イベントの反映", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("所持陣営が無い登録済みのプレイヤーに、陣営がNeutralの未処理のイベントを反映すると、反映したあとに取得した所持陣営一覧はNeutralだけになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				require.NoError(t, env.Faction.ApplyFactionAcquired(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "Neutral"))
				factions, err := env.Faction.ListFactions(t.Context(), playerID)
				require.NoError(t, err)
				assert.Equal(t, []string{"Neutral"}, factions)
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("所持陣営が無い登録済みのプレイヤーに未処理のイベントを反映するとき", func(t *testing.T) {
				t.Run("陣営がUnknownのとき、所持陣営として存在しない陣営を示すエラーになる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					err := env.Faction.ApplyFactionAcquired(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "Unknown")
					assert.ErrorIs(t, err, usecase.ErrUnknownFaction)
				})

				t.Run("陣営がUnknownのイベントの反映に失敗したあとに所持陣営一覧を取得すると、一覧は空のままになる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					require.Error(t, env.Faction.ApplyFactionAcquired(t.Context(), uuid.NewString(), usecasetest.DummyEventType, playerID, "Unknown"))
					factions, err := env.Faction.ListFactions(t.Context(), playerID)
					require.NoError(t, err)
					assert.Empty(t, factions)
				})
			})
		})
	})
}

func TestGrantFaction(t *testing.T) {
	t.Run("[所持陣営]陣営の付与", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("所持陣営が無い登録済みのプレイヤーにSHEを付与したあとに取得した所持陣営一覧は、SHEだけになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				playerID := usecasetest.RegisterPlayer(t, env.Auth)
				require.NoError(t, env.Faction.GrantFaction(t.Context(), playerID, "SHE"))
				factions, err := env.Faction.ListFactions(t.Context(), playerID)
				require.NoError(t, err)
				assert.Equal(t, []string{"SHE"}, factions)
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("所持陣営が無い登録済みのプレイヤーにNeutralを付与するとき", func(t *testing.T) {
				t.Run("不正な陣営を示すエラーになる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					err := env.Faction.GrantFaction(t.Context(), playerID, "Neutral")
					assert.ErrorIs(t, err, usecase.ErrInvalidFaction)
				})

				t.Run("付与に失敗したあとに所持陣営一覧を取得すると、一覧は空のままになる", func(t *testing.T) {
					env := usecasetest.NewUnitEnv(t)
					playerID := usecasetest.RegisterPlayer(t, env.Auth)
					require.Error(t, env.Faction.GrantFaction(t.Context(), playerID, "Neutral"))
					factions, err := env.Faction.ListFactions(t.Context(), playerID)
					require.NoError(t, err)
					assert.Empty(t, factions)
				})
			})

			t.Run("未登録のプレイヤーにSHEを付与すると、プレイヤーが存在しないことを示すエラーになる", func(t *testing.T) {
				env := usecasetest.NewUnitEnv(t)
				err := env.Faction.GrantFaction(t.Context(), uuid.NewString(), "SHE")
				assert.ErrorIs(t, err, port.ErrNotFound)
			})
		})
	})
}
