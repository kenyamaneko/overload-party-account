package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"cloud.google.com/go/pubsub/v2"

	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// PremiumUpdatedSubscriber は premium-updated-account-sub からイベントを取得し、
// players.is_premium と premium_expires_at を更新します。
type PremiumUpdatedSubscriber struct {
	client     *pubsub.Client
	subscriber *pubsub.Subscriber
	playerRepo port.PlayerRepo
	txRunner   port.TxRunner
	eventRepo  port.ProcessedEventRepo
}

// NewPremiumUpdatedSubscriber は PremiumUpdatedSubscriber を生成します。
func NewPremiumUpdatedSubscriber(
	ctx context.Context,
	projectID, subscriptionID string,
	playerRepo port.PlayerRepo,
	txRunner port.TxRunner,
	eventRepo port.ProcessedEventRepo,
) (*PremiumUpdatedSubscriber, error) {
	if projectID == "" || subscriptionID == "" {
		return nil, errors.New("premium-updated subscriber: projectID and subscriptionID are required")
	}
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("premium-updated subscriber: new client: %w", err)
	}
	return &PremiumUpdatedSubscriber{
		client:     client,
		subscriber: client.Subscriber(subscriptionID),
		playerRepo: playerRepo,
		txRunner:   txRunner,
		eventRepo:  eventRepo,
	}, nil
}

// Start は ctx がキャンセルされるか Receive がエラーを返すまでブロックします。
func (s *PremiumUpdatedSubscriber) Start(ctx context.Context) error {
	slog.Info("premium-updated subscriber: pulling", "subscription", s.subscriber.ID())
	return s.subscriber.Receive(ctx, s.handle)
}

// Close は Pub/Sub クライアントを閉じます。
func (s *PremiumUpdatedSubscriber) Close() error { return s.client.Close() }

func (s *PremiumUpdatedSubscriber) handle(ctx context.Context, msg *pubsub.Message) {
	if ack := s.processEvent(ctx, msg.Data); ack {
		msg.Ack()
	} else {
		msg.Nack()
	}
}

// processEvent は Pub/Sub ペイロードを処理し、ack すべきか (true) / nack すべきか
// (false) を返す。*pubsub.Message への依存を handle に閉じ込めるため、ビジネス
// ロジックはこちらに集約する。
func (s *PremiumUpdatedSubscriber) processEvent(ctx context.Context, data []byte) (ack bool) {
	var ev apishop.PremiumUpdatedEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		slog.Error("premium-updated subscriber: bad payload (nack)", "error", err)
		return false
	}
	if ev.EventType != apishop.EventTypePremiumUpdated {
		slog.Warn("premium-updated subscriber: unknown event_type, acking", "event_type", ev.EventType)
		return true
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
		return false
	}
	return true
}
