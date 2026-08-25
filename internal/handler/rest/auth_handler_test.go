//go:build integration

package rest_test

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

func TestAuthHandler_Register(t *testing.T) {
	t.Run("POST /internal/v1/auth/register", func(t *testing.T) {
		t.Run("firebase_uidが空のとき、400を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())

			w := doJSON(r, "POST", "/internal/v1/auth/register", apiaccount.RegisterRequest{FirebaseUID: ""}, nil)

			assert.Equal(t, 400, w.Code)
		})

		t.Run("指定したfirebase_uidが未登録のとき、新規プレイヤーを作成し、201とプレイヤー情報を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())

			w := doJSON(r, "POST", "/internal/v1/auth/register", apiaccount.RegisterRequest{FirebaseUID: "firebase-" + uuid.NewString()}, nil)

			require.Equal(t, 201, w.Code)
			var resp apiaccount.PlayerResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, int64(1), resp.Level)
			assert.Equal(t, int64(0), resp.Exp)
			assert.Equal(t, apiaccount.OnboardingStatusNotStarted, resp.OnboardingStatus)
			assert.Nil(t, resp.Name)
			assert.Nil(t, resp.InitialFaction)
			assert.False(t, resp.IsPremium)
		})

		t.Run("同一firebase_uidで既に登録済みのとき、409を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())
			firebaseUID := "firebase-" + uuid.NewString()
			w1 := doJSON(r, "POST", "/internal/v1/auth/register", apiaccount.RegisterRequest{FirebaseUID: firebaseUID}, nil)
			require.Equal(t, 201, w1.Code)

			w2 := doJSON(r, "POST", "/internal/v1/auth/register", apiaccount.RegisterRequest{FirebaseUID: firebaseUID}, nil)

			assert.Equal(t, 409, w2.Code)
		})
	})
}

func TestAuthHandler_Login(t *testing.T) {
	t.Run("POST /internal/v1/auth/login", func(t *testing.T) {
		t.Run("firebase_uidが空のとき、400を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())

			w := doJSON(r, "POST", "/internal/v1/auth/login", apiaccount.LoginRequest{FirebaseUID: ""}, nil)

			assert.Equal(t, 400, w.Code)
		})

		t.Run("指定したfirebase_uidのプレイヤーが存在するとき、200とプレイヤー情報を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())
			registered := registerPlayer(t, r)

			w := doJSON(r, "POST", "/internal/v1/auth/login", apiaccount.LoginRequest{FirebaseUID: registered.FirebaseUID}, nil)

			require.Equal(t, 200, w.Code)
			var resp apiaccount.PlayerResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, registered.PlayerID, resp.PlayerID)
		})

		t.Run("指定したfirebase_uidのプレイヤーが存在しないとき、404を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())

			w := doJSON(r, "POST", "/internal/v1/auth/login", apiaccount.LoginRequest{FirebaseUID: "missing-" + uuid.NewString()}, nil)

			assert.Equal(t, 404, w.Code)
		})
	})
}

func TestAuthHandler_GetPlayerByFirebaseUID(t *testing.T) {
	t.Run("GET /internal/v1/auth/by-firebase-uid/{firebaseUID}", func(t *testing.T) {
		t.Run("指定したfirebaseUIDのプレイヤーが存在するとき、200とプレイヤー情報を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())
			registered := registerPlayer(t, r)

			w := doJSON(r, "GET", "/internal/v1/auth/by-firebase-uid/"+registered.FirebaseUID, nil, nil)

			require.Equal(t, 200, w.Code)
			var resp apiaccount.PlayerResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			assert.Equal(t, registered.PlayerID, resp.PlayerID)
		})

		t.Run("指定したfirebaseUIDのプレイヤーが存在しないとき、404を返す", func(t *testing.T) {
			r, _ := newTestRouter(t, validGameConfigValues())

			w := doJSON(r, "GET", "/internal/v1/auth/by-firebase-uid/missing-"+uuid.NewString(), nil, nil)

			assert.Equal(t, 404, w.Code)
		})
	})
}
