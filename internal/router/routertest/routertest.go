// Package routertest は、メモリ上の偽物でつないだ account サービスのルーターを HTTP で呼び出すテストヘルパを提供する。
package routertest

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/kenyamaneko/overload-party-account/internal/handler/handlertest"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush"
	"github.com/kenyamaneko/overload-party-account/internal/handler/rest"
	"github.com/kenyamaneko/overload-party-account/internal/port/porttest"
	"github.com/kenyamaneko/overload-party-account/internal/router"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	internalauth "github.com/kenyamaneko/overload-party-gateway/packages/internalauth-go"
	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"
	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"
)

// テストで使うプレイヤー・内部認証トークン・FirebaseUID である。
const (
	PlayerAID = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
	PlayerBID = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	TokenA    = "token-a"
	TokenB    = "token-b"

	FirebaseUIDA = "firebase-a"
	FirebaseUIDB = "firebase-b"

	issuedPlayerID = "cccccccc-cccc-4ccc-8ccc-cccccccccccc"
)

// ゲーム設定のキーである。
const (
	ConfigKeyExpCoefficient   = "exp_formula_coefficient"
	ConfigKeyFreeDailyBattles = "free_daily_battle_limit"
	ConfigKeyExpWin           = "exp_win"
	ConfigKeyExpLoss          = "exp_loss"
	ConfigKeyExpDraw          = "exp_draw"
)

// イベントの購読の名前である。
const (
	SubscriptionFactionAcquired      = "faction-acquired"
	SubscriptionPremiumUpdated       = "premium-updated"
	SubscriptionPlayerOnboarded      = "player-onboarded"
	SubscriptionOnboardingNameSet    = "onboarding-name-set"
	SubscriptionOnboardingFactionSet = "onboarding-faction-set"
)

const (
	authHeaderName = "X-Internal-Auth"

	playerPath         = "/api/v1/account/me"
	battleLimitPath    = "/api/v1/account/me/battle-limit"
	factionsPath       = "/api/v1/account/me/factions"
	playerSettingsPath = "/api/v1/account/me/settings"

	jstOffsetSeconds = 9 * 60 * 60
)

// FixedNow はサーバーの時計が常に返す時刻である。
var FixedNow = time.Date(2026, time.October, 20, 12, 0, 0, 0, time.FixedZone("JST", jstOffsetSeconds))

// NoAuth は内部認証トークンを付けないリクエストのヘッダである。
var NoAuth = http.Header(nil)

func newAuthHeader(token string) http.Header {
	return http.Header{authHeaderName: {token}}
}

// NewPlayerSeedA はプレイヤーAとして登録しておく内容を返す。
func NewPlayerSeedA() porttest.PlayerSeed {
	return porttest.NewPlayerSeed(PlayerAID, FirebaseUIDA, FixedNow)
}

// NewPlayerSeedB はプレイヤーBとして登録しておく内容を返す。
func NewPlayerSeedB() porttest.PlayerSeed {
	return porttest.NewPlayerSeed(PlayerBID, FirebaseUIDB, FixedNow)
}

// Options はサーバーの構成である。
type Options struct {
	GameConfig      map[string]int64
	Tokens          map[string]string
	HandlerFailures map[string]error
	ViewRepoFailure error
	ViewRepoPanic   any
}

// Server はメモリ上の偽物でつないだルーターと、その保存内容である。
type Server struct {
	handler http.Handler
	Memory  *porttest.Memory
}

// NewServer は opts の構成のサーバーを返す。
// opts.Tokens は内部認証トークンからプレイヤーIDへの対応、opts.HandlerFailures は購読の名前から、その購読の処理を失敗させるエラーへの対応である。
func NewServer(opts Options) *Server {
	memory := porttest.NewMemory(opts.GameConfig)
	memory.FailPlayerViewLookupWith(opts.ViewRepoFailure)
	memory.PanicOnPlayerViewLookupWith(opts.ViewRepoPanic)
	clock := porttest.NewFixedClock(FixedNow)
	txRunner := memory.TxRunner()

	authInteractor := usecase.NewAuthInteractor(
		memory.PlayerRepo(), memory.PlayerViewRepo(), memory.PlayerSettingsRepo(), memory.GameConfigRepo(),
		txRunner, clock, porttest.NewFixedPlayerIDGenerator(issuedPlayerID),
	)
	playerInteractor := usecase.NewPlayerInteractor(
		memory.PlayerRepo(), memory.PlayerProgressionRepo(), memory.PlayerBattleRepo(), memory.BattleCountReversalRepo(),
		memory.PlayerViewRepo(), memory.GameConfigRepo(), txRunner, clock,
	)
	premiumInteractor := usecase.NewPremiumInteractor(memory.PlayerPremiumRepo(), memory.ProcessedEventRepo(), txRunner)
	factionInteractor := usecase.NewFactionInteractor(memory.PlayerRepo(), memory.FactionRepo(), memory.ProcessedEventRepo(), txRunner)
	settingsInteractor := usecase.NewPlayerSettingsInteractor(memory.PlayerSettingsRepo())

	engine := router.New(
		rest.NewAuthHandler(authInteractor),
		rest.NewPlayerHandler(playerInteractor, premiumInteractor),
		rest.NewFactionHandler(factionInteractor),
		rest.NewPlayerSettingsHandler(settingsInteractor),
		newTokenVerifier(opts.Tokens),
		newPubsubHandlers(opts.HandlerFailures),
	)
	return &Server{handler: engine, Memory: memory}
}

