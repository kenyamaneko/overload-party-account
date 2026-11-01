//go:build integration

package pubsubpushtest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/logging/loggingtest"
	"github.com/kenyamaneko/overload-party-account/internal/adapter/system"
	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/handler/handlertest"
	"github.com/kenyamaneko/overload-party-account/internal/handler/pubsubpush"
	"github.com/kenyamaneko/overload-party-account/internal/handler/rest"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	accountfirestore "github.com/kenyamaneko/overload-party-account/internal/repository/firestore"
	"github.com/kenyamaneko/overload-party-account/internal/repository/firestore/firestoretest"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres/postgrestest"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

const (
	// FixedEventID はイベントIDを固定したいときに使う値である。
	FixedEventID = "11111111-1111-4111-8111-111111111111"
	// UnregisteredPlayerID は登録していないプレイヤーを宛先にしたいときに使うプレイヤーIDである。
	UnregisteredPlayerID = "22222222-2222-4222-8222-222222222222"

	registerPath = "/internal/v1/auth/register"
)

// AccountStack はプレイヤー登録 API と Pub/Sub の push 受け口を、実 PostgreSQL に接続して公開する HTTP サーバである。
type AccountStack struct {
	engine   *gin.Engine
	views    *postgres.PlayerViewRepository
	factions *postgres.FactionRepository
}

// PlayerSetup はプレイヤーを用意し、そのプレイヤーIDを返す。
type PlayerSetup func(t *testing.T, s *AccountStack) string

// Outcome はイベントを送った結果の応答・ログ・宛先のプレイヤーを持つ。
type Outcome struct {
	Stack    *AccountStack
	Rec      *httptest.ResponseRecorder
	Logs     *loggingtest.Logs
	PlayerID string
}

// OutcomeCheck はイベントを送った結果を確かめる。
type OutcomeCheck func(t *testing.T, o Outcome)

func newGameConfigRepo(t *testing.T) *accountfirestore.GameConfigRepository {
	t.Helper()
	client := firestoretest.NewClient(t, firestoretest.NewProjectID())
	firestoretest.SeedGameConfig(t, client, map[string]int64{
		"free_daily_battle_limit":    10,
		"premium_daily_battle_limit": 100,
		"exp_win":                    40,
		"exp_loss":                   20,
		"exp_draw":                   30,
		"exp_formula_coefficient":    60,
	})
	return accountfirestore.New(client)
}

// NewAccountStack は pool のデータベースに接続し、プレイヤーIDを idGenerator で採番する AccountStack を返す。
func NewAccountStack(t *testing.T, pool *pgxpool.Pool, idGenerator port.PlayerIDGenerator) *AccountStack {
	t.Helper()
	txManager := postgres.NewTxManager(pool)
	playerRepo := postgres.NewPlayerRepository(pool)
	playerViewRepo := postgres.NewPlayerViewRepository(pool)
	playerSettingsRepo := postgres.NewPlayerSettingsRepository(pool)
	gameConfigRepo := newGameConfigRepo(t)
	factionRepo := postgres.NewFactionRepository(pool)
	eventRepo := postgres.NewProcessedEventRepository(pool)
	clock := system.NewSystemClock()

	authInteractor := usecase.NewAuthInteractor(playerRepo, playerViewRepo, playerSettingsRepo, gameConfigRepo, txManager, clock, idGenerator)
	factionInteractor := usecase.NewFactionInteractor(playerRepo, factionRepo, eventRepo, txManager)
	premiumInteractor := usecase.NewPremiumInteractor(playerRepo, eventRepo, txManager)
	onboardingInteractor := usecase.NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, txManager)

	factionAcquired := pubsubpush.NewFactionAcquiredSubscriber(factionInteractor)
	premiumUpdated := pubsubpush.NewPremiumUpdatedSubscriber(premiumInteractor)
	playerOnboarded := pubsubpush.NewPlayerOnboardedSubscriber(onboardingInteractor)
	onboardingNameSet := pubsubpush.NewOnboardingNameSetSubscriber(onboardingInteractor)
	onboardingFactionSet := pubsubpush.NewOnboardingFactionSetSubscriber(onboardingInteractor)

	engine := gin.New()
	engine.POST(registerPath, rest.NewAuthHandler(authInteractor).Register)
	engine.POST(PushPathFactionAcquired, pubsubpush.NewEventHandler(pubsubpush.FactionAcquiredSubscription, factionAcquired.ReceiveFactionAcquired).ReceivePush)
	engine.POST(PushPathPremiumUpdated, pubsubpush.NewEventHandler(pubsubpush.PremiumUpdatedSubscription, premiumUpdated.ReceivePremiumUpdated).ReceivePush)
	engine.POST(PushPathPlayerOnboarded, pubsubpush.NewEventHandler(pubsubpush.PlayerOnboardedSubscription, playerOnboarded.ReceivePlayerOnboarded).ReceivePush)
	engine.POST(PushPathOnboardingNameSet, pubsubpush.NewEventHandler(pubsubpush.OnboardingNameSetSubscription, onboardingNameSet.ReceiveOnboardingNameSet).ReceivePush)
	engine.POST(PushPathOnboardingFactionSet, pubsubpush.NewEventHandler(pubsubpush.OnboardingFactionSetSubscription, onboardingFactionSet.ReceiveOnboardingFactionSet).ReceivePush)

	return &AccountStack{engine: engine, views: playerViewRepo, factions: factionRepo}
}

