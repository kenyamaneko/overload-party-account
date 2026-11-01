// Package usecasetest は usecase のテストが共有する、メモリ上の偽物を組んだ環境と前提の用意を提供する。
package usecasetest

import (
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/logging/loggingtest"
	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/port/porttest"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

const japanUTCOffsetSeconds = 9 * 60 * 60

// ErrConnection は保存先が返す接続エラーの代わりに使うエラーである。
var ErrConnection = errors.New("connection refused")

// JapanTimeZone は日本時間のタイムゾーンである。
var JapanTimeZone = time.FixedZone("JST", japanUTCOffsetSeconds)

// DummyEventType はテストで反映するイベントの種別である。
const DummyEventType = "test.event"

// DecisiveWinReason は勝者が決まった対戦の理由として使う、引き分けでない値である。
const DecisiveWinReason = domain.WinReason("budget_zero")

// StepsToNameSet はオンボーディングを表示名設定済みまで進めるイベントの反映回数である。
const StepsToNameSet = 1

// StepsToFactionSet はオンボーディングを陣営設定済みまで進めるイベントの反映回数である。
const StepsToFactionSet = 2

// StepsToCompleted はオンボーディングを完了まで進めるイベントの反映回数である。
const StepsToCompleted = 3

// UnitEnv はメモリ上の保存先に各ユースケースをつないだ単体テスト用の環境である。
type UnitEnv struct {
	Store      *porttest.Memory
	Auth       *usecase.AuthInteractor
	Player     *usecase.PlayerInteractor
	Faction    *usecase.FactionInteractor
	Onboarding *usecase.OnboardingInteractor
	Premium    *usecase.PremiumInteractor
}

// NewGameConfigValues はテスト用のゲーム設定値を返す。
func NewGameConfigValues() map[string]int64 {
	return map[string]int64{
		"free_daily_battle_limit": 3,
		"exp_formula_coefficient": 100,
		"exp_win":                 100,
		"exp_loss":                30,
		"exp_draw":                50,
	}
}

// NewMemoryStore はテスト用のゲーム設定値を持つ、プレイヤーが1人もいないメモリ上の保存先を返す。
func NewMemoryStore() *porttest.Memory {
	return porttest.NewMemory(NewGameConfigValues())
}

// SelectWarnings は記録されたログのうち警告ログだけを返す。
func SelectWarnings(logs *loggingtest.Logs) []loggingtest.Record {
	return logs.SelectByLevel(slog.LevelWarn)
}

// SelectWarningValues は警告ログのうち key の項目の値を、記録された順に返す。
func SelectWarningValues(logs *loggingtest.Logs, key string) []string {
	values := []string{}
	for _, record := range SelectWarnings(logs) {
		values = append(values, record.String(key))
	}
	return values
}

// NewUnitEnv は日本時間の 2026-01-10 12:00 を現在時刻とする単体テスト用の環境を返す。
func NewUnitEnv(t *testing.T) *UnitEnv {
	t.Helper()
	return NewUnitEnvWithPlayerIDGenerator(t, porttest.NewSequentialPlayerIDGenerator())
}

// NewUnitEnvWithPlayerIDGenerator はプレイヤーIDの採番に generator を使う単体テスト用の環境を返す。
func NewUnitEnvWithPlayerIDGenerator(t *testing.T, generator port.PlayerIDGenerator) *UnitEnv {
	t.Helper()
	store := NewMemoryStore()
	clock := porttest.NewFixedClock(time.Date(2026, 1, 10, 12, 0, 0, 0, JapanTimeZone))
	playerRepo := store.PlayerRepo()
	viewRepo := store.PlayerViewRepo()
	txRunner := store.TxRunner()
	return &UnitEnv{
		Store: store,
		Auth: usecase.NewAuthInteractor(
			playerRepo, viewRepo, store.PlayerSettingsRepo(), store.GameConfigRepo(), txRunner, clock, generator,
		),
		Player: usecase.NewPlayerInteractor(
			playerRepo, store.PlayerProgressionRepo(), store.PlayerBattleRepo(), store.BattleCountReversalRepo(),
			viewRepo, store.GameConfigRepo(), txRunner, clock,
		),
		Faction: usecase.NewFactionInteractor(playerRepo, store.FactionRepo(), store.ProcessedEventRepo(), txRunner),
		Onboarding: usecase.NewOnboardingInteractor(
			playerRepo, store.PlayerOnboardingRepo(), store.FactionRepo(), store.ProcessedEventRepo(), txRunner,
		),
		Premium: usecase.NewPremiumInteractor(store.PlayerPremiumRepo(), store.ProcessedEventRepo(), txRunner),
	}
}

// RegisterPlayer は未登録の firebase_uid でプレイヤーを登録し、そのプレイヤーIDを返す。
func RegisterPlayer(t *testing.T, auth *usecase.AuthInteractor) string {
	t.Helper()
	info, err := auth.Register(t.Context(), "firebase-uid-"+uuid.NewString())
	require.NoError(t, err)
	return info.View.Player.PlayerID
}

// ToPointer は value を指すポインタを返す。
func ToPointer[T any](value T) *T {
	return &value
}

// AdvanceOnboarding は表示名設定・陣営設定 (SHE)・完了のイベントを順に stepCount 件反映して、プレイヤーのオンボーディングを進める。
func AdvanceOnboarding(t *testing.T, onboarding *usecase.OnboardingInteractor, playerID string, stepCount int) {
	t.Helper()
	ctx := t.Context()
	steps := []func() error{
		func() error {
			return onboarding.ApplyNameSet(ctx, uuid.NewString(), DummyEventType, playerID, "ゆうしゃ")
		},
		func() error {
			return onboarding.ApplyFactionSet(ctx, uuid.NewString(), DummyEventType, playerID, "SHE")
		},
		func() error {
			return onboarding.ApplyCompleted(ctx, uuid.NewString(), DummyEventType, playerID)
		},
	}
	for _, step := range steps[:stepCount] {
		require.NoError(t, step())
	}
}

// ReadStoredExp はメモリ上の保存先から、プレイヤーの参照用の情報を通して、累計経験値を読み直して返す。
func ReadStoredExp(t *testing.T, env *UnitEnv, playerID string) int64 {
	t.Helper()
	view, err := env.Store.PlayerViewRepo().GetByID(t.Context(), playerID)
	require.NoError(t, err)
	return view.Exp
}
