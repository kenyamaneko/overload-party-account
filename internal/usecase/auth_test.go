//go:build integration

package usecase_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

func TestAuthInteractor_Register(t *testing.T) {
	t.Run("AuthInteractor", func(t *testing.T) {
		t.Run("Register", func(t *testing.T) {
			t.Run("同一firebase_uidで既に登録済みのとき、ErrPlayerAlreadyRegisteredを返す", func(t *testing.T) {
				interactor := newTestAuthInteractor(t)
				_, err := interactor.Register(context.Background(), "firebase-dup")
				require.NoError(t, err)

				_, err = interactor.Register(context.Background(), "firebase-dup")

				assert.ErrorIs(t, err, usecase.ErrPlayerAlreadyRegistered)
			})

			t.Run("未登録のfirebase_uidを渡したとき、新規プレイヤーを作成する", func(t *testing.T) {
				interactor := newTestAuthInteractor(t)

				resp, err := interactor.Register(context.Background(), "firebase-new")

				require.NoError(t, err)
				assert.Equal(t, int64(1), resp.Level)
				assert.Equal(t, int64(0), resp.Exp)
				assert.Equal(t, apiaccount.OnboardingStatusNotStarted, resp.OnboardingStatus)
				assert.Nil(t, resp.Name)
				assert.Nil(t, resp.InitialFaction)
				assert.False(t, resp.IsPremium)
			})

			t.Run("新規プレイヤーの作成と同時に、既定値のプレイヤー設定を作成する", func(t *testing.T) {
				interactor := newTestAuthInteractor(t)
				resp, err := interactor.Register(context.Background(), "firebase-new")
				require.NoError(t, err)

				settingsInteractor := usecase.NewPlayerSettingsInteractor(postgres.NewPlayerSettingsRepository(sharedPg.Pool))
				settings, err := settingsInteractor.Get(context.Background(), resp.PlayerID)

				require.NoError(t, err)
				assert.Equal(t, "ja", settings.Language)
				assert.Equal(t, int64(50), settings.BgmVolume)
				assert.Equal(t, int64(50), settings.SeVolume)
				assert.True(t, settings.PushEnabled)
			})
		})
	})
}

func TestAuthInteractor_FindByFirebaseUID(t *testing.T) {
	t.Run("AuthInteractor", func(t *testing.T) {
		t.Run("FindByFirebaseUID", func(t *testing.T) {
			t.Run("指定したfirebase_uidのプレイヤーが存在するとき、プレイヤー情報を返す", func(t *testing.T) {
				interactor := newTestAuthInteractor(t)
				registered, err := interactor.Register(context.Background(), "firebase-find")
				require.NoError(t, err)

				resp, err := interactor.FindByFirebaseUID(context.Background(), "firebase-find")

				require.NoError(t, err)
				assert.Equal(t, registered.PlayerID, resp.PlayerID)
			})

			t.Run("存在しないとき、見つからないことを示すエラーを返す", func(t *testing.T) {
				interactor := newTestAuthInteractor(t)

				_, err := interactor.FindByFirebaseUID(context.Background(), "firebase-missing")

				assert.ErrorIs(t, err, usecase.ErrNotFound)
			})
		})
	})
}

func TestAuthInteractor_Login(t *testing.T) {
	t.Run("AuthInteractor", func(t *testing.T) {
		t.Run("Login", func(t *testing.T) {
			t.Run("指定したfirebase_uidのプレイヤーが存在するとき、プレイヤー情報を返す", func(t *testing.T) {
				interactor := newTestAuthInteractor(t)
				registered, err := interactor.Register(context.Background(), "firebase-login")
				require.NoError(t, err)

				resp, err := interactor.Login(context.Background(), "firebase-login")

				require.NoError(t, err)
				assert.Equal(t, registered.PlayerID, resp.PlayerID)
			})

			t.Run("存在しないとき、ErrPlayerNotFoundを返す(FindByFirebaseUIDとは異なるエラー値になる)", func(t *testing.T) {
				interactor := newTestAuthInteractor(t)

				_, err := interactor.Login(context.Background(), "firebase-missing")

				assert.ErrorIs(t, err, usecase.ErrPlayerNotFound)
			})
		})
	})
}
