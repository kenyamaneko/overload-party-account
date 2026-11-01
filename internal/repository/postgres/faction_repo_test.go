//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres/postgrestest"
)

const sqlStateCheckViolation = "23514"

func getPlayerFactions(t *testing.T, r *postgrestest.Repositories, playerID string) []string {
	t.Helper()
	factions, err := r.Factions.GetPlayerFactions(t.Context(), playerID)
	require.NoError(t, err)
	return factions
}

func getInitialFaction(t *testing.T, r *postgrestest.Repositories, playerID string) *string {
	t.Helper()
	faction, err := r.Factions.GetInitialFaction(t.Context(), playerID)
	require.NoError(t, err)
	return faction
}

func setInitialFactionsConcurrently(t *testing.T, r *postgrestest.Repositories, factions []string) []error {
	t.Helper()
	r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
	return postgrestest.RunConcurrently(len(factions), func(i int) error {
		return r.Factions.SetInitialFaction(t.Context(), postgrestest.PlayerID1, factions[i])
	})
}

func setInitialFactionAfterTenki(t *testing.T, r *postgrestest.Repositories, faction string) error {
	t.Helper()
	r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
	r.SetInitialFaction(t, postgrestest.PlayerID1, "Tenki")
	return r.Factions.SetInitialFaction(t.Context(), postgrestest.PlayerID1, faction)
}

func TestFactionGrant(t *testing.T) {
	t.Run("[陣営]所持陣営の付与", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("所持陣営が無いプレイヤーに付与して所持陣営一覧を取得すると", func(t *testing.T) {
				tests := []struct {
					name    string
					faction string
					want    []string
				}{
					{name: "付与する陣営が「SHE」のとき、所持陣営一覧が「SHE」だけになる", faction: "SHE", want: []string{"SHE"}},
					{name: "付与する陣営が「Tenki」のとき、所持陣営一覧が「Tenki」だけになる", faction: "Tenki", want: []string{"Tenki"}},
					{name: "付与する陣営が「Sugar」のとき、所持陣営一覧が「Sugar」だけになる", faction: "Sugar", want: []string{"Sugar"}},
					{name: "付与する陣営が「Tuners」のとき、所持陣営一覧が「Tuners」だけになる", faction: "Tuners", want: []string{"Tuners"}},
					{name: "付与する陣営が「Neutral」のとき、所持陣営一覧が「Neutral」だけになる", faction: "Neutral", want: []string{"Neutral"}},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))

						r.AddFaction(t, postgrestest.PlayerID1, tt.faction)

						assert.Equal(t, tt.want, getPlayerFactions(t, r, postgrestest.PlayerID1))
					})
				}
			})
			t.Run("「Tenki」が付与済みのとき、もう一度「Tenki」を付与すると、所持陣営一覧が「Tenki」だけになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				r.AddFaction(t, postgrestest.PlayerID1, "Tenki")

				r.AddFaction(t, postgrestest.PlayerID1, "Tenki")

				assert.Equal(t, []string{"Tenki"}, getPlayerFactions(t, r, postgrestest.PlayerID1))
			})
			t.Run("陣営を付与したとき、続けて初期陣営を取得すると、該当なしになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				r.AddFaction(t, postgrestest.PlayerID1, "Tenki")

				got := getInitialFaction(t, r, postgrestest.PlayerID1)

				assert.Nil(t, got)
			})
			t.Run("「Tenki」が初期陣営のとき、もう一度「Tenki」を付与してから初期陣営を取得すると、「Tenki」が返る", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				r.SetInitialFaction(t, postgrestest.PlayerID1, "Tenki")
				r.AddFaction(t, postgrestest.PlayerID1, "Tenki")

				got := getInitialFaction(t, r, postgrestest.PlayerID1)

				assert.Equal(t, postgrestest.NewPointer("Tenki"), got)
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("付与すると", func(t *testing.T) {
				tests := []struct {
					name    string
					faction string
				}{
					{name: "定義外の陣営名「Unknown」のとき、定義外の陣営名を示すエラーになる", faction: "Unknown"},
					{name: "定義済みの「Tenki」と大文字小文字だけが異なる「tenki」のとき、定義外の陣営名を示すエラーになる", faction: "tenki"},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))

						err := r.Factions.AddPlayerFaction(t.Context(), postgrestest.PlayerID1, tt.faction)

						postgrestest.RequirePgError(t, err, sqlStateCheckViolation)
					})
				}
			})
			t.Run("定義外の陣営名「Unknown」を付与して定義外の陣営名を示すエラーになった後に、続けて「Tenki」を付与すると、所持陣営一覧が「Tenki」だけになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				require.Error(t, r.Factions.AddPlayerFaction(t.Context(), postgrestest.PlayerID1, "Unknown"))

				r.AddFaction(t, postgrestest.PlayerID1, "Tenki")

				assert.Equal(t, []string{"Tenki"}, getPlayerFactions(t, r, postgrestest.PlayerID1))
			})
			t.Run("未登録のプレイヤーIDのとき、付与すると、存在しないプレイヤーを指すエラーになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)

				err := r.Factions.AddPlayerFaction(t.Context(), postgrestest.UnregisteredPlayerID, "Tenki")

				postgrestest.RequirePgError(t, err, postgrestest.SQLStateForeignKeyViolation)
			})
		})
	})
}

