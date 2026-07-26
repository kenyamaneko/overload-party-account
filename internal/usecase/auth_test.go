//go:build integration

package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// insertFailingSettingsRepo は Insert だけを強制失敗させる port.PlayerSettingsRepo フェイク。
// Register の Tx ロールバックを検証するため既定設定作成の失敗を注入する。他メソッドは
// このテスト経路で呼ばれない前提で埋め込み interface に委譲する。
type insertFailingSettingsRepo struct {
	port.PlayerSettingsRepo
}

func (insertFailingSettingsRepo) Insert(context.Context, *domain.PlayerSettings) error {
	return errors.New("forced insert failure")
}

// newAuthTestInteractor は実 PostgreSQL repository + TxManager を束ねて
// AuthInteractor を返す。DB の truncate は呼び出し側（各 Test が sharedPg.Truncate）で行う。
// gameConfigRepo は exp_formula_coefficient のみを必要とする
// (PlayerResponse 組み立てで派生値計算に使う)。
func newAuthTestInteractor() *AuthInteractor {
	playerRepo, playerViewRepo, _, playerSettingsRepo, tx := newRealRepos()
	gameConfigRepo := newFakeGameConfigRepo(map[string]int64{
		ConfigKeyExpFormulaCoefficient: 60,
	})
	return NewAuthInteractor(playerRepo, playerViewRepo, playerSettingsRepo, gameConfigRepo, tx)
}

func TestRegister(t *testing.T) {
	ctx := context.Background()

	t.Run("プレイヤー登録", func(t *testing.T) {
		successCases := []struct {
			name        string
			firebaseUID string
		}{
			{
				name:        "firebase_uid=firebase-uid-1 で新規登録するとき、name 未設定・level=1 の player が作られる",
				firebaseUID: "firebase-uid-1",
			},
			{
				name:        "firebase_uid=firebase-uid-2 で新規登録するとき、name 未設定・level=1 の player が作られる",
				firebaseUID: "firebase-uid-2",
			},
		}

		for _, tc := range successCases {
			t.Run(tc.name, func(t *testing.T) {
				sharedPg.Truncate(t)
				svc := newAuthTestInteractor()

				player, err := svc.Register(ctx, tc.firebaseUID)
				require.NoError(t, err)
				assert.NotEmpty(t, player.PlayerID)
				assert.Equal(t, tc.firebaseUID, player.FirebaseUID)
				// Register 直後の表示名は nil。オンボーディング完了時に
				// player-onboarded イベント経由で確定する契約。
				assert.Nil(t, player.Name)
				assert.Equal(t, int64(1), player.Level)
				assert.Equal(t, int64(0), player.Exp)
				assert.False(t, player.IsPremium)

				// DB 上でも name が NULL になっていることを確認する (server を信じない明示的検証)。
				var dbName *string
				require.NoError(t, sharedPg.Pool.QueryRow(ctx,
					`SELECT name FROM account.players WHERE player_id = $1`,
					player.PlayerID,
				).Scan(&dbName))
				assert.Nil(t, dbName, "Register は name を NULL で挿入する")

				// Register はトランザクションで player と player_settings をアトミックに作成する。
				// tx の commit を実 DB 行の存在で検証する。
				var count int
				require.NoError(t, sharedPg.Pool.QueryRow(ctx,
					`SELECT COUNT(*) FROM account.player_settings WHERE player_id = $1`,
					player.PlayerID,
				).Scan(&count))
				assert.Equal(t, 1, count, "Register はデフォルトの player_settings を作成する")

				var language string
				var bgm, se int64
				var push bool
				require.NoError(t, sharedPg.Pool.QueryRow(ctx,
					`SELECT language, bgm_volume, se_volume, push_enabled
					 FROM account.player_settings WHERE player_id = $1`,
					player.PlayerID,
				).Scan(&language, &bgm, &se, &push))
				assert.Equal(t, domain.DefaultLanguage, language)
				assert.Equal(t, domain.DefaultBgmVolume, bgm)
				assert.Equal(t, domain.DefaultSeVolume, se)
				assert.Equal(t, domain.DefaultPushEnabled, push)
			})
		}

		t.Run("同一 firebase_uid で再登録するとき、ErrPlayerAlreadyRegistered になる", func(t *testing.T) {
			sharedPg.Truncate(t)
			svc := newAuthTestInteractor()

			_, err := svc.Register(ctx, "firebase-uid-dup")
			require.NoError(t, err)

			_, err = svc.Register(ctx, "firebase-uid-dup")
			require.ErrorIs(t, err, ErrPlayerAlreadyRegistered)
		})

		t.Run("既定設定の作成に失敗すると、エラーになりプレイヤー行も進捗行も残らない", func(t *testing.T) {
			sharedPg.Truncate(t)
			playerRepo, playerViewRepo, _, _, tx := newRealRepos()
			svc := NewAuthInteractor(playerRepo, playerViewRepo, insertFailingSettingsRepo{}, newFakeGameConfigRepo(nil), tx)

			_, err := svc.Register(ctx, "firebase-uid-rollback")
			require.Error(t, err)

			var playerCount, progressionCount int
			require.NoError(t, sharedPg.Pool.QueryRow(ctx,
				`SELECT COUNT(*) FROM account.players WHERE firebase_uid = $1`, "firebase-uid-rollback",
			).Scan(&playerCount))
			require.NoError(t, sharedPg.Pool.QueryRow(ctx,
				`SELECT COUNT(*) FROM account.player_progression`,
			).Scan(&progressionCount))
			assert.Equal(t, 0, playerCount)
			assert.Equal(t, 0, progressionCount)
		})
	})
}

