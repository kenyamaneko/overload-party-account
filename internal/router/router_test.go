package router

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

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
	t.Run("ルートごとの認証要否", func(t *testing.T) {
		t.Run("/health は認証なしで 200 を返す", func(t *testing.T) {
			r := newTestRouter(nullVerifier{})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
			assert.Equal(t, http.StatusOK, w.Code)
		})

		t.Run("認証必須ルートは認証ヘッダが無いと 401 になる", func(t *testing.T) {
			r := newTestRouter(fakeRouterVerifier{playerID: "irrelevant"})

			cases := []struct {
				name   string
				method string
				path   string
			}{
				{name: "GET /api/v1/account/me は 401 になる", method: http.MethodGet, path: "/api/v1/account/me"},
				{name: "GET /api/v1/account/me/factions は 401 になる", method: http.MethodGet, path: "/api/v1/account/me/factions"},
				{name: "GET /api/v1/account/me/settings は 401 になる", method: http.MethodGet, path: "/api/v1/account/me/settings"},
			}

			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					w := httptest.NewRecorder()
					r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
					assert.Equal(t, http.StatusUnauthorized, w.Code)
				})
			}
		})

		t.Run("認証に失敗すると、401 になる", func(t *testing.T) {
			r := newTestRouter(fakeRouterVerifier{err: errors.New("invalid token")})
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/account/me", nil)
			req.Header.Set(internalauth.HeaderName, "any.token")
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
		})
	})
}
