package router

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/handler/rest"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"

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

// stubPlayerRepo は「未登録の firebase_uid」「存在するプレイヤー」を返す port.PlayerRepo スタブ。
type stubPlayerRepo struct{}

func (stubPlayerRepo) Create(context.Context, *domain.Player, *domain.PlayerProgression) error {
	return nil
}

func (stubPlayerRepo) FindByID(context.Context, string) (*domain.Player, error) {
	return nil, port.ErrNotFound
}

func (stubPlayerRepo) FindByFirebaseUID(context.Context, string) (*domain.Player, error) {
	return nil, port.ErrNotFound
}

func (stubPlayerRepo) Exists(context.Context, string) (bool, error) {
	return true, nil
}

func (stubPlayerRepo) ExistsByFirebaseUID(context.Context, string) (bool, error) {
	return false, nil
}

func (stubPlayerRepo) UpdateName(context.Context, string, string) error {
	return nil
}

// stubPlayerViewRepo は固定の Read Model を返す port.PlayerViewRepo スタブ。
type stubPlayerViewRepo struct{}

func (stubPlayerViewRepo) FindByID(context.Context, string) (*domain.PlayerView, error) {
	return newPlayerView(), nil
}

func (stubPlayerViewRepo) FindByFirebaseUID(context.Context, string) (*domain.PlayerView, error) {
	return newPlayerView(), nil
}

// newPlayerView は stub repository が返す固定の Read Model を生成する。
func newPlayerView() *domain.PlayerView {
	return &domain.PlayerView{
		Player: domain.Player{
			PlayerID:         "TST-PLAYER-1",
			FirebaseUID:      "fb-uid-1",
			OnboardingStatus: domain.OnboardingStatusNotStarted,
			CreatedAt:        time.Unix(0, 0),
			UpdatedAt:        time.Unix(0, 0),
		},
		Level: 1,
		Exp:   0,
	}
}

// stubPlayerSettingsRepo は常に成功する port.PlayerSettingsRepo スタブ。
type stubPlayerSettingsRepo struct{}

func (stubPlayerSettingsRepo) Insert(context.Context, *domain.PlayerSettings) error {
	return nil
}

func (stubPlayerSettingsRepo) Get(context.Context, string) (*domain.PlayerSettings, error) {
	return nil, port.ErrNotFound
}

func (stubPlayerSettingsRepo) UpdatePartial(context.Context, string, *port.PlayerSettingsPatch) error {
	return nil
}

// stubGameConfigRepo は全キーに同一値を返す port.GameConfigRepo スタブ。
type stubGameConfigRepo struct{}

// stubConfigValue は exp 係数・付与量の全キーに使う設定値。level 1 / exp 0 の
// stubPlayerView と組み合わせて exp 進捗計算が成功する正の値であれば良い。
const stubConfigValue = 100

func (stubGameConfigRepo) GetInt64(context.Context, string) (int64, error) {
	return stubConfigValue, nil
}

// stubTxRunner は渡された関数をそのまま実行する port.TxRunner スタブ。
type stubTxRunner struct{}

func (stubTxRunner) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

// stubProgressionRepo は level 1 / exp 0 の進捗を返す port.PlayerProgressionRepo スタブ。
type stubProgressionRepo struct{}

// newProgression は stub repository が返す level 1 / exp 0 の進捗を生成する。
func newProgression(playerID string) *domain.PlayerProgression {
	return &domain.PlayerProgression{PlayerID: playerID, Level: 1, Exp: 0, UpdatedAt: time.Unix(0, 0)}
}

func (stubProgressionRepo) GetProgression(_ context.Context, playerID string) (*domain.PlayerProgression, error) {
	return newProgression(playerID), nil
}

func (stubProgressionRepo) GetProgressionForUpdate(_ context.Context, playerID string) (*domain.PlayerProgression, error) {
	return newProgression(playerID), nil
}

func (stubProgressionRepo) UpdateProgression(_ context.Context, playerID string, exp, level int64) (*domain.PlayerProgression, error) {
	return &domain.PlayerProgression{PlayerID: playerID, Level: level, Exp: exp, UpdatedAt: time.Unix(0, 0)}, nil
}

// stubPremiumRepo は常に成功する port.PlayerPremiumRepo スタブ。
type stubPremiumRepo struct{}

func (stubPremiumRepo) UpdatePremium(context.Context, string, bool, *time.Time) error {
	return nil
}

// stubBattleRepo は当日履歴なしを返す port.PlayerBattleRepo スタブ。
type stubBattleRepo struct{}

func (stubBattleRepo) GetDailyBattle(context.Context, string, civil.Date) (*domain.PlayerDailyBattle, error) {
	return nil, nil
}

