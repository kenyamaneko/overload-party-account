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
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
)

const (
	testPlayerID1 = "11111111-1111-1111-1111-111111111111"
	testPlayerID2 = "22222222-2222-2222-2222-222222222222"
)

func TestCreate(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	viewRepo := postgres.NewPlayerViewRepository(sharedPg.Pool)
	ctx := context.Background()

	t.Run("プレイヤーの作成", func(t *testing.T) {
		t.Run("player と progression が作成され、name と初期 level=1 / exp=0 が読み出せる", func(t *testing.T) {
			sharedPg.Truncate(t)
			now := time.Now().UTC()
			alice := "Alice"
			p := &domain.Player{
				PlayerID:         testPlayerID1,
				FirebaseUID:      "uid-1",
				Name:             &alice,
				IsPremium:        false,
				OnboardingStatus: domain.OnboardingStatusNotStarted,
				CreatedAt:        now,
				UpdatedAt:        now,
			}
			prog := &domain.PlayerProgression{
				PlayerID:  p.PlayerID,
				Level:     1,
				Exp:       0,
				UpdatedAt: now,
			}
			require.NoError(t, repo.Create(ctx, p, prog))

			got, err := repo.FindByID(ctx, testPlayerID1)
			require.NoError(t, err)
			require.NotNil(t, got.Name)
			assert.Equal(t, "Alice", *got.Name)

			view, err := viewRepo.FindByID(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Equal(t, int64(1), view.Level)
			assert.Equal(t, int64(0), view.Exp)
		})
	})
}

func TestFindByID(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	t.Run("player_id によるプレイヤー取得", func(t *testing.T) {
		tests := []struct {
			name     string
			lookupID string
			wantName *string
			wantErr  error
		}{
			{
				name:     "シード済み player_id のとき、Player を返す",
				lookupID: testPlayerID1,
				wantName: ptr("Alice"),
				wantErr:  nil,
			},
			{
				name:     "未シード player_id のとき、ErrNotFound になる",
				lookupID: testPlayerID2,
				wantName: nil,
				wantErr:  port.ErrNotFound,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

				got, err := repo.FindByID(ctx, tt.lookupID)
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, tt.wantName, playerNamePtr(got))
			})
		}
	})
}

func TestFindByFirebaseUID(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	t.Run("firebase_uid によるプレイヤー取得", func(t *testing.T) {
		// 既登録検出などの業務分岐は呼び出し側が errors.Is(ErrNotFound) で行う契約なので、
		// repo は一致行の有無を Player / ErrNotFound で返すだけ。
		tests := []struct {
			name      string
			lookupUID string
			wantName  *string
			wantErr   error
		}{
			{
				name:      "シード済み firebase_uid のとき、Player を返す",
				lookupUID: "uid-1",
				wantName:  ptr("Alice"),
				wantErr:   nil,
			},
			{
				name:      "未シード firebase_uid のとき、ErrNotFound になる",
				lookupUID: "uid-missing",
				wantName:  nil,
				wantErr:   port.ErrNotFound,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

				got, err := repo.FindByFirebaseUID(ctx, tt.lookupUID)
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, tt.wantName, playerNamePtr(got))
			})
		}
	})
}

func TestExists(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	t.Run("player_id 行の存在確認", func(t *testing.T) {
		tests := []struct {
			name     string
			lookupID string
			want     bool
		}{
			{
				name:     "シード済みの player_id のとき、true を返す",
				lookupID: testPlayerID1,
				want:     true,
			},
			{
				name:     "未シードの player_id のとき、false を返す (該当なしはエラーではない)",
				lookupID: testPlayerID2,
				want:     false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

				got, err := repo.Exists(ctx, tt.lookupID)
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			})
		}
	})
}