func TestFactionInitialSelection(t *testing.T) {
	t.Run("[陣営]初期陣営選択", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("「Tenki」を設定して初期陣営を取得すると", func(t *testing.T) {
				tests := []struct {
					name    string
					prepare func(t *testing.T, r *postgrestest.Repositories)
				}{
					{
						name:    "所持陣営が無いとき、「Tenki」が返る",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {},
					},
					{
						name: "「Tenki」を付与済み(初期陣営ではない)のとき、「Tenki」が返る",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.AddFaction(t, postgrestest.PlayerID1, "Tenki")
						},
					},
					{
						name: "「SHE」を付与済みのとき、「Tenki」が返る",
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

						r.SetInitialFaction(t, postgrestest.PlayerID1, "Tenki")

						assert.Equal(t, postgrestest.NewPointer("Tenki"), getInitialFaction(t, r, postgrestest.PlayerID1))
					})
				}
			})
			t.Run("「Tenki」を設定して所持陣営一覧を取得すると", func(t *testing.T) {
				tests := []struct {
					name    string
					prepare func(t *testing.T, r *postgrestest.Repositories)
					want    []string
				}{
					{
						name:    "所持陣営が無いとき、「Tenki」だけが返る",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {},
						want:    []string{"Tenki"},
					},
					{
						name: "「Tenki」を付与済み(初期陣営ではない)のとき、「Tenki」だけが返る",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.AddFaction(t, postgrestest.PlayerID1, "Tenki")
						},
						want: []string{"Tenki"},
					},
					{
						name: "「SHE」を付与済みのとき、「SHE」と「Tenki」の2件が返る",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.AddFaction(t, postgrestest.PlayerID1, "SHE")
						},
						want: []string{"SHE", "Tenki"},
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
						tt.prepare(t, r)

						r.SetInitialFaction(t, postgrestest.PlayerID1, "Tenki")

						assert.ElementsMatch(t, tt.want, getPlayerFactions(t, r, postgrestest.PlayerID1))
					})
				}
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("「Tenki」が初期陣営のとき", func(t *testing.T) {
				t.Run("初期陣営を設定すると", func(t *testing.T) {
					tests := []struct {
						name    string
						faction string
					}{
						{name: "設定する陣営が「Tenki」のとき、初期陣営が既にあることを示すエラーになる", faction: "Tenki"},
						{name: "設定する陣営が「Sugar」のとき、初期陣営が既にあることを示すエラーになる", faction: "Sugar"},
					}
					for _, tt := range tests {
						t.Run(tt.name, func(t *testing.T) {
							r := postgrestest.NewRepositories(t, sharedPg)

							err := setInitialFactionAfterTenki(t, r, tt.faction)

							assert.ErrorIs(t, err, port.ErrAlreadyExists)
						})
					}
				})
				t.Run("初期陣営を設定して初期陣営が既にあることを示すエラーになった後に", func(t *testing.T) {
					t.Run("設定した陣営が「Tenki」のとき、続けて所持陣営一覧を取得すると、「Tenki」だけが返る", func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						require.ErrorIs(t, setInitialFactionAfterTenki(t, r, "Tenki"), port.ErrAlreadyExists)

						assert.Equal(t, []string{"Tenki"}, getPlayerFactions(t, r, postgrestest.PlayerID1))
					})
					t.Run("設定した陣営が「Sugar」のとき", func(t *testing.T) {
						t.Run("続けて初期陣営を取得すると、「Tenki」が返る", func(t *testing.T) {
							r := postgrestest.NewRepositories(t, sharedPg)
							require.ErrorIs(t, setInitialFactionAfterTenki(t, r, "Sugar"), port.ErrAlreadyExists)

							assert.Equal(t, postgrestest.NewPointer("Tenki"), getInitialFaction(t, r, postgrestest.PlayerID1))
						})
						t.Run("続けて「Sugar」を付与すると、所持陣営一覧が「Tenki」と「Sugar」の2件になる", func(t *testing.T) {
							r := postgrestest.NewRepositories(t, sharedPg)
							require.ErrorIs(t, setInitialFactionAfterTenki(t, r, "Sugar"), port.ErrAlreadyExists)

							r.AddFaction(t, postgrestest.PlayerID1, "Sugar")

							assert.ElementsMatch(t, []string{"Tenki", "Sugar"}, getPlayerFactions(t, r, postgrestest.PlayerID1))
						})
					})
				})
			})
			t.Run("初期陣営が未設定のプレイヤーに初期陣営を同時に設定すると", func(t *testing.T) {
				tests := []struct {
					name     string
					factions []string
				}{
					{name: "設定する陣営が「Tenki」と「Sugar」のとき、片方だけが成功し、もう片方は初期陣営が既にあることを示すエラーになる", factions: []string{"Tenki", "Sugar"}},
					{name: "設定する陣営が「Tenki」の2件のとき、片方だけが成功し、もう片方は初期陣営が既にあることを示すエラーになる", factions: []string{"Tenki", "Tenki"}},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)

						errs := setInitialFactionsConcurrently(t, r, tt.factions)

						require.Len(t, postgrestest.FilterFailures(errs), 1)
						assert.ErrorIs(t, postgrestest.FilterFailures(errs)[0], port.ErrAlreadyExists)
					})
				}
			})
			t.Run("初期陣営が未設定のプレイヤーに初期陣営を同時に設定して片方だけが成功した後に", func(t *testing.T) {
				t.Run("続けて初期陣営を取得すると", func(t *testing.T) {
					tests := []struct {
						name     string
						factions []string
					}{
						{name: "設定した陣営が「Tenki」と「Sugar」のとき、取得結果が成功した側の陣営になる", factions: []string{"Tenki", "Sugar"}},
						{name: "設定した陣営が「Tenki」の2件のとき、「Tenki」が返る", factions: []string{"Tenki", "Tenki"}},
					}
					for _, tt := range tests {
						t.Run(tt.name, func(t *testing.T) {
							r := postgrestest.NewRepositories(t, sharedPg)
							errs := setInitialFactionsConcurrently(t, r, tt.factions)
							require.Len(t, postgrestest.FilterFailures(errs), 1)

							got := getInitialFaction(t, r, postgrestest.PlayerID1)

							assert.Equal(t, postgrestest.NewPointer(tt.factions[postgrestest.FindSucceededIndex(errs)]), got)
						})
					}
				})
				t.Run("続けて所持陣営一覧を取得すると", func(t *testing.T) {
					tests := []struct {
						name     string
						factions []string
					}{
						{name: "設定した陣営が「Tenki」と「Sugar」のとき、成功した側の陣営だけが返る", factions: []string{"Tenki", "Sugar"}},
						{name: "設定した陣営が「Tenki」の2件のとき、「Tenki」だけが返る", factions: []string{"Tenki", "Tenki"}},
					}
					for _, tt := range tests {
						t.Run(tt.name, func(t *testing.T) {
							r := postgrestest.NewRepositories(t, sharedPg)
							errs := setInitialFactionsConcurrently(t, r, tt.factions)
							require.Len(t, postgrestest.FilterFailures(errs), 1)

							got := getPlayerFactions(t, r, postgrestest.PlayerID1)

							assert.Equal(t, []string{tt.factions[postgrestest.FindSucceededIndex(errs)]}, got)
						})
					}
				})
			})
		})
	})
}

