package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// PremiumInteractor はプレイヤーのプレミアム状態を更新する。
type PremiumInteractor struct {
	premiumRepo port.PlayerPremiumRepo
	eventGuard  *processedEventGuard
}

// NewPremiumInteractor は PremiumInteractor を生成する。
func NewPremiumInteractor(
	premiumRepo port.PlayerPremiumRepo,
	eventRepo port.ProcessedEventRepo,
	txRunner port.TxRunner,
) *PremiumInteractor {
	return &PremiumInteractor{
		premiumRepo: premiumRepo,
		eventGuard:  &processedEventGuard{txRunner: txRunner, eventRepo: eventRepo},
	}
}

// UpdatePremium はプレミアム状態と有効期限を更新する。expiresAt が nil のときは有効期限なしにする。
func (pr *PremiumInteractor) UpdatePremium(ctx context.Context, playerID string, isPremium bool, expiresAt *time.Time) error {
	return pr.premiumRepo.UpdatePremium(ctx, playerID, isPremium, expiresAt)
}

// ApplyPremiumUpdated はプレミアム状態更新イベントを反映して、プレミアム状態と有効期限を更新する。
// 同じイベントIDのイベントを反映済みのときは何もしない。
// イベントの発生時刻が、そのプレイヤーに最後に反映したイベントの発生時刻より古いときは、更新せずに受理する。
func (pr *PremiumInteractor) ApplyPremiumUpdated(
	ctx context.Context,
	eventID, eventType, playerID string,
	isPremium bool,
	expiresAt *time.Time,
	occurredAt time.Time,
) error {
	return pr.eventGuard.applyOnce(ctx, eventID, eventType, func(txCtx context.Context) error {
		lastOccurredAt, err := pr.premiumRepo.GetLastPremiumEventOccurredAtForUpdate(txCtx, playerID)
		if err != nil {
			return fmt.Errorf("get last premium event occurred at: %w", err)
		}
		if lastOccurredAt != nil && occurredAt.Before(*lastOccurredAt) {
			return nil
		}
		if err := pr.premiumRepo.UpdatePremiumFromEvent(txCtx, playerID, isPremium, expiresAt, occurredAt); err != nil {
			return fmt.Errorf("update premium: %w", err)
		}
		return nil
	})
}