// NewCase は pg から作ったケース専用のデータベースに接続した AccountStack と、ログの捕捉結果を返す。
func NewCase(t *testing.T, pg *postgrestest.Postgres) (*AccountStack, *loggingtest.Logs) {
	t.Helper()
	db := pg.NewDatabase(t)
	stack := NewAccountStack(t, db.Pool, system.NewUUIDPlayerIDGenerator())
	return stack, loggingtest.Capture(t)
}

// NewCaseWithSetup は NewCase の AccountStack に setup でプレイヤーを用意し、そのプレイヤーIDとあわせて返す。
func NewCaseWithSetup(t *testing.T, pg *postgrestest.Postgres, setup PlayerSetup) (*AccountStack, *loggingtest.Logs, string) {
	t.Helper()
	stack, logs := NewCase(t, pg)
	return stack, logs, setup(t, stack)
}

// PushEventForPlayer は setup で用意したプレイヤーを宛先に buildBody が作ったイベント本文を path へ送り、結果を返す。
func PushEventForPlayer(t *testing.T, pg *postgrestest.Postgres, path string, setup PlayerSetup, buildBody func(t *testing.T, playerID string) string) Outcome {
	t.Helper()
	stack, logs, playerID := NewCaseWithSetup(t, pg, setup)
	rec := stack.Push(t, path, buildBody(t, playerID))
	return Outcome{Stack: stack, Rec: rec, Logs: logs, PlayerID: playerID}
}

// SetupRegistered は登録直後のプレイヤーを用意する。
func SetupRegistered(t *testing.T, s *AccountStack) string {
	t.Helper()
	return s.Register(t)
}

// SetupNameSet はオンボーディング進行状態が「name_set」で表示名が「テスト太郎」のプレイヤーを用意する。
func SetupNameSet(t *testing.T, s *AccountStack) string {
	t.Helper()
	return s.RegisterWithNameSet(t, "テスト太郎")
}

// SetupFactionSet はオンボーディング進行状態が「faction_set」で表示名が「テスト太郎」のプレイヤーを用意する。
func SetupFactionSet(t *testing.T, s *AccountStack) string {
	t.Helper()
	return s.RegisterWithFactionSet(t, "テスト太郎")
}

// SetupCompleted はオンボーディング進行状態が「completed」で表示名が「テスト太郎」のプレイヤーを用意する。
func SetupCompleted(t *testing.T, s *AccountStack) string {
	t.Helper()
	return s.RegisterWithCompleted(t, "テスト太郎")
}

// SetupUnregistered は登録していないプレイヤーのプレイヤーIDを返す。
func SetupUnregistered(t *testing.T, _ *AccountStack) string {
	t.Helper()
	return uuid.NewString()
}

// NewEventID は新しいイベントIDを返す。
func NewEventID() string { return uuid.NewString() }

// Push はイベント本文 eventBody を path の push 受け口へ送り、応答を返す。
func (s *AccountStack) Push(t *testing.T, path, eventBody string) *httptest.ResponseRecorder {
	t.Helper()
	return PushEvent(t, s.engine, path, eventBody)
}

// PushAccepted は eventBody を path の push 受け口へ送り、応答が 200 であることを確かめる。
func (s *AccountStack) PushAccepted(t *testing.T, path, eventBody string) {
	t.Helper()
	rec := s.Push(t, path, eventBody)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
}

// PushRejected は eventBody を path の push 受け口へ送り、応答が 500 であることを確かめる。
func (s *AccountStack) PushRejected(t *testing.T, path, eventBody string) {
	t.Helper()
	rec := s.Push(t, path, eventBody)
	require.Equal(t, http.StatusInternalServerError, rec.Code, rec.Body.String())
}

// Register はプレイヤーを登録し、そのプレイヤーIDを返す。
func (s *AccountStack) Register(t *testing.T) string {
	t.Helper()
	body := MarshalJSON(t, map[string]any{"firebase_uid": "test-firebase-uid-" + uuid.NewString()})
	rec := handlertest.PostJSON(t, s.engine, registerPath, body)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var player apiaccount.PlayerResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &player))
	return player.PlayerID
}

// RegisterWithNameSet はプレイヤーを登録し、表示名 name のイベントを反映してそのプレイヤーIDを返す。
func (s *AccountStack) RegisterWithNameSet(t *testing.T, name string) string {
	t.Helper()
	playerID := s.Register(t)
	s.PushAccepted(t, PushPathOnboardingNameSet, BuildNameSetBody(t, NewEventID(), playerID, name))
	return playerID
}

