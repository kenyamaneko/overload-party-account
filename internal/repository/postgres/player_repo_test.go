//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
)

func TestPlayerRepository_ReferenceMethodsNotFound(t *testing.T) {
	t.Run("PlayerRepository", func(t *testing.T) {
		t.Run("参照系メソッドに共通する仕様", func(t *testing.T) {
			t.Run("存在しないplayer_idでプレイヤーを取得したとき、見つからないことを示すエラーを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewPlayerRepository(sharedPg.Pool)

				_, err := repo.FindByID(context.Background(), uuid.NewString())

				assert.ErrorIs(t, err, port.ErrNotFound)
			})

			t.Run("存在しないfirebase_uidでプレイヤーを取得したとき、見つからないことを示すエラーを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewPlayerRepository(sharedPg.Pool)

				_, err := repo.FindByFirebaseUID(context.Background(), "missing-firebase-uid")

				assert.ErrorIs(t, err, port.ErrNotFound)
			})

			t.Run("存在しないplayer_idでレベルと累計経験値を取得したとき、見つからないことを示すエラーを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewPlayerRepository(sharedPg.Pool)

				_, err := repo.GetProgression(context.Background(), uuid.NewString())

				assert.ErrorIs(t, err, port.ErrNotFound)
			})

			t.Run("存在しないplayer_idでレベルと累計経験値を更新用に取得したとき、見つからないことを示すエラーを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewPlayerRepository(sharedPg.Pool)

				_, err := repo.GetProgressionForUpdate(context.Background(), uuid.NewString())

				assert.ErrorIs(t, err, port.ErrNotFound)
			})

			t.Run("存在しないplayer_idでオンボーディング状態を取得したとき、見つからないことを示すエラーを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewPlayerRepository(sharedPg.Pool)

				_, err := repo.GetOnboardingStatus(context.Background(), uuid.NewString())

				assert.ErrorIs(t, err, port.ErrNotFound)
			})
		})
	})
}

func TestPlayerRepository_UpdateMethodsNotFound(t *testing.T) {
	t.Run("PlayerRepository", func(t *testing.T) {
		t.Run("更新系メソッドに共通する仕様", func(t *testing.T) {
			t.Run("存在しないplayer_idで表示名を更新しようとしたとき、見つからないことを示すエラーを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewPlayerRepository(sharedPg.Pool)

				err := repo.UpdateName(context.Background(), uuid.NewString(), "名前")

				assert.ErrorIs(t, err, port.ErrNotFound)
			})

			t.Run("存在しないplayer_idでプレミアムステータスを更新しようとしたとき、見つからないことを示すエラーを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewPlayerRepository(sharedPg.Pool)

				err := repo.UpdatePremium(context.Background(), uuid.NewString(), true, nil)

				assert.ErrorIs(t, err, port.ErrNotFound)
			})

			t.Run("存在しないplayer_idでレベルと累計経験値を更新しようとしたとき、見つからないことを示すエラーを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewPlayerRepository(sharedPg.Pool)

				_, err := repo.UpdateProgression(context.Background(), uuid.NewString(), 100, 2)

				assert.ErrorIs(t, err, port.ErrNotFound)
			})

			t.Run("存在しないplayer_idでオンボーディング状態を更新しようとしたとき、見つからないことを示すエラーを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewPlayerRepository(sharedPg.Pool)

				err := repo.UpdateOnboardingStatus(context.Background(), uuid.NewString(), "name_set")

				assert.ErrorIs(t, err, port.ErrNotFound)
			})
		})
	})
}

func TestPlayerRepository_Create(t *testing.T) {
	t.Run("PlayerRepository", func(t *testing.T) {
		t.Run("Create", func(t *testing.T) {
			t.Run("指定したplayerの行と、レベル1・経験値0のprogression行を1回の呼び出しで両方作成する", func(t *testing.T) {
				sharedPg.Truncate(t)
				player := createTestPlayer(t)
				repo := postgres.NewPlayerRepository(sharedPg.Pool)

				found, err := repo.FindByID(context.Background(), player.PlayerID)
				require.NoError(t, err)
				assert.Equal(t, player.FirebaseUID, found.FirebaseUID)

				progression, err := repo.GetProgression(context.Background(), player.PlayerID)
				require.NoError(t, err)
				assert.Equal(t, int64(1), progression.Level)
				assert.Equal(t, int64(0), progression.Exp)
			})
		})
	})
}

func TestPlayerRepository_Exists(t *testing.T) {
	t.Run("PlayerRepository", func(t *testing.T) {
		t.Run("Exists", func(t *testing.T) {
			t.Run("対象が存在するとき、trueを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				player := createTestPlayer(t)
				repo := postgres.NewPlayerRepository(sharedPg.Pool)

				exists, err := repo.Exists(context.Background(), player.PlayerID)

				require.NoError(t, err)
				assert.True(t, exists)
			})

			t.Run("対象が存在しないとき、エラーにはせずfalseを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewPlayerRepository(sharedPg.Pool)

				exists, err := repo.Exists(context.Background(), uuid.NewString())

				require.NoError(t, err)
				assert.False(t, exists)
			})
		})
	})
}

func TestPlayerRepository_ExistsByFirebaseUID(t *testing.T) {
	t.Run("PlayerRepository", func(t *testing.T) {
		t.Run("ExistsByFirebaseUID", func(t *testing.T) {
			t.Run("対象が存在するとき、trueを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				player := createTestPlayer(t)
				repo := postgres.NewPlayerRepository(sharedPg.Pool)

				exists, err := repo.ExistsByFirebaseUID(context.Background(), player.FirebaseUID)

				require.NoError(t, err)
				assert.True(t, exists)
			})

			t.Run("対象が存在しないとき、エラーにはせずfalseを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				repo := postgres.NewPlayerRepository(sharedPg.Pool)

				exists, err := repo.ExistsByFirebaseUID(context.Background(), "missing-firebase-uid")

				require.NoError(t, err)
				assert.False(t, exists)
			})
		})
	})
}

