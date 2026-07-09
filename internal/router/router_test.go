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

func TestNew(t *testing.T) {
	t.Run("ルーターの認証配線", func(t *testing.T) {
		t.Run("/health は auth middleware を通らず 200 を返す", func(t *testing.T) {
			r := newTestRouter(nullVerifier{})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
			assert.Equal(t, http.StatusOK, w.Code)
		})

		t.Run("auth-free ルートは Verify を呼ばず 401 にならない", func(t *testing.T) {
			// nullVerifier は呼ばれると panic するので、NotPanics で「auth middleware を
			// 通らない」ことを、401 でないことで「認証が要求されない」ことを確かめる。
			cases := []struct {
				name   string
				method string
				path   string
			}{
				{name: "POST /internal/v1/auth/register は認証なしで到達できる", method: http.MethodPost, path: "/internal/v1/auth/register"},
				{name: "POST /internal/v1/auth/login は認証なしで到達できる", method: http.MethodPost, path: "/internal/v1/auth/login"},
				{name: "GET /internal/v1/auth/by-firebase-uid/:uid は認証なしで到達できる", method: http.MethodGet, path: "/internal/v1/auth/by-firebase-uid/uid-1"},
				{name: "GET /internal/v1/players/:playerID/factions は認証なしで到達できる", method: http.MethodGet, path: "/internal/v1/players/p1/factions"},
				{name: "POST /internal/v1/players/award-game-exp は認証なしで到達できる", method: http.MethodPost, path: "/internal/v1/players/award-game-exp"},
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
		})

		t.Run("/api/v1/account 配下は auth header 欠落で 401 になる", func(t *testing.T) {
			r := newTestRouter(fakeRouterVerifier{playerID: "irrelevant"})

			cases := []struct {
				name   string
				method string
				path   string
			}{
				{name: "GET /api/v1/account/me は auth header 欠落で 401 になる", method: http.MethodGet, path: "/api/v1/account/me"},
				{name: "GET /api/v1/account/me/factions は auth header 欠落で 401 になる", method: http.MethodGet, path: "/api/v1/account/me/factions"},
				{name: "GET /api/v1/account/me/settings は auth header 欠落で 401 になる", method: http.MethodGet, path: "/api/v1/account/me/settings"},
			}

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					w := httptest.NewRecorder()
					r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
					assert.Equal(t, http.StatusUnauthorized, w.Code)
				})
			}
		})

		t.Run("verifier がエラーを返すとき、401 になる", func(t *testing.T) {
			r := newTestRouter(fakeRouterVerifier{err: errors.New("invalid token")})
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/account/me", nil)
			req.Header.Set(internalauth.HeaderName, "any.token")
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	})
}