// RegisterWithFactionSet はプレイヤーを登録し、表示名 name のイベントと陣営「SHE」のイベントを反映してそのプレイヤーIDを返す。
func (s *AccountStack) RegisterWithFactionSet(t *testing.T, name string) string {
	t.Helper()
	playerID := s.RegisterWithNameSet(t, name)
	s.PushAccepted(t, PushPathOnboardingFactionSet, BuildFactionSetBody(t, NewEventID(), playerID, "SHE"))
	return playerID
}

// RegisterWithCompleted はプレイヤーを登録し、表示名 name のイベント・陣営「SHE」のイベント・完了のイベントを反映してそのプレイヤーIDを返す。
func (s *AccountStack) RegisterWithCompleted(t *testing.T, name string) string {
	t.Helper()
	playerID := s.RegisterWithFactionSet(t, name)
	s.PushAccepted(t, PushPathPlayerOnboarded, BuildOnboardedBody(t, NewEventID(), playerID))
	return playerID
}

// FetchPlayerView は永続化層から読み直したプレイヤー情報を返す。
func (s *AccountStack) FetchPlayerView(t *testing.T, playerID string) *domain.PlayerView {
	t.Helper()
	view, err := s.views.GetByID(context.Background(), playerID)
	require.NoError(t, err)
	return view
}

// FetchOwnedFactions は永続化層から読み直した所持陣営を返す。
func (s *AccountStack) FetchOwnedFactions(t *testing.T, playerID string) []string {
	t.Helper()
	factions, err := s.factions.GetPlayerFactions(context.Background(), playerID)
	require.NoError(t, err)
	return factions
}

// AssertStatus は応答のステータスが want であることを確かめる。
func AssertStatus(t *testing.T, want int, rec *httptest.ResponseRecorder) {
	t.Helper()
	assert.Equal(t, want, rec.Code, rec.Body.String())
}

// AssertInitialFaction はプレイヤー情報の初期陣営が want であることを確かめる。
func AssertInitialFaction(t *testing.T, want string, view *domain.PlayerView) {
	t.Helper()
	require.NotNil(t, view.InitialFaction)
	assert.Equal(t, want, *view.InitialFaction)
}

// CheckPushResult は check で応答と捕捉したログを確かめる OutcomeCheck を返す。
func CheckPushResult(check PushCheck) OutcomeCheck {
	return func(t *testing.T, o Outcome) {
		t.Helper()
		check(t, o.Rec, o.Logs)
	}
}

// RespondsOK は応答が 200 であることを確かめる OutcomeCheck を返す。
func RespondsOK() OutcomeCheck {
	return func(t *testing.T, o Outcome) {
		t.Helper()
		AssertStatus(t, http.StatusOK, o.Rec)
	}
}

// OwnsFactions は宛先のプレイヤーの所持陣営が want であることを確かめる OutcomeCheck を返す。
func OwnsFactions(want ...string) OutcomeCheck {
	return func(t *testing.T, o Outcome) {
		t.Helper()
		assert.Equal(t, want, o.Stack.FetchOwnedFactions(t, o.PlayerID))
	}
}

// BuildFactionAcquiredBody は陣営獲得イベントのイベント本文を返す。
func BuildFactionAcquiredBody(t *testing.T, eventID, playerID, faction string) string {
	t.Helper()
	return MarshalJSON(t, map[string]any{
		"event_type": "faction_acquired",
		"event_id":   eventID,
		"timestamp":  "2026-01-01T00:00:00Z",
		"player_id":  playerID,
		"faction":    faction,
	})
}

// BuildNameSetBody はオンボーディング表示名設定イベントのイベント本文を返す。
func BuildNameSetBody(t *testing.T, eventID, playerID, name string) string {
	t.Helper()
	return MarshalJSON(t, map[string]any{
		"event_type": "onboarding_name_set",
		"event_id":   eventID,
		"timestamp":  "2026-01-01T00:00:00Z",
		"player_id":  playerID,
		"name":       name,
	})
}

// BuildFactionSetBody はオンボーディング陣営設定イベントのイベント本文を返す。
func BuildFactionSetBody(t *testing.T, eventID, playerID, faction string) string {
	t.Helper()
	return MarshalJSON(t, map[string]any{
		"event_type":         "onboarding_faction_set",
		"event_id":           eventID,
		"timestamp":          "2026-01-01T00:00:00Z",
		"player_id":          playerID,
		"initial_faction_id": faction,
	})
}

// BuildOnboardedBody はオンボーディング完了イベントのイベント本文を返す。
func BuildOnboardedBody(t *testing.T, eventID, playerID string) string {
	t.Helper()
	return MarshalJSON(t, map[string]any{
		"event_type":         "player_onboarded",
		"event_id":           eventID,
		"timestamp":          "2026-01-01T00:00:00Z",
		"player_id":          playerID,
		"initial_faction_id": "SHE",
	})
}
