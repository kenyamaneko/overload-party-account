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

func TestPlayerRegistration(t *testing.T) {
	t.Run("[プレイヤー]プレイヤーの登録", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("登録後にプレイヤーIDで取得すると", func(t *testing.T) {
				t.Run("表示名が未設定のとき、取得結果の表示名が未設定になる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))

					got := r.GetPlayer(t, postgrestest.PlayerID1)

					assert.Nil(t, got.Name)
				})
				t.Run("装備アイコン番号が未設定のとき、取得結果の装備アイコン番号が未設定になる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))

					got := r.GetPlayer(t, postgrestest.PlayerID1)

					assert.Nil(t, got.EquippedIconNo)
				})
				t.Run("プレミアムの有効期限が未設定のとき、取得結果のプレミアムの有効期限が未設定になる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))

					got := r.GetPlayer(t, postgrestest.PlayerID1)

					assert.Nil(t, got.PremiumExpiresAt)
				})
				t.Run("表示名が「テスト太郎」のとき、取得結果の表示名が「テスト太郎」になる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
						player.Name = postgrestest.NewPointer("テスト太郎")
					}))

					got := r.GetPlayer(t, postgrestest.PlayerID1)

					assert.Equal(t, postgrestest.NewPointer("テスト太郎"), got.Name)
				})
				t.Run("装備アイコン番号が3のとき、取得結果の装備アイコン番号が3になる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
						player.EquippedIconNo = postgrestest.NewPointer(int64(3))
					}))

					got := r.GetPlayer(t, postgrestest.PlayerID1)

					assert.Equal(t, postgrestest.NewPointer(int64(3)), got.EquippedIconNo)
				})
				t.Run("プレミアム状態が有効のとき、取得結果のプレミアム状態が有効になる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
						player.IsPremium = true
					}))

					got := r.GetPlayer(t, postgrestest.PlayerID1)

					assert.True(t, got.IsPremium)
				})
				t.Run("プレミアムの有効期限が2026-12-31T23:59:59Zのとき、取得結果のプレミアムの有効期限が2026-12-31T23:59:59Zになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
						player.PremiumExpiresAt = postgrestest.NewPointer(postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59))
					}))

					got := r.GetPlayer(t, postgrestest.PlayerID1)

					assert.Equal(t, []time.Time{postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59)}, postgrestest.ToUTCTimes(got.PremiumExpiresAt))
				})
				t.Run("作成日時が2026-01-01T00:00:00Zのとき、取得結果の作成日時が2026-01-01T00:00:00Zになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
						player.CreatedAt = postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0)
					}))

					got := r.GetPlayer(t, postgrestest.PlayerID1)

					assert.Equal(t, postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0), got.CreatedAt.UTC())
				})
				t.Run("更新日時が2026-01-02T00:00:00Zのとき、取得結果の更新日時が2026-01-02T00:00:00Zになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
						player.UpdatedAt = postgrestest.NewUTCTime(2026, time.January, 2, 0, 0, 0)
					}))

					got := r.GetPlayer(t, postgrestest.PlayerID1)

					assert.Equal(t, postgrestest.NewUTCTime(2026, time.January, 2, 0, 0, 0), got.UpdatedAt.UTC())
				})
			})
			t.Run("オンボーディング進行状態を指定して登録し、プレイヤーIDで取得すると", func(t *testing.T) {
				tests := []struct {
					name   string
					status string
					want   string
				}{
					{name: "状態がnot_startedのとき、取得結果の状態がnot_startedになる", status: "not_started", want: "not_started"},
					{name: "状態がname_setのとき、取得結果の状態がname_setになる", status: "name_set", want: "name_set"},
					{name: "状態がfaction_setのとき、取得結果の状態がfaction_setになる", status: "faction_set", want: "faction_set"},
					{name: "状態がcompletedのとき、取得結果の状態がcompletedになる", status: "completed", want: "completed"},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
							player.OnboardingStatus = tt.status
						}))

						got := r.GetPlayer(t, postgrestest.PlayerID1)

						assert.Equal(t, tt.want, got.OnboardingStatus)
					})
				}
			})
			t.Run("経験値を指定して登録し、プレイヤーIDで取得すると", func(t *testing.T) {
				tests := []struct {
					name string
					exp  int64
					want int64
				}{
					{name: "経験値が0のとき、取得結果の経験値が0になる", exp: 0, want: 0},
					{name: "経験値が120のとき、取得結果の経験値が120になる", exp: 120, want: 120},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						r.RegisterPlayerWithProgression(t,
							postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1),
							postgrestest.NewProgressionWithExp(postgrestest.PlayerID1, tt.exp))

						got := r.GetProgression(t, postgrestest.PlayerID1)

						assert.Equal(t, tt.want, got.Exp)
					})
				}
			})
			t.Run("累計経験値の更新日時が2026-01-03T00:00:00Zのとき、登録後に累計経験値を取得すると、取得結果の更新日時が2026-01-03T00:00:00Zになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				progression := postgrestest.NewProgression(postgrestest.PlayerID1)
				progression.UpdatedAt = postgrestest.NewUTCTime(2026, time.January, 3, 0, 0, 0)
				r.RegisterPlayerWithProgression(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1), progression)

				got := r.GetProgression(t, postgrestest.PlayerID1)

				assert.Equal(t, postgrestest.NewUTCTime(2026, time.January, 3, 0, 0, 0), got.UpdatedAt.UTC())
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("登録すると", func(t *testing.T) {
				tests := []struct {
					name           string
					first          *domain.Player
					duplicate      *domain.Player
					wantConstraint string
				}{
					{
						name:           "FirebaseUIDが登録済みのプレイヤーと重複し、プレイヤーIDが異なるとき、FirebaseUIDの重複を示すエラーになる",
						first:          postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1),
						duplicate:      postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID1),
						wantConstraint: "idx_players_firebase_uid",
					},
					{
						name:           "プレイヤーIDが登録済みのプレイヤーと重複し、FirebaseUIDが異なるとき、プレイヤーIDの重複を示すエラーになる",
						first:          postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1),
						duplicate:      postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID2),
						wantConstraint: "players_pkey",
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						r.RegisterPlayer(t, tt.first)

						err := r.Players.Create(t.Context(), tt.duplicate, postgrestest.NewProgression(tt.duplicate.PlayerID))

						pgErr := postgrestest.RequirePgError(t, err, postgrestest.SQLStateUniqueViolation)
						assert.Equal(t, tt.wantConstraint, pgErr.ConstraintName)
						assert.ErrorIs(t, err, port.ErrAlreadyExists)
					})
				}
			})
			t.Run("FirebaseUIDが登録済みのプレイヤーと重複し、プレイヤーIDが異なる登録が、重複を示すエラーになったとき", func(t *testing.T) {
				duplicate := func(t *testing.T, r *postgrestest.Repositories, firstName, duplicateName *string) error {
					r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
						player.Name = firstName
					}))
					return r.Players.Create(t.Context(), postgrestest.NewPlayerWith(postgrestest.PlayerID2, postgrestest.FirebaseUID1, func(player *domain.Player) {
						player.Name = duplicateName
					}), postgrestest.NewProgression(postgrestest.PlayerID2))
				}
				t.Run("最初の表示名が「テスト太郎」で、重複する登録の表示名が異なるとき、続けてFirebaseUIDで取得すると、取得結果の表示名が「テスト太郎」のままになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					require.ErrorIs(t, duplicate(t, r, postgrestest.NewPointer("テスト太郎"), postgrestest.NewPointer("テスト次郎")), port.ErrAlreadyExists)

					got, err := r.Views.GetByFirebaseUID(t.Context(), postgrestest.FirebaseUID1)

					require.NoError(t, err)
					assert.Equal(t, postgrestest.NewPointer("テスト太郎"), got.Player.Name)
				})
				t.Run("続けて別のFirebaseUID、別のプレイヤーIDで表示名「テスト太郎」のプレイヤーを登録すると、取得結果の表示名が「テスト太郎」になる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					require.ErrorIs(t, duplicate(t, r, nil, nil), port.ErrAlreadyExists)
					r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID3, postgrestest.FirebaseUID3, func(player *domain.Player) {
						player.Name = postgrestest.NewPointer("テスト太郎")
					}))

					got := r.GetPlayer(t, postgrestest.PlayerID3)

					assert.Equal(t, postgrestest.NewPointer("テスト太郎"), got.Name)
				})
			})
			t.Run("プレイヤーIDが登録済みのプレイヤーと重複し、FirebaseUIDが異なる登録が、重複を示すエラーになった後に、最初のFirebaseUIDが「test-uid-1」のとき、プレイヤーIDで取得すると、取得結果のFirebaseUIDが「test-uid-1」のままになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				require.ErrorIs(t, r.Players.Create(t.Context(),
					postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID2),
					postgrestest.NewProgression(postgrestest.PlayerID1)), port.ErrAlreadyExists)

				got := r.GetPlayer(t, postgrestest.PlayerID1)

				assert.Equal(t, "test-uid-1", got.FirebaseUID)
			})
			t.Run("プレイヤーが1件も登録されていないとき、同じFirebaseUIDで2件を同時に登録すると", func(t *testing.T) {
				registerConcurrently := func(t *testing.T, r *postgrestest.Repositories) ([]*domain.Player, []error) {
					players := []*domain.Player{
						postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1),
						postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID1),
					}
					errs := postgrestest.RunConcurrently(len(players), func(i int) error {
						return r.Players.Create(t.Context(), players[i], postgrestest.NewProgression(players[i].PlayerID))
					})
					return players, errs
				}
				t.Run("片方はエラーなしで登録を終え、もう片方はFirebaseUIDの重複を示すエラーになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)

					_, errs := registerConcurrently(t, r)

					failures := postgrestest.FilterFailures(errs)
					require.Len(t, failures, 1)
					pgErr := postgrestest.RequirePgError(t, failures[0], postgrestest.SQLStateUniqueViolation)
					assert.Equal(t, "idx_players_firebase_uid", pgErr.ConstraintName)
					assert.ErrorIs(t, failures[0], port.ErrAlreadyExists)
				})
				t.Run("続けてFirebaseUIDで取得すると、取得結果が登録を終えた側のプレイヤーになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					players, errs := registerConcurrently(t, r)
					require.Len(t, postgrestest.FilterFailures(errs), 1)

					got, err := r.Views.GetByFirebaseUID(t.Context(), postgrestest.FirebaseUID1)

					require.NoError(t, err)
					assert.Equal(t, players[postgrestest.FindSucceededIndex(errs)].PlayerID, got.Player.PlayerID)
				})
			})
			t.Run("累計経験値に指定したプレイヤーIDが登録するプレイヤーのIDと異なるとき、登録すると", func(t *testing.T) {
				mismatched := func(t *testing.T, r *postgrestest.Repositories) error {
					return r.Players.Create(t.Context(),
						postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1),
						postgrestest.NewProgression(postgrestest.PlayerID2))
				}
				t.Run("存在しないプレイヤーを指すエラーになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)

					err := mismatched(t, r)

					postgrestest.RequirePgError(t, err, postgrestest.SQLStateForeignKeyViolation)
				})
				t.Run("存在しないプレイヤーを指すエラーになった後に、登録するはずだったプレイヤーIDで取得すると、見つからないエラーになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					require.Error(t, mismatched(t, r))

					_, err := r.Players.GetByID(t.Context(), postgrestest.PlayerID1)

					assert.ErrorIs(t, err, port.ErrNotFound)
				})
			})
		})
	})
}