func TestFactionOwnedList(t *testing.T) {
	t.Run("[陣営]所持陣営一覧の取得", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("所持陣営が無いとき、空の一覧が返る", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))

				assert.Empty(t, getPlayerFactions(t, r, postgrestest.PlayerID1))
			})
			t.Run("初期陣営「Tenki」と、付与済みの「SHE」「Sugar」を所持しているとき、「Tenki」「SHE」「Sugar」の3件が順不同で返る", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				r.SetInitialFaction(t, postgrestest.PlayerID1, "Tenki")
				r.AddFaction(t, postgrestest.PlayerID1, "SHE")
				r.AddFaction(t, postgrestest.PlayerID1, "Sugar")

				assert.ElementsMatch(t, []string{"Tenki", "SHE", "Sugar"}, getPlayerFactions(t, r, postgrestest.PlayerID1))
			})
			t.Run("所持陣営一覧を取得すると", func(t *testing.T) {
				tests := []struct {
					name    string
					prepare func(t *testing.T, r *postgrestest.Repositories)
				}{
					{
						name: "所持陣営が「SHE」の1件のとき、「SHE」だけが返る",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.AddFaction(t, postgrestest.PlayerID1, "SHE")
						},
					},
					{
						name: "指定したプレイヤーが「SHE」だけを所持し、別のプレイヤーも陣営を所持しているとき、「SHE」だけが返る",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2))
							r.AddFaction(t, postgrestest.PlayerID1, "SHE")
							r.AddFaction(t, postgrestest.PlayerID2, "Neutral")
						},
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
						tt.prepare(t, r)

						assert.Equal(t, []string{"SHE"}, getPlayerFactions(t, r, postgrestest.PlayerID1))
					})
				}
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("未登録のプレイヤーIDのとき、空の一覧が返る", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)

				assert.Empty(t, getPlayerFactions(t, r, postgrestest.UnregisteredPlayerID))
			})
		})
	})
}

