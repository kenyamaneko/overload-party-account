package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/repository"
)

func newAuthTestService() *AuthService {
	playerRepo := repository.NewMockPlayerRepository()
	userSettingsRepo := repository.NewMockUserSettingsRepository()
	return NewAuthService(playerRepo, userSettingsRepo, &repository.MockTxRunner{})
}

func TestRegister_ReturnsPlayerWithCorrectFields(t *testing.T) {
	svc := newAuthTestService()

	player, err := svc.Register(context.Background(), "firebase-uid-1", "TestUser")

	require.NoError(t, err)
	assert.NotEmpty(t, player.PlayerID)
	assert.Equal(t, "firebase-uid-1", player.FirebaseUID)
	assert.Equal(t, "TestUser", player.Username)
	assert.Equal(t, int64(1), player.Level)
	assert.Equal(t, int64(0), player.Exp)
	assert.False(t, player.IsPremium)
}

func TestRegister_ThenLoginSucceeds(t *testing.T) {
	svc := newAuthTestService()

	registered, err := svc.Register(context.Background(), "firebase-uid-login", "LoginUser")
	require.NoError(t, err)

	loggedIn, err := svc.Login(context.Background(), "firebase-uid-login")
	require.NoError(t, err)
	assert.Equal(t, registered.PlayerID, loggedIn.PlayerID)
	assert.Equal(t, "LoginUser", loggedIn.Username)
}

func TestRegister_AlreadyRegistered(t *testing.T) {
	svc := newAuthTestService()

	_, err := svc.Register(context.Background(), "firebase-uid-dup", "FirstUser")
	require.NoError(t, err)

	_, err = svc.Register(context.Background(), "firebase-uid-dup", "SecondUser")
	require.ErrorIs(t, err, ErrPlayerAlreadyRegistered)
}

func TestLogin_PlayerNotFound(t *testing.T) {
	svc := newAuthTestService()

	_, err := svc.Login(context.Background(), "nonexistent-uid")
	require.ErrorIs(t, err, ErrPlayerNotFound)
}
