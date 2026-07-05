//go:build integration

package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

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
			playerSvc := NewPlayerInteractor(playerRepo, playerRepo, playerRepo, playerRepo, playerViewRepo, newFakeGameConfigRepo(map[string]int64{
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