func TestFactionInitialLookup(t *testing.T) {
	t.Run("[陣営]初期陣営の取得", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("初期陣営を取得すると", func(t *testing.T) {
				tests := []struct {
					name    string
					prepare func(t *testing.T, r *postgrestest.Repositories)
				}{
					{
						name: "初期陣営に「Tenki」を設定済みのとき、「Tenki」が返る",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.SetInitialFaction(t, postgrestest.PlayerID1, "Tenki")
						},
					},
					{
						name: "初期陣営「Tenki」と、初期陣営ではない陣営を所持しているとき、「Tenki」が返る",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.SetInitialFaction(t, postgrestest.PlayerID1, "Tenki")
							r.AddFaction(t, postgrestest.PlayerID1, "SHE")
						},
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
						tt.prepare(t, r)

						assert.Equal(t, postgrestest.NewPointer("Tenki"), getInitialFaction(t, r, postgrestest.PlayerID1))
					})
				}
			})
			t.Run("初期陣営が未選択のとき、取得すると", func(t *testing.T) {
				tests := []struct {
					name    string
					prepare func(t *testing.T, r *postgrestest.Repositories)
				}{
					{
						name:    "所持陣営が無いとき、該当なしになる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {},
					},
					{
						name: "初期陣営ではない陣営だけを所持しているとき、該当なしになる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.AddFaction(t, postgrestest.PlayerID1, "SHE")
						},
					},
					{
						name: "別のプレイヤーだけが初期陣営を設定済みのとき、該当なしになる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2))
							r.SetInitialFaction(t, postgrestest.PlayerID2, "Tenki")
						},
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
						tt.prepare(t, r)

						assert.Nil(t, getInitialFaction(t, r, postgrestest.PlayerID1))
					})
				}
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("呼び出しが取り消し済みのとき、取得すると、取り消しを示すエラーになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				r.SetInitialFaction(t, postgrestest.PlayerID1, "Tenki")
				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				_, err := r.Factions.GetInitialFaction(ctx, postgrestest.PlayerID1)

				assert.ErrorIs(t, err, context.Canceled)
			})
		})
	})
}
