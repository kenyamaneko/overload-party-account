//go:build integration

package rest_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	internalauth "github.com/kenyamaneko/overload-party-gateway/packages/internalauth-go"

	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush"
	"github.com/kenyamaneko/overload-party-account/internal/handler/rest"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-account/internal/router"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// fakeGameConfigRepo は port.GameConfigRepo を満たすメモリ内 fake。
// handler/rest 章は Firestore を起動せず、これを注入する (spec: 実行基盤の節)。
type fakeGameConfigRepo struct {
	values map[string]int64
}

func (r *fakeGameConfigRepo) GetInt64(ctx context.Context, key string) (int64, error) {
	v, ok := r.values[key]
	if !ok {
		return 0, port.ErrNotFound
	}
	return v, nil
}

func validGameConfigValues() map[string]int64 {
	return map[string]int64{
		"free_daily_battle_limit": 5,
		"exp_formula_coefficient": 100,
		"exp_win":                 50,
		"exp_loss":                10,
		"exp_draw":                20,
	}
}

// newTestRouter は実 PostgreSQL + fake GameConfigRepo を結線した実 handler 群から
// router.New で組み立てた *gin.Engine を返す。X-Internal-Auth の検証は差し替え可能な
// MockVerifier で行う。
func newTestRouter(t *testing.T, gameConfig map[string]int64) (*gin.Engine, *internalauth.MockVerifier) {
	t.Helper()
	sharedPg.Truncate(t)

	gameConfigRepo := &fakeGameConfigRepo{values: gameConfig}
	playerRepo := postgres.NewPlayerRepository(sharedPg.Pool)
	playerViewRepo := postgres.NewPlayerViewRepository(sharedPg.Pool)
	playerSettingsRepo := postgres.NewPlayerSettingsRepository(sharedPg.Pool)
	factionRepo := postgres.NewFactionRepository(sharedPg.Pool)
	battleCountReversalRepo := postgres.NewBattleCountReversalRepository(sharedPg.Pool)
	txManager := postgres.NewTxManager(sharedPg.Pool)

	authInteractor := usecase.NewAuthInteractor(playerRepo, playerViewRepo, playerSettingsRepo, gameConfigRepo, txManager)
	playerInteractor := usecase.NewPlayerInteractor(playerRepo, playerRepo, playerRepo, playerRepo, battleCountReversalRepo, playerViewRepo, gameConfigRepo, txManager)
	factionInteractor := usecase.NewFactionInteractor(playerRepo, factionRepo, txManager)
	settingsInteractor := usecase.NewPlayerSettingsInteractor(playerSettingsRepo)

	noopHandle := func(ctx context.Context, data []byte) error { return nil }
	pubsubHandlers := pubsubpush.Handlers{
		FactionAcquired:      pubsubpush.NewEventHandler(noopHandle),
		PremiumUpdated:       pubsubpush.NewEventHandler(noopHandle),
		PlayerOnboarded:      pubsubpush.NewEventHandler(noopHandle),
		OnboardingNameSet:    pubsubpush.NewEventHandler(noopHandle),
		OnboardingFactionSet: pubsubpush.NewEventHandler(noopHandle),
	}

	verifier := &internalauth.MockVerifier{}
	r := router.New(
		rest.NewAuthHandler(authInteractor),
		rest.NewPlayerHandler(playerInteractor),
		rest.NewFactionHandler(factionInteractor),
		rest.NewPlayerSettingsHandler(settingsInteractor),
		verifier,
		pubsubHandlers,
	)
	return r, verifier
}

// doJSON は router に対して JSON ボディ付きリクエストを送り、応答を返す。
func doJSON(r *gin.Engine, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// doRaw は router に対して任意のバイト列をボディとするリクエストを送り、応答を返す。
// 不正な JSON を送るケースの検証に使う。
func doRaw(r *gin.Engine, method, path string, rawBody []byte, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(rawBody))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// authHeader は player-scoped エンドポイント向けの X-Internal-Auth ヘッダを組み立てる。
// MockVerifier がトークン内容を検証しないため、値そのものは意味を持たない。
func authHeader() map[string]string {
	return map[string]string{internalauth.HeaderName: "test-token"}
}

// registerPlayer は POST /internal/v1/auth/register を実際のHTTP経路で呼び出し、
// 登録されたプレイヤー情報を返す。
func registerPlayer(t *testing.T, r *gin.Engine) apiaccount.PlayerResponse {
	t.Helper()
	w := doJSON(r, "POST", "/internal/v1/auth/register", apiaccount.RegisterRequest{FirebaseUID: "firebase-" + uuid.NewString()}, nil)
	require.Equal(t, 201, w.Code)
	var resp apiaccount.PlayerResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}