func TestPlayerLookup(t *testing.T) {
	t.Run("[プレイヤー]プレイヤーの取得", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("表示名が「テスト太郎」「テスト次郎」「テスト三郎」の3件のプレイヤーが登録されているとき、「テスト次郎」のプレイヤーIDで取得すると、取得結果の表示名が「テスト次郎」になる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterThreeNamedPlayers(t)

				got, err := r.Players.GetByID(t.Context(), postgrestest.PlayerID2)

				require.NoError(t, err)
				assert.Equal(t, postgrestest.NewPointer("テスト次郎"), got.Name)
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

						_, err := r.Players.GetByID(t.Context(), postgrestest.PlayerID1)

						assert.ErrorIs(t, err, port.ErrNotFound)
					})
				}
			})
		})
	})
}

func TestPlayerExistence(t *testing.T) {
	t.Run("[プレイヤー]プレイヤーの存在確認", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("プレイヤーIDで確認すると", func(t *testing.T) {
				tests := []struct {
					name     string
					prepare  func(t *testing.T, r *postgrestest.Repositories)
					playerID string
					want     bool
				}{
					{
						name: "指定したプレイヤーIDのプレイヤーが登録済みのとき、存在するという結果になる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
						},
						playerID: postgrestest.PlayerID1,
						want:     true,
					},
					{
						name: "3件のプレイヤーが登録済みで、指定したプレイヤーIDのプレイヤーがその中にあるとき、存在するという結果になる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
							r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2))
							r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID3, postgrestest.FirebaseUID3))
						},
						playerID: postgrestest.PlayerID2,
						want:     true,
					},
					{
						name: "別のプレイヤーだけが登録済みのとき、存在しないという結果になる",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID2, postgrestest.FirebaseUID2))
						},
						playerID: postgrestest.PlayerID1,
						want:     false,
					},
					{
						name:     "プレイヤーが1件も登録されていないとき、存在しないという結果になる",
						prepare:  func(t *testing.T, r *postgrestest.Repositories) {},
						playerID: postgrestest.PlayerID1,
						want:     false,
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						tt.prepare(t, r)

						got, err := r.Players.Exists(t.Context(), tt.playerID)

						require.NoError(t, err)
						assert.Equal(t, tt.want, got)
					})
				}
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("呼び出しが取り消し済みのとき、確認すると、取り消しを示すエラーになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				_, err := r.Players.Exists(ctx, postgrestest.PlayerID1)

				assert.ErrorIs(t, err, context.Canceled)
			})
		})
	})
}

