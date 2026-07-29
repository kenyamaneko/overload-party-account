package router

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush"
	"github.com/kenyamaneko/overload-party-account/internal/handler/rest"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"

	internalauth "github.com/kenyamaneko/overload-party-gateway/packages/internalauth-go"
)

func init() {
	gin.SetMode(gin.TestMode)
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

func (stubBattleRepo) DecrementDailyBattleCount(context.Context, string, civil.Date) (bool, error) {
	return true, nil
}

// stubBattleCountReversalRepo は返却が未記録である状態を返す port.BattleCountReversalRepo スタブ。
type stubBattleCountReversalRepo struct{}

func (stubBattleCountReversalRepo) MarkReverted(context.Context, string) (bool, error) {
	return true, nil
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
		stubBattleCountReversalRepo{}, stubPlayerViewRepo{}, stubGameConfigRepo{}, stubTxRunner{},
	)
	factionI := usecase.NewFactionInteractor(stubPlayerRepo{}, stubFactionRepo{}, stubTxRunner{})
	settingsI := usecase.NewPlayerSettingsInteractor(stubPlayerSettingsRepo{})

	return New(
		rest.NewAuthHandler(authI),
		rest.NewPlayerHandler(playerI),
		rest.NewFactionHandler(factionI),
		rest.NewPlayerSettingsHandler(settingsI),
		verifier,
		newTestPubsubHandlers(),
	)
}

// noopMessageHandler は push 受け口の配線先。常に ack (nil) を返す。
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

func TestNew(t *testing.T) {
	t.Run("ルーターの認証配線", func(t *testing.T) {
		t.Run("/health は auth middleware を通らず 200 を返す", func(t *testing.T) {
			// VerifyFn 未設定: /health が verifier に到達しないことの検出を兼ねる
			r := newTestRouter(&internalauth.MockVerifier{})
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
			assert.Equal(t, http.StatusOK, w.Code)
		})

		t.Run("internal ルートは auth-free で handler に到達し成功応答になる", func(t *testing.T) {
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

			// VerifyFn 未設定: auth-free ルートが verifier に到達しないことの検出を兼ねる
			r := newTestRouter(&internalauth.MockVerifier{})
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					w := httptest.NewRecorder()
					req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
					req.Header.Set("Content-Type", "application/json")
					r.ServeHTTP(w, req)
					assert.Equal(t, tc.wantCode, w.Code)
				})
			}
		})

		t.Run("/api/v1/account 配下は auth header 欠落で 401 になる", func(t *testing.T) {
			// VerifyFn 未設定: header 欠落時は middleware が verifier に到達しないことの検出を兼ねる
			r := newTestRouter(&internalauth.MockVerifier{})

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
					assert.Contains(t, w.Body.String(), "header is required")
				})
			}
		})

		t.Run("認証トークンの検証が失敗するとき、401 になり、header 欠落とは異なるトークン不正のエラーが応答本文に含まれる", func(t *testing.T) {
			r := newTestRouter(&internalauth.MockVerifier{
				VerifyFn: func(string) (string, error) { return "", errors.New("invalid token") },
			})
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/account/me", nil)
			req.Header.Set(internalauth.HeaderName, "any.token")
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Contains(t, w.Body.String(), "invalid internal auth token")
		})

		t.Run("有効な token のとき、handler に到達し 200 になる", func(t *testing.T) {
			r := newTestRouter(&internalauth.MockVerifier{
				VerifyFn: func(string) (string, error) { return "TST-PLAYER-1", nil },
			})
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/v1/account/me", nil)
			req.Header.Set(internalauth.HeaderName, "any.token")
			r.ServeHTTP(w, req)
			assert.Equal(t, http.StatusOK, w.Code)
		})

		t.Run("Pub/Sub push の受け口は JWT 認証を経ずに実際の受け口に到達する", func(t *testing.T) {
			// VerifyFn 未設定: push 経路が verifier に到達しないことの検出を兼ねる
			r := newTestRouter(&internalauth.MockVerifier{})

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