func TestGetDailyBattle(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()
	today := civil.DateOf(time.Now().UTC())

	t.Run("当日バトル記録の取得", func(t *testing.T) {
		// 該当なしは (nil, nil)。呼び出し側はカウント 0 として扱うため、エラーにしない。
		tests := []struct {
			name      string
			seedCount []int64 // 0 件 = 未シード、1 件 = その count をシード
			want      *domain.PlayerDailyBattle
		}{
			{
				name:      "該当行があるとき、永続層の値をそのまま返す",
				seedCount: []int64{5},
				want: &domain.PlayerDailyBattle{
					PlayerID:         testPlayerID1,
					GameDate:         today,
					DailyBattleCount: 5,
				},
			},
			{
				name:      "該当行が無いとき、(nil, nil) を返す (エラーではない)",
				seedCount: nil,
				want:      nil,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
				for _, c := range tt.seedCount {
					seedPlayerDailyBattle(t, testPlayerID1, today, c)
				}

				got, err := repo.GetDailyBattle(ctx, testPlayerID1, today)
				require.NoError(t, err)
				assert.Equal(t, tt.want, got)
			})
		}
	})
}

func TestGetProgressionForUpdate(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	txMgr := postgres.NewTxManager(sharedPg.Pool)
	ctx := context.Background()

	t.Run("更新用の進捗取得", func(t *testing.T) {
		// 行ロック取得自体は検証せず、行取得経路とエラー伝播のみを repo テストの責務とする
		// (加算・レベル計算は usecase 層の責務)。
		tests := []struct {
			name      string
			lookupID  string
			wantLevel int64
			wantExp   int64
			wantErr   error
		}{
			{
				name:      "シード済みのとき、現在値を返す",
				lookupID:  testPlayerID1,
				wantLevel: 1,
				wantExp:   0,
				wantErr:   nil,
			},
			{
				name:     "未シードのとき、ErrNotFound になる",
				lookupID: testPlayerID2,
				wantErr:  port.ErrNotFound,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

				err := txMgr.RunInTx(ctx, func(txCtx context.Context) error {
					prog, err := repo.GetProgressionForUpdate(txCtx, tt.lookupID)
					gotLevel, gotExp := progressionLevelExp(prog)
					assert.Equal(t, tt.wantLevel, gotLevel)
					assert.Equal(t, tt.wantExp, gotExp)
					return err
				})
				require.ErrorIs(t, err, tt.wantErr)
			})
		}
	})
}

func TestUpdateName(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	t.Run("name の更新", func(t *testing.T) {
		// 永続化はシード済みケースのみ FindByID 経由で確認する。
		tests := []struct {
			name     string
			lookupID string
			wantName *string
			wantErr  error
		}{
			{
				name:     "シード済みのとき、name を更新する",
				lookupID: testPlayerID1,
				wantName: ptr("Bob"),
				wantErr:  nil,
			},
			{
				name:     "未シードのとき、ErrNotFound になる",
				lookupID: testPlayerID2,
				wantName: nil,
				wantErr:  port.ErrNotFound,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

				err := repo.UpdateName(ctx, tt.lookupID, "Bob")
				require.ErrorIs(t, err, tt.wantErr)

				got, _ := repo.FindByID(ctx, tt.lookupID)
				assert.Equal(t, tt.wantName, playerNamePtr(got))
			})
		}
	})
}

func TestUpdatePremium(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()
	expiresAt := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)

	t.Run("premium 状態の更新", func(t *testing.T) {
		// 永続化はシード済みケースのみ FindByID 経由で確認する。
		tests := []struct {
			name        string
			lookupID    string
			wantPremium bool
			wantErr     error
		}{
			{
				name:        "シード済みのとき、is_premium を更新する",
				lookupID:    testPlayerID1,
				wantPremium: true,
				wantErr:     nil,
			},
			{
				name:        "未シードのとき、ErrNotFound になる",
				lookupID:    testPlayerID2,
				wantPremium: false,
				wantErr:     port.ErrNotFound,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

				err := repo.UpdatePremium(ctx, tt.lookupID, true, &expiresAt)
				require.ErrorIs(t, err, tt.wantErr)

				got, _ := repo.FindByID(ctx, tt.lookupID)
				assert.Equal(t, tt.wantPremium, playerIsPremium(got))
			})
		}
	})
}