func TestPlayerNameUpdate(t *testing.T) {
	t.Run("[プレイヤー]表示名更新", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("表示名を更新してプレイヤーIDで取得すると", func(t *testing.T) {
				t.Run("更新前と更新する表示名を指定すると", func(t *testing.T) {
					tests := []struct {
						name   string
						before *string
						next   string
					}{
						{name: "更新前が未設定で、更新する表示名が「テスト太郎」のとき、取得結果の表示名が「テスト太郎」になる", before: nil, next: "テスト太郎"},
						{name: "更新前が「テスト太郎」で、更新する表示名が「テスト花子」のとき、取得結果の表示名が「テスト花子」になる", before: postgrestest.NewPointer("テスト太郎"), next: "テスト花子"},
					}
					for _, tt := range tests {
						t.Run(tt.name, func(t *testing.T) {
							r := postgrestest.NewRepositories(t, sharedPg)
							r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
								player.Name = tt.before
							}))

							require.NoError(t, r.Players.UpdateName(t.Context(), postgrestest.PlayerID1, tt.next))

							assert.Equal(t, postgrestest.NewPointer(tt.next), r.GetPlayer(t, postgrestest.PlayerID1).Name)
						})
					}
				})
				t.Run("更新日時が2026-01-01T00:00:00Zのとき、表示名を更新すると、取得結果の更新日時が表示名を更新した日時になる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
						player.UpdatedAt = postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0)
					}))
					before := postgrestest.ReadDatabaseTime(t, r.Pool)

					require.NoError(t, r.Players.UpdateName(t.Context(), postgrestest.PlayerID1, "テスト太郎"))

					after := postgrestest.ReadDatabaseTime(t, r.Pool)
					updatedAt := r.GetPlayer(t, postgrestest.PlayerID1).UpdatedAt
					assert.GreaterOrEqual(t, updatedAt, before)
					assert.LessOrEqual(t, updatedAt, after)
				})
				t.Run("FirebaseUIDが「test-uid-1」のとき、表示名を更新すると、取得結果のFirebaseUIDが「test-uid-1」のままになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, "test-uid-1"))

					require.NoError(t, r.Players.UpdateName(t.Context(), postgrestest.PlayerID1, "テスト花子"))

					assert.Equal(t, "test-uid-1", r.GetPlayer(t, postgrestest.PlayerID1).FirebaseUID)
				})
				t.Run("装備アイコン番号が3のとき、表示名を更新すると、取得結果の装備アイコン番号が3のままになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
						player.EquippedIconNo = postgrestest.NewPointer(int64(3))
					}))

					require.NoError(t, r.Players.UpdateName(t.Context(), postgrestest.PlayerID1, "テスト花子"))

					assert.Equal(t, postgrestest.NewPointer(int64(3)), r.GetPlayer(t, postgrestest.PlayerID1).EquippedIconNo)
				})
				t.Run("プレミアム状態が有効のとき、表示名を更新すると、取得結果のプレミアム状態が有効のままになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
						player.IsPremium = true
					}))

					require.NoError(t, r.Players.UpdateName(t.Context(), postgrestest.PlayerID1, "テスト花子"))

					assert.True(t, r.GetPlayer(t, postgrestest.PlayerID1).IsPremium)
				})
				t.Run("プレミアムの有効期限が2026-12-31T23:59:59Zのとき、表示名を更新すると、取得結果のプレミアムの有効期限が2026-12-31T23:59:59Zのままになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
						player.PremiumExpiresAt = postgrestest.NewPointer(postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59))
					}))

					require.NoError(t, r.Players.UpdateName(t.Context(), postgrestest.PlayerID1, "テスト花子"))

					assert.Equal(t, []time.Time{postgrestest.NewUTCTime(2026, time.December, 31, 23, 59, 59)},
						postgrestest.ToUTCTimes(r.GetPlayer(t, postgrestest.PlayerID1).PremiumExpiresAt))
				})
				t.Run("オンボーディング進行状態がcompletedのとき、表示名を更新すると、取得結果のオンボーディング進行状態がcompletedのままになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
						player.OnboardingStatus = "completed"
					}))

					require.NoError(t, r.Players.UpdateName(t.Context(), postgrestest.PlayerID1, "テスト花子"))

					assert.Equal(t, "completed", r.GetPlayer(t, postgrestest.PlayerID1).OnboardingStatus)
				})
				t.Run("作成日時が2026-01-01T00:00:00Zのとき、表示名を更新すると、取得結果の作成日時が2026-01-01T00:00:00Zのままになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID1, postgrestest.FirebaseUID1, func(player *domain.Player) {
						player.CreatedAt = postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0)
					}))

					require.NoError(t, r.Players.UpdateName(t.Context(), postgrestest.PlayerID1, "テスト花子"))

					assert.Equal(t, postgrestest.NewUTCTime(2026, time.January, 1, 0, 0, 0), r.GetPlayer(t, postgrestest.PlayerID1).CreatedAt.UTC())
				})
				t.Run("別のプレイヤーの表示名が「テスト次郎」のとき、指定したプレイヤーの表示名を更新すると、別のプレイヤーの取得結果の表示名が「テスト次郎」のままになる", func(t *testing.T) {
					r := postgrestest.NewRepositories(t, sharedPg)
					r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))
					r.RegisterPlayer(t, postgrestest.NewPlayerWith(postgrestest.PlayerID2, postgrestest.FirebaseUID2, func(player *domain.Player) {
						player.Name = postgrestest.NewPointer("テスト次郎")
					}))

					require.NoError(t, r.Players.UpdateName(t.Context(), postgrestest.PlayerID1, "テスト太郎"))

					assert.Equal(t, postgrestest.NewPointer("テスト次郎"), r.GetPlayer(t, postgrestest.PlayerID2).Name)
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("未登録のプレイヤーIDのとき、表示名を更新すると、見つからないエラーになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)

				err := r.Players.UpdateName(t.Context(), postgrestest.UnregisteredPlayerID, "テスト太郎")

				assert.ErrorIs(t, err, port.ErrNotFound)
			})
			t.Run("未登録のプレイヤーIDで表示名を「テスト太郎」に更新して見つからないエラーになった後に、そのプレイヤーIDでプレイヤーを登録して同じ更新をすると、取得結果の表示名が「テスト太郎」になる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				require.ErrorIs(t, r.Players.UpdateName(t.Context(), postgrestest.PlayerID1, "テスト太郎"), port.ErrNotFound)
				r.RegisterPlayer(t, postgrestest.NewPlayer(postgrestest.PlayerID1, postgrestest.FirebaseUID1))

				require.NoError(t, r.Players.UpdateName(t.Context(), postgrestest.PlayerID1, "テスト太郎"))

				assert.Equal(t, postgrestest.NewPointer("テスト太郎"), r.GetPlayer(t, postgrestest.PlayerID1).Name)
			})
		})
	})
}
