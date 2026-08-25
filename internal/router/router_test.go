package router_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	internalauth "github.com/kenyamaneko/overload-party-gateway/packages/internalauth-go"

	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush"
	"github.com/kenyamaneko/overload-party-account/internal/handler/rest"
	"github.com/kenyamaneko/overload-party-account/internal/router"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

func init() {
	gin.SetMode(gin.TestMode)
}

var assertVerifyError = errors.New("invalid token")

// newTestRouter は router.New に渡す実 handler 群を、認証境界検証専用の fake 依存で組み立てる。
// この章は認証ミドルウェアの配置のみを対象とするため (spec: internal/router 章)、
// ハンドラの実処理結果は問わない。
func newTestRouter(verifier internalauth.Verifier) *gin.Engine {
	authInteractor := usecase.NewAuthInteractor(
		fakePlayerRepo{}, fakePlayerViewRepo{}, fakePlayerSettingsRepo{}, fakeGameConfigRepo{}, fakeTxRunner{},
	)
	playerInteractor := usecase.NewPlayerInteractor(
		fakePlayerRepo{}, fakePlayerPremiumRepo{}, fakePlayerProgressionRepo{}, fakePlayerBattleRepo{},
		fakeBattleCountReversalRepo{}, fakePlayerViewRepo{}, fakeGameConfigRepo{}, fakeTxRunner{},
	)
	factionInteractor := usecase.NewFactionInteractor(fakePlayerRepo{}, fakeFactionRepo{}, fakeTxRunner{})
	settingsInteractor := usecase.NewPlayerSettingsInteractor(fakePlayerSettingsRepo{})

	noopHandle := func(ctx context.Context, data []byte) error { return nil }
	pubsubHandlers := pubsubpush.Handlers{
		FactionAcquired:      pubsubpush.NewEventHandler(noopHandle),
		PremiumUpdated:       pubsubpush.NewEventHandler(noopHandle),
		PlayerOnboarded:      pubsubpush.NewEventHandler(noopHandle),
		OnboardingNameSet:    pubsubpush.NewEventHandler(noopHandle),
		OnboardingFactionSet: pubsubpush.NewEventHandler(noopHandle),
	}

	return router.New(
		rest.NewAuthHandler(authInteractor),
		rest.NewPlayerHandler(playerInteractor),
		rest.NewFactionHandler(factionInteractor),
		rest.NewPlayerSettingsHandler(settingsInteractor),
		verifier,
		pubsubHandlers,
	)
}

func TestRouter_AuthBoundary(t *testing.T) {
	t.Run("認証ミドルウェアの配置", func(t *testing.T) {
		t.Run("/api/v1/account/me配下のエンドポイントは、X-Internal-Authヘッダが無いとき、401を返す", func(t *testing.T) {
			verifier := &internalauth.MockVerifier{VerifyFn: func(token string) (string, error) {
				return "player-1", nil
			}}
			r := newTestRouter(verifier)

			req := httptest.NewRequest("GET", "/api/v1/account/me", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, 401, w.Code)
		})

		t.Run("/api/v1/account/me配下のエンドポイントは、X-Internal-Authの検証に失敗するとき、401を返す", func(t *testing.T) {
			verifier := &internalauth.MockVerifier{VerifyFn: func(token string) (string, error) {
				return "", assertVerifyError
			}}
			r := newTestRouter(verifier)

			req := httptest.NewRequest("GET", "/api/v1/account/me", nil)
			req.Header.Set(internalauth.HeaderName, "invalid-token")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, 401, w.Code)
		})

		t.Run("/internal/v1/*配下は、X-Internal-Authヘッダの有無に関わらず401にならない", func(t *testing.T) {
			verifier := &internalauth.MockVerifier{VerifyFn: func(token string) (string, error) {
				return "", assertVerifyError
			}}
			r := newTestRouter(verifier)

			body, err := json.Marshal(map[string]string{"firebase_uid": "test-uid"})
			require.NoError(t, err)
			req := httptest.NewRequest("POST", "/internal/v1/auth/login", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.NotEqual(t, 401, w.Code)
		})

		t.Run("/healthは、X-Internal-Authヘッダの有無に関わらず401にならない", func(t *testing.T) {
			verifier := &internalauth.MockVerifier{VerifyFn: func(token string) (string, error) {
				return "", assertVerifyError
			}}
			r := newTestRouter(verifier)

			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.NotEqual(t, 401, w.Code)
		})

		t.Run("GET /healthは、認証なしで200とstatus=okを返す", func(t *testing.T) {
			verifier := &internalauth.MockVerifier{VerifyFn: func(token string) (string, error) {
				return "", assertVerifyError
			}}
			r := newTestRouter(verifier)

			req := httptest.NewRequest("GET", "/health", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			assert.Equal(t, 200, w.Code)
			var body map[string]string
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, "ok", body["status"])
		})
	})
}
