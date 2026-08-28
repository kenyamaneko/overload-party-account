package pubsub_test

import (
	"context"
	"errors"
	"time"
)

// txBufferKey は fakeTxRunner が context に積む txBuffer を取り出すためのキー。
type txBufferKey struct{}

// txBuffer は fakeTxRunner.RunInTx 配下での書き込みを、fn がエラーを返さず完走した場合にのみ
// 実データへ反映するためのコミット待ちキュー。実 PostgreSQL の "同一トランザクション内でコミット"
// (processed_events への記録と業務データの書き込みが揃って成功/失敗する) を fake 上で再現する。
type txBuffer struct {
	pending []func()
}

func withTxBuffer(ctx context.Context, buf *txBuffer) context.Context {
	return context.WithValue(ctx, txBufferKey{}, buf)
}

func txBufferFrom(ctx context.Context) (*txBuffer, bool) {
	buf, ok := ctx.Value(txBufferKey{}).(*txBuffer)
	return buf, ok
}

// fakeTxRunner は port.TxRunner を満たす。fn が完走したときのみ、fn 内で登録された保留書き込みを
// 実データへ反映する (ロールバック相当の再現)。
type fakeTxRunner struct{}

func (fakeTxRunner) RunInTx(ctx context.Context, fn func(ctx context.Context) error) error {
	buf := &txBuffer{}
	if err := fn(withTxBuffer(ctx, buf)); err != nil {
		return err
	}
	for _, apply := range buf.pending {
		apply()
	}
	return nil
}

// fakeFactionRepo は port.FactionRepo を満たすメモリ内 fake。
type fakeFactionRepo struct {
	factions map[string][]string
	addErr   error
}

func newFakeFactionRepo() *fakeFactionRepo {
	return &fakeFactionRepo{factions: map[string][]string{}}
}

func (r *fakeFactionRepo) AddPlayerFaction(ctx context.Context, playerID, faction string) error {
	if r.addErr != nil {
		return r.addErr
	}
	commit := func() {
		for _, f := range r.factions[playerID] {
			if f == faction {
				return
			}
		}
		r.factions[playerID] = append(r.factions[playerID], faction)
	}
	if buf, ok := txBufferFrom(ctx); ok {
		buf.pending = append(buf.pending, commit)
		return nil
	}
	commit()
	return nil
}

func (r *fakeFactionRepo) GetPlayerFactions(ctx context.Context, playerID string) ([]string, error) {
	return r.factions[playerID], nil
}

func (r *fakeFactionRepo) GetInitialFaction(ctx context.Context, playerID string) (*string, error) {
	return nil, nil
}

func (r *fakeFactionRepo) SetInitialFaction(ctx context.Context, playerID, faction string) error {
	return nil
}

// fakePlayerPremiumRepo は port.PlayerPremiumRepo を満たすメモリ内 fake。
type fakePlayerPremiumRepo struct {
	isPremium map[string]bool
	expiresAt map[string]*time.Time
	updateErr error
}

func newFakePlayerPremiumRepo() *fakePlayerPremiumRepo {
	return &fakePlayerPremiumRepo{isPremium: map[string]bool{}, expiresAt: map[string]*time.Time{}}
}

func (r *fakePlayerPremiumRepo) UpdatePremium(ctx context.Context, playerID string, isPremium bool, expiresAt *time.Time) error {
	if r.updateErr != nil {
		return r.updateErr
	}
	commit := func() {
		r.isPremium[playerID] = isPremium
		r.expiresAt[playerID] = expiresAt
	}
	if buf, ok := txBufferFrom(ctx); ok {
		buf.pending = append(buf.pending, commit)
		return nil
	}
	commit()
	return nil
}

// fakeProcessedEventRepo は port.ProcessedEventRepo を満たすメモリ内 fake。
type fakeProcessedEventRepo struct {
	processed map[string]bool
}

func newFakeProcessedEventRepo() *fakeProcessedEventRepo {
	return &fakeProcessedEventRepo{processed: map[string]bool{}}
}

func (r *fakeProcessedEventRepo) Insert(ctx context.Context, eventID, eventType string) (bool, error) {
	if r.processed[eventID] {
		return false, nil
	}
	commit := func() { r.processed[eventID] = true }
	if buf, ok := txBufferFrom(ctx); ok {
		buf.pending = append(buf.pending, commit)
		return true, nil
	}
	commit()
	return true, nil
}

// fakeApplier は OnboardingNameSetApplier / OnboardingFactionSetApplier / OnboardingCompletedApplier
// をまとめて満たすメモリ内 fake。呼び出し引数を記録し、戻り値をテストケースごとに差し替えられる。
type fakeApplier struct {
	processed    bool
	err          error
	calledWith   []string
	requireEmpty bool
}

func (f *fakeApplier) ApplyNameSet(ctx context.Context, eventID, eventType, playerID, name string) (bool, error) {
	f.calledWith = []string{eventID, eventType, playerID, name}
	if f.requireEmpty {
		return false, errors.New("should not be called")
	}
	return f.processed, f.err
}

func (f *fakeApplier) ApplyFactionSet(ctx context.Context, eventID, eventType, playerID, initialFactionID string) (bool, error) {
	f.calledWith = []string{eventID, eventType, playerID, initialFactionID}
	if f.requireEmpty {
		return false, errors.New("should not be called")
	}
	return f.processed, f.err
}

func (f *fakeApplier) ApplyCompleted(ctx context.Context, eventID, eventType, playerID string) (bool, error) {
	f.calledWith = []string{eventID, eventType, playerID}
	if f.requireEmpty {
		return false, errors.New("should not be called")
	}
	return f.processed, f.err
}
