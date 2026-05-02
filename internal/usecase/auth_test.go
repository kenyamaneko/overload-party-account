//go:build integration

package usecase

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

// newAuthTestService は実 PostgreSQL repository + TxManager を束ねて
// AuthInteractor を返す。DB の truncate は呼び出し側（各 Test が sharedPg.Truncate）で行う。
// gameConfigRepo は exp_formula_coefficient のみを必要とする
// (PlayerResponse 組み立てで派生値計算に使う)。
func newAuthTestService() *AuthInteractor {
	playerRepo, playerViewRepo, _, playerSettingsRepo, tx := newRealRepos()
	gameConfigRepo := newFakeGameConfigRepo(map[string]int64{
		ConfigKeyExpFormulaCoefficient: 60,
	})
	return NewAuthInteractor(playerRepo, playerViewRepo, playerSettingsRepo, gameConfigRepo, tx)
}

func TestAuthInteractor_Register_Success(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name        string
		firebaseUID string
	}{
		{
			name:        "新規登録 1",
			firebaseUID: "firebase-uid-1",
		},
		{
			name:        "新規登録 2",
			firebaseUID: "firebase-uid-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			svc := newAuthTestService()

			player, err := svc.Register(ctx, tt.firebaseUID)
			require.NoError(t, err)
			assert.NotEmpty(t, player.PlayerID)
			assert.Equal(t, tt.firebaseUID, player.FirebaseUID)
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
}

func TestAuthInteractor_Register_DuplicateFirebaseUID_ReturnsAlreadyRegistered(t *testing.T) {
	ctx := context.Background()
	sharedPg.Truncate(t)
	svc := newAuthTestService()

	_, err := svc.Register(ctx, "firebase-uid-dup")
	require.NoError(t, err)

	_, err = svc.Register(ctx, "firebase-uid-dup")
	require.ErrorIs(t, err, ErrPlayerAlreadyRegistered)
}

func TestAuthInteractor_Login_Success(t *testing.T) {
	ctx := context.Background()
	sharedPg.Truncate(t)
	svc := newAuthTestService()

	registered, err := svc.Register(ctx, "firebase-uid-login")
	require.NoError(t, err)

	loggedIn, err := svc.Login(ctx, "firebase-uid-login")
	require.NoError(t, err)
	assert.Equal(t, registered.PlayerID, loggedIn.PlayerID)
	// 直後 Login では name は未設定のまま。
	assert.Nil(t, loggedIn.Name)
}

func TestAuthInteractor_Login_NotFound(t *testing.T) {
	sharedPg.Truncate(t)
	svc := newAuthTestService()

	_, err := svc.Login(context.Background(), "nonexistent-uid")
	require.ErrorIs(t, err, ErrPlayerNotFound)
}

// オンボーディング動線の通し: Register で name 未設定のまま行を作り、
// 後続の UpdateName で初めて表示名が確定する仕様を固定する。
// 「途中でゲームを落とした後の再起動でも Register をやり直さない」設計の
// 基礎が成立していることをここで保証する。
func TestAuthInteractor_RegisterThenUpdateName_OnboardingFlow(t *testing.T) {
	ctx := context.Background()
	sharedPg.Truncate(t)

	authSvc := newAuthTestService()
	registered, err := authSvc.Register(ctx, "firebase-uid-onboard")
	require.NoError(t, err)
	require.Nil(t, registered.Name, "Register 直後は name が nil")

	// オンボーディングシナリオ完了相当の表示名確定。
	playerRepo, playerViewRepo, factionRepo, _, tx := newRealRepos()
	playerSvc := NewPlayerInteractor(playerRepo, playerViewRepo, newFakeGameConfigRepo(map[string]int64{
		ConfigKeyExpFormulaCoefficient: 60,
	}), factionRepo, tx)

	updated, err := playerSvc.UpdateName(ctx, registered.PlayerID, "Alice")
	require.NoError(t, err)
	require.NotNil(t, updated.Name, "UpdateName 後は name が確定する")
	assert.Equal(t, "Alice", *updated.Name)

	// Login で再取得しても確定済みの name が残ることを確認 (永続化されていること)。
	loggedIn, err := authSvc.Login(ctx, "firebase-uid-onboard")
	require.NoError(t, err)
	require.NotNil(t, loggedIn.Name)
	assert.Equal(t, "Alice", *loggedIn.Name)
}
