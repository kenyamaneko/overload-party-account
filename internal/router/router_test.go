package router

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/handler/rest"

	internalauth "github.com/kenyamaneko/overload-party-gateway/packages/internalauth-go"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// fakeRouterVerifier は router 単体テスト用の internalauth.Verifier 最小 fake。
type fakeRouterVerifier struct {
	playerID string
	err      error
}

func (f fakeRouterVerifier) Verify(string) (string, error) {
	return f.playerID, f.err
}

// nullVerifier は auth 経路を通らないルートで Verify が呼ばれてはならないことを示す。
type nullVerifier struct{}

func (nullVerifier) Verify(string) (string, error) {
	panic("Verify should not be called for routes outside /api/v1/account")
}

func newTestRouter(verifier internalauth.Verifier) *gin.Engine {
	return New(
		rest.NewAuthHandler(nil),
		rest.NewPlayerHandler(nil),
		rest.NewFactionHandler(nil),
		rest.NewPlayerSettingsHandler(nil),
		verifier,
	)
}

// /health は auth middleware を通らず常に 200 を返す。
func TestNew_HealthEndpoint(t *testing.T) {
	r := newTestRouter(nullVerifier{})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

// /internal/v1/auth/* (bootstrap 系) と /internal/v1/players/award-game-exp は auth
// middleware を通らない。nullVerifier が呼ばれた場合 panic するため、NotPanics +
// 401 でないことの両方で確認する。
func TestNew_InternalRoutesAreAuthFree(t *testing.T) {
	cases := []struct {
		name   string
		method string
		path   string
	}{
		{name: "auth/register は auth-free", method: http.MethodPost, path: "/internal/v1/auth/register"},
		{name: "auth/login は auth-free", method: http.MethodPost, path: "/internal/v1/auth/login"},
		{name: "auth/by-firebase-uid は auth-free", method: http.MethodGet, path: "/internal/v1/auth/by-firebase-uid/uid-1"},
		{name: "players/:playerID/factions は auth-free", method: http.MethodGet, path: "/internal/v1/players/p1/factions"},
		{name: "players/award-game-exp は auth-free", method: http.MethodPost, path: "/internal/v1/players/award-game-exp"},
	}

	r := newTestRouter(nullVerifier{})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			require.NotPanics(t, func() {
				r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			})
			assert.NotEqual(t, http.StatusUnauthorized, w.Code)
		})
	}
}

// /api/v1/account 配下は auth middleware を通る。
// header 欠落時は 401 を返し、handler に到達しないことを確認する。
func TestNew_ApiRouteRequiresInternalAuth(t *testing.T) {
	r := newTestRouter(fakeRouterVerifier{playerID: "irrelevant"})

	cases := []struct {
		name   string
		method string
		path   string
	}{
		{name: "GET /me は auth header 欠落で 401", method: http.MethodGet, path: "/api/v1/account/me"},
		{name: "GET /me/factions は auth header 欠落で 401", method: http.MethodGet, path: "/api/v1/account/me/factions"},
		{name: "GET /me/settings は auth header 欠落で 401", method: http.MethodGet, path: "/api/v1/account/me/settings"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	}
}

// verifier が error を返すと 401 を返し handler に到達しない。
func TestNew_ApiRouteRejectsVerifierError(t *testing.T) {
	r := newTestRouter(fakeRouterVerifier{err: errors.New("invalid token")})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/account/me", nil)
	req.Header.Set(internalauth.HeaderName, "any.token")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
