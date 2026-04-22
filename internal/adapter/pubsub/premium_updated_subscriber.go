package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// PremiumUpdatedSubscriber は premium-updated subscription からイベントを取得し、
// players.is_premium と premium_expires_at を更新します。
type PremiumUpdatedSubscriber struct {
	stream     port.MessageStream
	playerRepo port.PlayerRepo
	txRunner   port.TxRunner
	eventRepo  port.ProcessedEventRepo
}

// NewPremiumUpdatedSubscriber は PremiumUpdatedSubscriber を生成します。
func NewPremiumUpdatedSubscriber(
	stream port.MessageStream,
	playerRepo port.PlayerRepo,
	txRunner port.TxRunner,
	eventRepo port.ProcessedEventRepo,
) *PremiumUpdatedSubscriber {
	return &PremiumUpdatedSubscriber{
		stream:     stream,
		playerRepo: playerRepo,
		txRunner:   txRunner,
		eventRepo:  eventRepo,
	}
}

// Start は ctx がキャンセルされるか stream がエラーを返すまでブロックします。
func (s *PremiumUpdatedSubscriber) Start(ctx context.Context) error {
	slog.Info("premium-updated subscriber: consuming")
	return s.stream.Consume(ctx, s.processEvent)
}

// processEvent は 1 イベントを処理する。戻り値 nil = ack、非 nil = nack。
func (s *PremiumUpdatedSubscriber) processEvent(ctx context.Context, data []byte) error {
	var ev apishop.PremiumUpdatedEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		slog.Error("premium-updated subscriber: bad payload (nack)", "error", err)
		return fmt.Errorf("premium-updated: bad payload: %w", err)
	}
	if ev.EventType != apishop.EventTypePremiumUpdated {
		slog.Warn("premium-updated subscriber: unknown event_type, acking", "event_type", ev.EventType)
		return nil
	}

	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		inserted, err := s.eventRepo.Insert(txCtx, ev.EventID, ev.EventType)
		if err != nil {
			return fmt.Errorf("insert processed_events: %w", err)
		}
		if !inserted {
			return nil
		}
		if err := s.playerRepo.UpdatePremium(txCtx, ev.PlayerID, ev.IsPremium, ev.PremiumExpiresAt); err != nil {
			return fmt.Errorf("update premium: %w", err)
		}
		return nil
	}); err != nil {
		slog.Error("premium-updated subscriber: handler failed",
			"event_id", ev.EventID, "player_id", ev.PlayerID, "error", err)
		return fmt.Errorf("premium-updated: handler failed: %w", err)
	}
	return nil
}