// NewSeededServer は、ゲーム設定が gameConfig で、プレイヤーAとプレイヤーBの内部認証トークンが有効で、seeds のプレイヤーが登録済みのサーバーを返す。
func NewSeededServer(gameConfig map[string]int64, seeds ...porttest.PlayerSeed) *Server {
	s := NewServer(Options{
		GameConfig: gameConfig,
		Tokens:     map[string]string{TokenA: PlayerAID, TokenB: PlayerBID},
	})
	for _, seed := range seeds {
		s.Memory.AddPlayer(seed)
	}
	return s
}

// Call はメソッド method・パス path・ヘッダ headers・JSON の本文 body のリクエストを送り、応答を返す。
func (s *Server) Call(t *testing.T, method, path string, headers http.Header, body string) *httptest.ResponseRecorder {
	t.Helper()
	return handlertest.Call(t, s.handler, method, path, headers, body)
}

// GetAs は内部認証トークン token を付けて path を GET し、応答を返す。
func (s *Server) GetAs(t *testing.T, token, path string) *httptest.ResponseRecorder {
	t.Helper()
	return s.Call(t, http.MethodGet, path, newAuthHeader(token), "")
}

// PostAs は内部認証トークン token を付けて path へ JSON の本文 body を POST し、応答を返す。
func (s *Server) PostAs(t *testing.T, token, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return s.Call(t, http.MethodPost, path, newAuthHeader(token), body)
}

// PutAs は内部認証トークン token を付けて path へ JSON の本文 body を PUT し、応答を返す。
func (s *Server) PutAs(t *testing.T, token, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	return s.Call(t, http.MethodPut, path, newAuthHeader(token), body)
}

// GetPlayer は内部認証トークン token のプレイヤーのプレイヤー情報を取得し、応答を返す。
func (s *Server) GetPlayer(t *testing.T, token string) *httptest.ResponseRecorder {
	t.Helper()
	return s.GetAs(t, token, playerPath)
}

// GetBattleLimit は内部認証トークン token のプレイヤーのバトル回数制限を取得し、応答を返す。
func (s *Server) GetBattleLimit(t *testing.T, token string) *httptest.ResponseRecorder {
	t.Helper()
	return s.GetAs(t, token, battleLimitPath)
}

// GetFactions は内部認証トークン token のプレイヤーの所持陣営一覧を取得し、応答を返す。
func (s *Server) GetFactions(t *testing.T, token string) *httptest.ResponseRecorder {
	t.Helper()
	return s.GetAs(t, token, factionsPath)
}

// GetSettings は内部認証トークン token のプレイヤーのプレイヤー設定を取得し、応答を返す。
func (s *Server) GetSettings(t *testing.T, token string) *httptest.ResponseRecorder {
	t.Helper()
	return s.GetAs(t, token, playerSettingsPath)
}

func newTokenVerifier(tokens map[string]string) internalauth.Verifier {
	return &internalauth.MockVerifier{VerifyFn: func(token string) (string, error) {
		playerID, isValid := tokens[token]
		if !isValid {
			return "", errors.New("token is not valid")
		}
		return playerID, nil
	}}
}

func newPubsubHandlers(failures map[string]error) pubsubpush.Handlers {
	return pubsubpush.Handlers{
		FactionAcquired: pubsubpush.NewEventHandler(SubscriptionFactionAcquired, func(_ context.Context, _ apishop.FactionAcquiredEvent) error {
			return failures[SubscriptionFactionAcquired]
		}),
		PremiumUpdated: pubsubpush.NewEventHandler(SubscriptionPremiumUpdated, func(_ context.Context, _ pubsubpush.PremiumUpdatedEvent) error {
			return failures[SubscriptionPremiumUpdated]
		}),
		PlayerOnboarded: pubsubpush.NewEventHandler(SubscriptionPlayerOnboarded, func(_ context.Context, _ apiscenario.PlayerOnboardedEvent) error {
			return failures[SubscriptionPlayerOnboarded]
		}),
		OnboardingNameSet: pubsubpush.NewEventHandler(SubscriptionOnboardingNameSet, func(_ context.Context, _ apiscenario.OnboardingNameSetEvent) error {
			return failures[SubscriptionOnboardingNameSet]
		}),
		OnboardingFactionSet: pubsubpush.NewEventHandler(SubscriptionOnboardingFactionSet, func(_ context.Context, _ apiscenario.OnboardingFactionSetEvent) error {
			return failures[SubscriptionOnboardingFactionSet]
		}),
	}
}
