//go:build integration

package usecasetest

import (
	"testing"
	"time"

	"github.com/kenyamaneko/overload-party-account/internal/adapter/system"
	"github.com/kenyamaneko/overload-party-account/internal/port/porttest"
	"github.com/kenyamaneko/overload-party-account/internal/repository/firestore"
	"github.com/kenyamaneko/overload-party-account/internal/repository/firestore/firestoretest"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres/postgrestest"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

// IntegrationEnv は実 PostgreSQL と Firestore エミュレータに各ユースケースをつないだ結合テスト用の環境である。
type IntegrationEnv struct {
	Auth       *usecase.AuthInteractor
	Player     *usecase.PlayerInteractor
	Faction    *usecase.FactionInteractor
	Onboarding *usecase.OnboardingInteractor
}

// NewIntegrationEnv は pg から作ったケース専用のデータベースと、専用のプロジェクトの Firestore に各ユースケースをつないだ環境を返す。
func NewIntegrationEnv(t *testing.T, pg *postgrestest.Postgres) *IntegrationEnv {
	t.Helper()
	client := firestoretest.NewClient(t, firestoretest.NewProjectID())
	firestoretest.SeedGameConfig(t, client, NewGameConfigValues())

	pool := pg.NewDatabase(t).Pool
	clock := porttest.NewFixedClock(time.Date(2026, 1, 10, 12, 0, 0, 0, JapanTimeZone))
	playerRepo := postgres.NewPlayerRepository(pool)
	viewRepo := postgres.NewPlayerViewRepository(pool)
	factionRepo := postgres.NewFactionRepository(pool)
	eventRepo := postgres.NewProcessedEventRepository(pool)
	txRunner := postgres.NewTxManager(pool)
	configRepo := firestore.New(client)
	return &IntegrationEnv{
		Auth: usecase.NewAuthInteractor(
			playerRepo, viewRepo, postgres.NewPlayerSettingsRepository(pool), configRepo,
			txRunner, clock, system.NewUUIDPlayerIDGenerator(),
		),
		Player: usecase.NewPlayerInteractor(
			playerRepo, playerRepo, playerRepo, postgres.NewBattleCountReversalRepository(pool),
			viewRepo, configRepo, txRunner, clock,
		),
		Faction:    usecase.NewFactionInteractor(playerRepo, factionRepo, eventRepo, txRunner),
		Onboarding: usecase.NewOnboardingInteractor(playerRepo, playerRepo, factionRepo, eventRepo, txRunner),
	}
}
