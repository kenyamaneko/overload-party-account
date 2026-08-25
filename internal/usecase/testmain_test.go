//go:build integration

package usecase_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres/postgrestest"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

var sharedPg *postgrestest.Postgres

func TestMain(m *testing.M) {
	os.Exit(postgrestest.RunMain(m, &sharedPg,
		postgrestest.WithSchemaFile("db/schema.sql"),
		postgrestest.WithSchema("account"),
	))
}

// newTestAuthInteractor は、ケースごとに独立した状態から始めるため、
// 実 PostgreSQL を Truncate してから実リポジトリを結線した AuthInteractor を返す。
func newTestAuthInteractor(t *testing.T) *usecase.AuthInteractor {
	t.Helper()
	sharedPg.Truncate(t)
	return usecase.NewAuthInteractor(
		postgres.NewPlayerRepository(sharedPg.Pool),
		postgres.NewPlayerViewRepository(sharedPg.Pool),
		postgres.NewPlayerSettingsRepository(sharedPg.Pool),
		newFakeGameConfigRepo(validGameConfigValues()),
		postgres.NewTxManager(sharedPg.Pool),
	)
}

// newTestFactionInteractor はケースごとに独立した状態から始める FactionInteractor を返す。
func newTestFactionInteractor(t *testing.T) *usecase.FactionInteractor {
	t.Helper()
	sharedPg.Truncate(t)
	return usecase.NewFactionInteractor(
		postgres.NewPlayerRepository(sharedPg.Pool),
		postgres.NewFactionRepository(sharedPg.Pool),
		postgres.NewTxManager(sharedPg.Pool),
	)
}

// newTestOnboardingInteractor はケースごとに独立した状態から始める OnboardingInteractor を返す。
func newTestOnboardingInteractor(t *testing.T) *usecase.OnboardingInteractor {
	t.Helper()
	sharedPg.Truncate(t)
	playerRepo := postgres.NewPlayerRepository(sharedPg.Pool)
	return usecase.NewOnboardingInteractor(
		playerRepo,
		playerRepo,
		postgres.NewFactionRepository(sharedPg.Pool),
		postgres.NewProcessedEventRepository(sharedPg.Pool),
		postgres.NewTxManager(sharedPg.Pool),
	)
}

// newTestPlayerInteractor はケースごとに独立した状態から始める PlayerInteractor を返す。
// free_daily_battle_limit を差し替えたい結合テストのために、game_config の値を引数で受け取る。
func newTestPlayerInteractor(t *testing.T, gameConfig map[string]int64) *usecase.PlayerInteractor {
	t.Helper()
	sharedPg.Truncate(t)
	playerRepo := postgres.NewPlayerRepository(sharedPg.Pool)
	return usecase.NewPlayerInteractor(
		playerRepo,
		playerRepo,
		playerRepo,
		playerRepo,
		postgres.NewBattleCountReversalRepository(sharedPg.Pool),
		postgres.NewPlayerViewRepository(sharedPg.Pool),
		newFakeGameConfigRepo(gameConfig),
		postgres.NewTxManager(sharedPg.Pool),
	)
}

// newTestPlayerSettingsInteractor はケースごとに独立した状態から始める PlayerSettingsInteractor を返す。
func newTestPlayerSettingsInteractor(t *testing.T) *usecase.PlayerSettingsInteractor {
	t.Helper()
	sharedPg.Truncate(t)
	return usecase.NewPlayerSettingsInteractor(postgres.NewPlayerSettingsRepository(sharedPg.Pool))
}

// registerTestPlayer は AuthInteractor.Register を通じてテスト用プレイヤーを1人登録し、
// 発行された player_id を返す。Truncate は呼び出し元の newTest*Interactor が既に行っている前提。
func registerTestPlayer(t *testing.T, firebaseUID string) string {
	t.Helper()
	authInteractor := usecase.NewAuthInteractor(
		postgres.NewPlayerRepository(sharedPg.Pool),
		postgres.NewPlayerViewRepository(sharedPg.Pool),
		postgres.NewPlayerSettingsRepository(sharedPg.Pool),
		newFakeGameConfigRepo(validGameConfigValues()),
		postgres.NewTxManager(sharedPg.Pool),
	)
	resp, err := authInteractor.Register(context.Background(), firebaseUID)
	require.NoError(t, err)
	return resp.PlayerID
}
