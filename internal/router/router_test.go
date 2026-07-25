package router

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush"
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

// noopMessageHandler は router 単体テスト用の port.MessageHandler 最小 stub。
func noopMessageHandler(context.Context, []byte) error { return nil }

func newTestPubsubHandlers() pubsubpush.Handlers {
	return pubsubpush.Handlers{
		FactionAcquired:      pubsubpush.NewEventHandler(noopMessageHandler),
		PremiumUpdated:       pubsubpush.NewEventHandler(noopMessageHandler),
		PlayerOnboarded:      pubsubpush.NewEventHandler(noopMessageHandler),
		OnboardingNameSet:    pubsubpush.NewEventHandler(noopMessageHandler),
		OnboardingFactionSet: pubsubpush.NewEventHandler(noopMessageHandler),
	}
}

func newTestRouter(verifier internalauth.Verifier) *gin.Engine {
	return New(
		rest.NewAuthHandler(nil),
		rest.NewPlayerHandler(nil),
		rest.NewFactionHandler(nil),
		rest.NewPlayerSettingsHandler(nil),
		verifier,
		newTestPubsubHandlers(),
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

		t.Run("Pub/Sub push の受け口は JWT 認証を経ずに実際の受け口に到達する", func(t *testing.T) {
			r := newTestRouter(nullVerifier{})

			paths := []string{
				"/internal/v1/pubsub/faction-acquired",
				"/internal/v1/pubsub/premium-updated",
				"/internal/v1/pubsub/player-onboarded",
				"/internal/v1/pubsub/onboarding-name-set",
				"/internal/v1/pubsub/onboarding-faction-set",
			}

			for _, path := range paths {
				t.Run("POST "+path+" は 200 を返す", func(t *testing.T) {
					data := base64.StdEncoding.EncodeToString([]byte(`{"event_type":"noop"}`))
					body := bytes.NewReader([]byte(`{"message":{"data":"` + data + `","messageId":"m-1"},"subscription":"test-sub"}`))
					req := httptest.NewRequest(http.MethodPost, path, body)
					req.Header.Set("Content-Type", "application/json")
					w := httptest.NewRecorder()
					r.ServeHTTP(w, req)
					require.Equal(t, http.StatusOK, w.Code)
				})
			}
		})
	})
}
