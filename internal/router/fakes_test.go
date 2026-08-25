package router_test

import (
	"context"
	"time"

	"cloud.google.com/go/civil"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// このファイルの fake 群は、認証ミドルウェアの配置のみを検証する router 章の目的のために存在する。
// ハンドラへ渡す interactor の実データが何であっても認証境界の検証は成立するため
// (spec: internal/router 章)、各メソッドは port.ErrNotFound を返すだけの最小実装にとどめる。

type fakePlayerRepo struct{}

func (fakePlayerRepo) Create(ctx context.Context, player *domain.Player, progression *domain.PlayerProgression) error {
	return port.ErrNotFound
}
func (fakePlayerRepo) FindByID(ctx context.Context, playerID string) (*domain.Player, error) {
	return nil, port.ErrNotFound
}
func (fakePlayerRepo) FindByFirebaseUID(ctx context.Context, firebaseUID string) (*domain.Player, error) {
	return nil, port.ErrNotFound
}
func (fakePlayerRepo) Exists(ctx context.Context, playerID string) (bool, error) {
	return false, nil
}
func (fakePlayerRepo) ExistsByFirebaseUID(ctx context.Context, firebaseUID string) (bool, error) {
	return false, nil
}
func (fakePlayerRepo) UpdateName(ctx context.Context, playerID string, name string) error {
	return port.ErrNotFound
}

type fakePlayerViewRepo struct{}

func (fakePlayerViewRepo) FindByID(ctx context.Context, playerID string) (*domain.PlayerView, error) {
	return nil, port.ErrNotFound
}
func (fakePlayerViewRepo) FindByFirebaseUID(ctx context.Context, firebaseUID string) (*domain.PlayerView, error) {
	return nil, port.ErrNotFound
}

type fakePlayerSettingsRepo struct{}

func (fakePlayerSettingsRepo) Insert(ctx context.Context, s *domain.PlayerSettings) error {
	return nil
}
func (fakePlayerSettingsRepo) Get(ctx context.Context, playerID string) (*domain.PlayerSettings, error) {
	return nil, port.ErrNotFound
}
func (fakePlayerSettingsRepo) UpdatePartial(ctx context.Context, playerID string, patch *port.PlayerSettingsPatch) error {
	return port.ErrNotFound
}

type fakeGameConfigRepo struct{}

func (fakeGameConfigRepo) GetInt64(ctx context.Context, key string) (int64, error) {
	return 0, port.ErrNotFound
}

type fakeTxRunner struct{}

func (fakeTxRunner) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
}

type fakeFactionRepo struct{}

func (fakeFactionRepo) AddPlayerFaction(ctx context.Context, playerID, faction string) error {
	return port.ErrNotFound
}
func (fakeFactionRepo) GetPlayerFactions(ctx context.Context, playerID string) ([]string, error) {
	return nil, nil
}
func (fakeFactionRepo) GetInitialFaction(ctx context.Context, playerID string) (*string, error) {
	return nil, nil
}
func (fakeFactionRepo) SetInitialFaction(ctx context.Context, playerID, faction string) error {
	return port.ErrNotFound
}

type fakePlayerPremiumRepo struct{}

func (fakePlayerPremiumRepo) UpdatePremium(ctx context.Context, playerID string, isPremium bool, expiresAt *time.Time) error {
	return port.ErrNotFound
}

type fakePlayerProgressionRepo struct{}

func (fakePlayerProgressionRepo) GetProgression(ctx context.Context, playerID string) (*domain.PlayerProgression, error) {
	return nil, port.ErrNotFound
}
func (fakePlayerProgressionRepo) GetProgressionForUpdate(ctx context.Context, playerID string) (*domain.PlayerProgression, error) {
	return nil, port.ErrNotFound
}
func (fakePlayerProgressionRepo) UpdateProgression(ctx context.Context, playerID string, exp, level int64) (*domain.PlayerProgression, error) {
	return nil, port.ErrNotFound
}

type fakePlayerBattleRepo struct{}

func (fakePlayerBattleRepo) GetDailyBattle(ctx context.Context, playerID string, gameDate civil.Date) (*domain.PlayerDailyBattle, error) {
	return nil, nil
}
func (fakePlayerBattleRepo) IncrementDailyBattleCount(ctx context.Context, playerID string, gameDate civil.Date) (int64, error) {
	return 0, port.ErrNotFound
}
func (fakePlayerBattleRepo) DecrementDailyBattleCount(ctx context.Context, playerID string, gameDate civil.Date) (bool, error) {
	return false, nil
}

type fakeBattleCountReversalRepo struct{}

func (fakeBattleCountReversalRepo) MarkReverted(ctx context.Context, gameID string) (bool, error) {
	return false, port.ErrNotFound
}