func TestLogin(t *testing.T) {
	t.Run("プレイヤーログイン", func(t *testing.T) {
		t.Run("登録済み firebase_uid でログインするとき、同じ player を返す", func(t *testing.T) {
			ctx := context.Background()
			sharedPg.Truncate(t)
			svc := newAuthTestInteractor()

			registered, err := svc.Register(ctx, "firebase-uid-login")
			require.NoError(t, err)

			loggedIn, err := svc.Login(ctx, "firebase-uid-login")
			require.NoError(t, err)
			assert.Equal(t, registered.PlayerID, loggedIn.PlayerID)
			// 直後 Login では name は未設定のまま。
			assert.Nil(t, loggedIn.Name)
		})

		t.Run("未登録 firebase_uid でログインするとき、ErrPlayerNotFound になる", func(t *testing.T) {
			sharedPg.Truncate(t)
			svc := newAuthTestInteractor()

			_, err := svc.Login(context.Background(), "nonexistent-uid")
			require.ErrorIs(t, err, ErrPlayerNotFound)
		})

		t.Run("ログイン応答の組み立て時に係数が読めないとき、エラーになる", func(t *testing.T) {
			ctx := context.Background()
			sharedPg.Truncate(t)
			playerRepo, playerViewRepo, _, playerSettingsRepo, tx := newRealRepos()
			registerSvc := NewAuthInteractor(playerRepo, playerViewRepo, playerSettingsRepo, newFakeGameConfigRepo(map[string]int64{
				ConfigKeyExpFormulaCoefficient: 60,
			}), tx)
			_, err := registerSvc.Register(ctx, "firebase-uid-no-coeff")
			require.NoError(t, err)

			svc := NewAuthInteractor(playerRepo, playerViewRepo, playerSettingsRepo, newFakeGameConfigRepo(nil), tx)
			_, err = svc.Login(ctx, "firebase-uid-no-coeff")
			require.ErrorIs(t, err, port.ErrNotFound)
		})
	})
}

func TestFindByFirebaseUID(t *testing.T) {
	ctx := context.Background()

	t.Run("firebase_uid によるプレイヤー参照", func(t *testing.T) {
		t.Run("登録済みの firebase_uid で参照すると、そのプレイヤーが返る", func(t *testing.T) {
			sharedPg.Truncate(t)
			svc := newAuthTestInteractor()
			registered, err := svc.Register(ctx, "firebase-uid-find")
			require.NoError(t, err)

			found, err := svc.FindByFirebaseUID(ctx, "firebase-uid-find")
			require.NoError(t, err)
			assert.Equal(t, registered.PlayerID, found.PlayerID)
		})

		t.Run("未登録の firebase_uid で参照すると、port.ErrNotFound になる", func(t *testing.T) {
			sharedPg.Truncate(t)
			svc := newAuthTestInteractor()

			_, err := svc.FindByFirebaseUID(ctx, "firebase-uid-missing")
			require.ErrorIs(t, err, port.ErrNotFound)
		})
	})
}

func TestRegisterThenUpdateName(t *testing.T) {
	t.Run("オンボーディング動線 (Register→UpdateName→Login)", func(t *testing.T) {
		t.Run("Register で name 未設定の行を作り、UpdateName で表示名が確定して Login 後も残る", func(t *testing.T) {
			// 途中でゲームを落として再起動しても Register をやり直さない設計の基礎として、
			// Register が name 未設定の行を作り、確定は後続の UpdateName に委ねることを固定する。
			ctx := context.Background()
			sharedPg.Truncate(t)

			authSvc := newAuthTestInteractor()
			registered, err := authSvc.Register(ctx, "firebase-uid-onboard")
			require.NoError(t, err)
			require.Nil(t, registered.Name, "Register 直後は name が nil")

			// オンボーディングシナリオ完了相当の表示名確定。
			playerRepo, playerViewRepo, _, _, tx := newRealRepos()
			playerSvc := NewPlayerInteractor(playerRepo, playerRepo, playerRepo, playerRepo, newBattleCountReversalRepo(), playerViewRepo, newFakeGameConfigRepo(map[string]int64{
				ConfigKeyExpFormulaCoefficient: 60,
			}), tx)

			updated, err := playerSvc.UpdateName(ctx, registered.PlayerID, "Alice")
			require.NoError(t, err)
			require.NotNil(t, updated.Name, "UpdateName 後は name が確定する")
			assert.Equal(t, "Alice", *updated.Name)

			// Login で再取得しても確定済みの name が残ることを確認 (永続化されていること)。
			loggedIn, err := authSvc.Login(ctx, "firebase-uid-onboard")
			require.NoError(t, err)
			require.NotNil(t, loggedIn.Name)
			assert.Equal(t, "Alice", *loggedIn.Name)
		})
	})
}