func (stubBattleRepo) IncrementDailyBattleCount(context.Context, string, civil.Date) (int64, error) {
	return 1, nil
}

// stubFactionRepo は固定の所持ファクションを返す port.FactionRepo スタブ。
type stubFactionRepo struct{}

func (stubFactionRepo) AddPlayerFaction(context.Context, string, string) error {
	return nil
}

func (stubFactionRepo) GetPlayerFactions(context.Context, string) ([]string, error) {
	return []string{"TST-FACTION"}, nil
}

func (stubFactionRepo) GetInitialFaction(context.Context, string) (*string, error) {
	return nil, nil
}

func (stubFactionRepo) SetInitialFaction(context.Context, string, string) error {
	return nil
}

// newTestRouter はスタブ repository で組んだ実 interactor / handler で router を構築する。
// handler まで実物なので、成功 status の一致だけで auth 誤付与 (header 欠落の internal
// リクエストが 401 になる)・ルート未登録 (404)・handler までの配線破壊 (500) を全て検別できる。
func newTestRouter(verifier internalauth.Verifier) *gin.Engine {
	authI := usecase.NewAuthInteractor(
		stubPlayerRepo{}, stubPlayerViewRepo{}, stubPlayerSettingsRepo{}, stubGameConfigRepo{}, stubTxRunner{},
	)
	playerI := usecase.NewPlayerInteractor(
		stubPlayerRepo{}, stubPremiumRepo{}, stubProgressionRepo{}, stubBattleRepo{},
		stubPlayerViewRepo{}, stubGameConfigRepo{}, stubTxRunner{},
	)
	factionI := usecase.NewFactionInteractor(stubPlayerRepo{}, stubFactionRepo{}, stubTxRunner{})
	settingsI := usecase.NewPlayerSettingsInteractor(stubPlayerSettingsRepo{})

	return New(
		rest.NewAuthHandler(authI),
		rest.NewPlayerHandler(playerI),
		rest.NewFactionHandler(factionI),
		rest.NewPlayerSettingsHandler(settingsI),
		verifier,
	)
}

// /health は auth middleware を通らず常に 200 を返す。
func TestNew_HealthEndpoint(t *testing.T) {
	r := newTestRouter(fakeRouterVerifier{})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestNew_InternalRoutesAreAuthFree(t *testing.T) {
	cases := []struct {
		name     string
		method   string
		path     string
		body     string
		wantCode int
	}{
		{
			name:     "auth/register は auth-free で登録が完了し 201",
			method:   http.MethodPost,
			path:     "/internal/v1/auth/register",
			body:     `{"firebase_uid":"fb-uid-1"}`,
			wantCode: http.StatusCreated,
		},
		{
			name:     "auth/login は auth-free でログインが完了し 200",
			method:   http.MethodPost,
			path:     "/internal/v1/auth/login",
			body:     `{"firebase_uid":"fb-uid-1"}`,
			wantCode: http.StatusOK,
		},
		{
			name:     "auth/by-firebase-uid は auth-free でプレイヤーを返し 200",
			method:   http.MethodGet,
			path:     "/internal/v1/auth/by-firebase-uid/fb-uid-1",
			wantCode: http.StatusOK,
		},
		{
			name:     "players/:playerID/factions は auth-free で一覧を返し 200",
			method:   http.MethodGet,
			path:     "/internal/v1/players/TST-PLAYER-1/factions",
			wantCode: http.StatusOK,
		},
		{
			name:     "players/award-game-exp は auth-free で付与が完了し 204",
			method:   http.MethodPost,
			path:     "/internal/v1/players/award-game-exp",
			body:     `{"player1_id":"TST-PLAYER-1","player2_id":"TST-PLAYER-2","winner_num":0,"reason":"","match_type":""}`,
			wantCode: http.StatusNoContent,
		},
	}

	r := newTestRouter(fakeRouterVerifier{})
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			assert.Equal(t, tc.wantCode, w.Code)
		})
	}
}

// /api/v1/account 配下は auth middleware を通る。
// header 欠落時は 401 を返し、handler に到達しないことを確認する。
func TestNew_ApiRouteRequiresInternalAuth(t *testing.T) {
	r := newTestRouter(fakeRouterVerifier{playerID: "TST-PLAYER-1"})

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

func TestNew_ApiRouteWithValidTokenReachesHandler(t *testing.T) {
	r := newTestRouter(fakeRouterVerifier{playerID: "TST-PLAYER-1"})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/account/me", nil)
	req.Header.Set(internalauth.HeaderName, "any.token")
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}