func TestIncrementDailyBattleCount(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	today := civil.DateOf(time.Now().UTC())
	yesterday := today.AddDays(-1)

	type call struct {
		playerID string
		gameDate civil.Date
		want     int64
	}

	t.Run("当日バトル数のインクリメント", func(t *testing.T) {
		// 連続呼び出しの履歴を calls スライスで表し、日単位 UPSERT の独立性 (別日は独立行、
		// 別プレイヤーは独立カウント) を 1 ケース内で通しで確かめる。
		tests := []struct {
			name  string
			calls []call
		}{
			{
				name: "当日の行が無い初回のとき、1 が返る",
				calls: []call{
					{playerID: testPlayerID1, gameDate: today, want: 1},
				},
			},
			{
				name: "同一ゲーム日を繰り返すとき、+1 ずつ加算される",
				calls: []call{
					{playerID: testPlayerID1, gameDate: today, want: 1},
					{playerID: testPlayerID1, gameDate: today, want: 2},
					{playerID: testPlayerID1, gameDate: today, want: 3},
				},
			},
			{
				name: "別ゲーム日のとき、独立した行として 1 から始まる",
				calls: []call{
					{playerID: testPlayerID1, gameDate: today, want: 1},
					{playerID: testPlayerID1, gameDate: yesterday, want: 1},
				},
			},
			{
				name: "既存日に戻るとき、履歴が独立して残り加算が続く",
				calls: []call{
					{playerID: testPlayerID1, gameDate: today, want: 1},
					{playerID: testPlayerID1, gameDate: today, want: 2},
					{playerID: testPlayerID1, gameDate: yesterday, want: 1},
					{playerID: testPlayerID1, gameDate: today, want: 3},
				},
			},
			{
				name: "別プレイヤーのとき、カウントを共有せず独立する",
				calls: []call{
					{playerID: testPlayerID1, gameDate: today, want: 1},
					{playerID: testPlayerID2, gameDate: today, want: 1},
					{playerID: testPlayerID1, gameDate: today, want: 2},
					{playerID: testPlayerID2, gameDate: today, want: 2},
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
				seedPlayer(t, testPlayerID2, "uid-2", "Bob", false)

				for _, c := range tt.calls {
					got, err := repo.IncrementDailyBattleCount(ctx, c.playerID, c.gameDate)
					require.NoError(t, err)
					assert.Equal(t, c.want, got)
				}
			})
		}
	})
}

func TestDecrementDailyBattleCount(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()
	today := civil.DateOf(time.Now().UTC())

	t.Run("当日バトル数の減算", func(t *testing.T) {
		t.Run("カウントが 2 のとき、1 に減算されて true が返る", func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			seedPlayerDailyBattle(t, testPlayerID1, today, 2)

			reverted, err := repo.DecrementDailyBattleCount(ctx, testPlayerID1, today)
			require.NoError(t, err)
			assert.True(t, reverted)

			got, err := repo.GetDailyBattle(ctx, testPlayerID1, today)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, int64(1), got.DailyBattleCount)
		})

		t.Run("カウントが 0 のとき、0 のまま true が返る (負の値にならない)", func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			seedPlayerDailyBattle(t, testPlayerID1, today, 0)

			reverted, err := repo.DecrementDailyBattleCount(ctx, testPlayerID1, today)
			require.NoError(t, err)
			assert.True(t, reverted)

			got, err := repo.GetDailyBattle(ctx, testPlayerID1, today)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, int64(0), got.DailyBattleCount)
		})

		t.Run("対象日の行が無いとき、何も作らず false が返る", func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			reverted, err := repo.DecrementDailyBattleCount(ctx, testPlayerID1, today)
			require.NoError(t, err)
			assert.False(t, reverted)

			got, err := repo.GetDailyBattle(ctx, testPlayerID1, today)
			require.NoError(t, err)
			assert.Nil(t, got)
		})

		t.Run("別ゲーム日の行があるとき、対象日には影響しない", func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)
			yesterday := today.AddDays(-1)
			seedPlayerDailyBattle(t, testPlayerID1, yesterday, 3)

			reverted, err := repo.DecrementDailyBattleCount(ctx, testPlayerID1, today)
			require.NoError(t, err)
			assert.False(t, reverted)

			gotYesterday, err := repo.GetDailyBattle(ctx, testPlayerID1, yesterday)
			require.NoError(t, err)
			require.NotNil(t, gotYesterday)
			assert.Equal(t, int64(3), gotYesterday.DailyBattleCount)
		})
	})
}