func TestPlayerRepository_GetDailyBattle(t *testing.T) {
	t.Run("PlayerRepository", func(t *testing.T) {
		t.Run("GetDailyBattle", func(t *testing.T) {
			t.Run("指定したplayer_idとgame_dateの組に一致する行が無いとき、エラーにはせずnilを返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				player := createTestPlayer(t)
				repo := postgres.NewPlayerRepository(sharedPg.Pool)

				got, err := repo.GetDailyBattle(context.Background(), player.PlayerID, civil.Date{Year: 2026, Month: 1, Day: 1})

				require.NoError(t, err)
				assert.Nil(t, got)
			})

			t.Run("指定したplayer_idとgame_dateの組に一致する行だけを返す(別の日付の記録があっても混同しない)", func(t *testing.T) {
				sharedPg.Truncate(t)
				player := createTestPlayer(t)
				repo := postgres.NewPlayerRepository(sharedPg.Pool)
				day1 := civil.Date{Year: 2026, Month: 1, Day: 1}
				day2 := civil.Date{Year: 2026, Month: 1, Day: 2}
				_, err := repo.IncrementDailyBattleCount(context.Background(), player.PlayerID, day1)
				require.NoError(t, err)
				_, err = repo.IncrementDailyBattleCount(context.Background(), player.PlayerID, day2)
				require.NoError(t, err)
				_, err = repo.IncrementDailyBattleCount(context.Background(), player.PlayerID, day2)
				require.NoError(t, err)

				got, err := repo.GetDailyBattle(context.Background(), player.PlayerID, day1)

				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Equal(t, int64(1), got.DailyBattleCount)
			})
		})
	})
}

func TestPlayerRepository_IncrementDailyBattleCount(t *testing.T) {
	t.Run("PlayerRepository", func(t *testing.T) {
		t.Run("IncrementDailyBattleCount", func(t *testing.T) {
			t.Run("対象日の記録が無いとき、新規に記録を作成しカウントを1として返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				player := createTestPlayer(t)
				repo := postgres.NewPlayerRepository(sharedPg.Pool)

				count, err := repo.IncrementDailyBattleCount(context.Background(), player.PlayerID, civil.Date{Year: 2026, Month: 1, Day: 1})

				require.NoError(t, err)
				assert.Equal(t, int64(1), count)
			})

			t.Run("対象日の記録が既にあるとき、カウントを1加算した値を返す", func(t *testing.T) {
				sharedPg.Truncate(t)
				player := createTestPlayer(t)
				repo := postgres.NewPlayerRepository(sharedPg.Pool)
				day := civil.Date{Year: 2026, Month: 1, Day: 1}
				_, err := repo.IncrementDailyBattleCount(context.Background(), player.PlayerID, day)
				require.NoError(t, err)

				count, err := repo.IncrementDailyBattleCount(context.Background(), player.PlayerID, day)

				require.NoError(t, err)
				assert.Equal(t, int64(2), count)
			})
		})
	})
}

func TestPlayerRepository_DecrementDailyBattleCount(t *testing.T) {
	t.Run("PlayerRepository", func(t *testing.T) {
		t.Run("対象日のカウントが1以上のとき、1減算する", func(t *testing.T) {
			sharedPg.Truncate(t)
			player := createTestPlayer(t)
			repo := postgres.NewPlayerRepository(sharedPg.Pool)
			day := civil.Date{Year: 2026, Month: 1, Day: 1}
			_, err := repo.IncrementDailyBattleCount(context.Background(), player.PlayerID, day)
			require.NoError(t, err)

			ok, err := repo.DecrementDailyBattleCount(context.Background(), player.PlayerID, day)

			require.NoError(t, err)
			assert.True(t, ok)
			got, err := repo.GetDailyBattle(context.Background(), player.PlayerID, day)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, int64(0), got.DailyBattleCount)
		})

		t.Run("対象日のカウントが既に0のとき、減算してもマイナスにならず0のままになる", func(t *testing.T) {
			sharedPg.Truncate(t)
			player := createTestPlayer(t)
			repo := postgres.NewPlayerRepository(sharedPg.Pool)
			day := civil.Date{Year: 2026, Month: 1, Day: 1}
			_, err := repo.IncrementDailyBattleCount(context.Background(), player.PlayerID, day)
			require.NoError(t, err)
			_, err = repo.DecrementDailyBattleCount(context.Background(), player.PlayerID, day)
			require.NoError(t, err)

			_, err = repo.DecrementDailyBattleCount(context.Background(), player.PlayerID, day)

			require.NoError(t, err)
			got, err := repo.GetDailyBattle(context.Background(), player.PlayerID, day)
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Equal(t, int64(0), got.DailyBattleCount)
		})

		t.Run("対象日の記録が無いとき、何も変更せずfalseを返す", func(t *testing.T) {
			sharedPg.Truncate(t)
			player := createTestPlayer(t)
			repo := postgres.NewPlayerRepository(sharedPg.Pool)

			ok, err := repo.DecrementDailyBattleCount(context.Background(), player.PlayerID, civil.Date{Year: 2026, Month: 1, Day: 1})

			require.NoError(t, err)
			assert.False(t, ok)
		})
	})
}
