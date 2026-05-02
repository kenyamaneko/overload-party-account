package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// mustMarshal は json.Marshal の testing 版。event_type 欠落など、typed helper
// (apishopfake.PublishXxx) で表現できない壊れたペイロードをケースに含めるための
// ヘルパ。
func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

// fakeTxRunner はトランザクション境界を持たないパススルー実装。
// subscriber の business logic から見た RunInTx の契約は「fn を同じ ctx で呼ぶ」だけ
// なので、テストでは実 DB を立てず直列実行する。
type fakeTxRunner struct{}

func (fakeTxRunner) RunInTx(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}

// fakeProcessedEventRepo は processed_events の冪等ガードをメモリで再現する。
// Insert は同一 event_id で 2 回目に false を返し、subscriber の idempotent ack
// 経路を検証できるようにする。
type fakeProcessedEventRepo struct {
	seen    map[string]string // event_id -> event_type
	insertN int
	// insertErr を設定すると Insert が常にこのエラーを返す (DB 障害の再現)。
	insertErr error
}

func newFakeProcessedEventRepo() *fakeProcessedEventRepo {
	return &fakeProcessedEventRepo{seen: map[string]string{}}
}

func (r *fakeProcessedEventRepo) Insert(_ context.Context, eventID, eventType string) (bool, error) {
	r.insertN++
	if r.insertErr != nil {
		return false, r.insertErr
	}
	if _, ok := r.seen[eventID]; ok {
		return false, nil
	}
	r.seen[eventID] = eventType
	return true, nil
}

// fakeFactionRepo は AddPlayerFaction / SetInitialFaction / GetInitialFaction /
// GetPlayerFactions をメモリで実装する。
type fakeFactionRepo struct {
	added  []factionAdd
	addErr error
}

type factionAdd struct {
	PlayerID  string
	Faction   string
	IsInitial bool
}

func (r *fakeFactionRepo) AddPlayerFaction(_ context.Context, playerID, faction string) error {
	if r.addErr != nil {
		return r.addErr
	}
	r.added = append(r.added, factionAdd{PlayerID: playerID, Faction: faction, IsInitial: false})
	return nil
}

func (r *fakeFactionRepo) SetInitialFaction(_ context.Context, playerID, faction string) error {
	for _, a := range r.added {
		if a.PlayerID == playerID && a.Faction == faction {
			return fmt.Errorf("duplicate (player_id, faction)")
		}
		if a.PlayerID == playerID && a.IsInitial {
			return fmt.Errorf("partial unique index violation: another initial exists")
		}
	}
	r.added = append(r.added, factionAdd{PlayerID: playerID, Faction: faction, IsInitial: true})
	return nil
}

func (r *fakeFactionRepo) GetInitialFaction(_ context.Context, playerID string) (*string, error) {
	for _, a := range r.added {
		if a.PlayerID == playerID && a.IsInitial {
			f := a.Faction
			return &f, nil
		}
	}
	return nil, nil
}

func (r *fakeFactionRepo) GetPlayerFactions(_ context.Context, playerID string) ([]string, error) {
	var out []string
	for _, a := range r.added {
		if a.PlayerID == playerID {
			out = append(out, a.Faction)
		}
	}
	return out, nil
}

// fakePlayerRepo は premium_updated subscriber 等、port.PlayerRepo を要求する
// subscriber 用の最小実装。onboarded subscriber は usecase.FactionInteractor を
// 経由して repo に触れるため、fakePlayerRepo は参照されない。
// 将来 premium_updated のテストを書く際に使えるよう骨格だけ残す。
type fakePlayerRepo struct {
	names            map[string]string
	premium          map[string]premiumState
	updateNameErr    error
	updatePremiumErr error
}

type premiumState struct {
	IsPremium bool
	ExpiresAt *time.Time
}

func newFakePlayerRepo() *fakePlayerRepo {
	return &fakePlayerRepo{
		names:   map[string]string{},
		premium: map[string]premiumState{},
	}
}

func (r *fakePlayerRepo) Create(_ context.Context, _ *domain.Player, _ *domain.PlayerProgression) error {
	panic("not implemented in test")
}
func (r *fakePlayerRepo) FindByID(_ context.Context, _ string) (*domain.Player, error) {
	panic("not implemented in test")
}
func (r *fakePlayerRepo) FindByFirebaseUID(_ context.Context, _ string) (*domain.Player, error) {
	panic("not implemented in test")
}
func (r *fakePlayerRepo) GetDailyBattle(_ context.Context, _ string, _ civil.Date) (*domain.PlayerDailyBattle, error) {
	panic("not implemented in test")
}
func (r *fakePlayerRepo) IncrementDailyBattleCount(_ context.Context, _ string, _ civil.Date) (int64, error) {
	panic("not implemented in test")
}
func (r *fakePlayerRepo) UpdateName(_ context.Context, playerID, name string) error {
	if r.updateNameErr != nil {
		return r.updateNameErr
	}
	r.names[playerID] = name
	return nil
}
func (r *fakePlayerRepo) UpdatePremium(_ context.Context, playerID string, isPremium bool, expiresAt *time.Time) error {
	if r.updatePremiumErr != nil {
		return r.updatePremiumErr
	}
	r.premium[playerID] = premiumState{IsPremium: isPremium, ExpiresAt: expiresAt}
	return nil
}
func (r *fakePlayerRepo) Exists(_ context.Context, _ string) (bool, error) {
	panic("not implemented in test")
}
func (r *fakePlayerRepo) GetProgression(_ context.Context, _ string) (*domain.PlayerProgression, error) {
	panic("not implemented in test")
}
func (r *fakePlayerRepo) GetProgressionForUpdate(_ context.Context, _ string) (*domain.PlayerProgression, error) {
	panic("not implemented in test")
}
func (r *fakePlayerRepo) UpdateProgression(_ context.Context, _ string, _, _ int64) (*domain.PlayerProgression, error) {
	panic("not implemented in test")
}
func (r *fakePlayerRepo) GetOnboardingStatus(_ context.Context, _ string) (string, error) {
	panic("not implemented in test")
}
func (r *fakePlayerRepo) UpdateOnboardingStatus(_ context.Context, _, _ string) error {
	panic("not implemented in test")
}

// port 境界をテスト時に検証する (コンパイル時 assertion)。
var _ port.PlayerRepo = (*fakePlayerRepo)(nil)
var _ port.FactionRepo = (*fakeFactionRepo)(nil)
var _ port.ProcessedEventRepo = (*fakeProcessedEventRepo)(nil)
var _ port.TxRunner = fakeTxRunner{}