func TestUpdateProgression(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	t.Run("進捗の更新", func(t *testing.T) {
		// 永続化は GetProgression 経由で反映されることまで確認する。
		tests := []struct {
			name      string
			lookupID  string
			wantLevel int64
			wantExp   int64
			wantErr   error
		}{
			{
				name:      "シード済みのとき、exp / level をそのまま書き込む",
				lookupID:  testPlayerID1,
				wantLevel: 2,
				wantExp:   150,
				wantErr:   nil,
			},
			{
				name:     "未シードのとき、ErrNotFound になる",
				lookupID: testPlayerID2,
				wantErr:  port.ErrNotFound,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

				prog, err := repo.UpdateProgression(ctx, tt.lookupID, 150, 2)
				require.ErrorIs(t, err, tt.wantErr)
				gotLevel, gotExp := progressionLevelExp(prog)
				assert.Equal(t, tt.wantLevel, gotLevel)
				assert.Equal(t, tt.wantExp, gotExp)

				persisted, _ := repo.GetProgression(ctx, tt.lookupID)
				persistedLevel, persistedExp := progressionLevelExp(persisted)
				assert.Equal(t, tt.wantLevel, persistedLevel)
				assert.Equal(t, tt.wantExp, persistedExp)
			})
		}
	})
}

func TestGetOnboardingStatus(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	t.Run("onboarding_status の取得", func(t *testing.T) {
		tests := []struct {
			name       string
			lookupID   string
			wantStatus string
			wantErr    error
		}{
			{
				name:       "シード直後のとき、not_started を返す",
				lookupID:   testPlayerID1,
				wantStatus: domain.OnboardingStatusNotStarted,
			},
			{
				name:     "未シードのとき、ErrNotFound になる",
				lookupID: testPlayerID2,
				wantErr:  port.ErrNotFound,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

				got, err := repo.GetOnboardingStatus(ctx, tt.lookupID)
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, tt.wantStatus, got)
			})
		}
	})
}

func TestUpdateOnboardingStatus(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	t.Run("onboarding_status の更新", func(t *testing.T) {
		t.Run("name_set に更新すると、取得で name_set が返る", func(t *testing.T) {
			sharedPg.Truncate(t)
			seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

			require.NoError(t, repo.UpdateOnboardingStatus(ctx, testPlayerID1, domain.OnboardingStatusNameSet))

			got, err := repo.GetOnboardingStatus(ctx, testPlayerID1)
			require.NoError(t, err)
			assert.Equal(t, domain.OnboardingStatusNameSet, got)
		})

		t.Run("未シードのとき、ErrNotFound になる", func(t *testing.T) {
			sharedPg.Truncate(t)

			err := repo.UpdateOnboardingStatus(ctx, testPlayerID2, domain.OnboardingStatusNameSet)
			require.ErrorIs(t, err, port.ErrNotFound)
		})
	})
}

func TestGetProgression(t *testing.T) {
	repo := postgres.NewPlayerRepository(sharedPg.Pool)
	ctx := context.Background()

	t.Run("進捗の取得", func(t *testing.T) {
		tests := []struct {
			name      string
			lookupID  string
			wantLevel int64
			wantExp   int64
			wantErr   error
		}{
			{
				name:      "シード済みのとき、現在の level と exp を返す",
				lookupID:  testPlayerID1,
				wantLevel: 1,
				wantExp:   0,
			},
			{
				name:     "未シードのとき、ErrNotFound になる",
				lookupID: testPlayerID2,
				wantErr:  port.ErrNotFound,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				seedPlayer(t, testPlayerID1, "uid-1", "Alice", false)

				got, err := repo.GetProgression(ctx, tt.lookupID)
				gotLevel, gotExp := progressionLevelExp(got)
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, tt.wantLevel, gotLevel)
				assert.Equal(t, tt.wantExp, gotExp)
			})
		}
	})
}
