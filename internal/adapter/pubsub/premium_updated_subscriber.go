package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	"cloud.google.com/go/pubsub"

	pubsubevents "github.com/kenyamaneko/overload-party-common/packages/pubsub-events"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// PremiumUpdatedSubscriber は premium-updated-account-sub からイベントを取得し、
// players.is_premium と premium_expires_at を更新します。
type PremiumUpdatedSubscriber struct {
	client       *pubsub.Client
	subscription *pubsub.Subscription
	playerRepo   port.PlayerRepo
	txRunner     port.TxRunner
	eventRepo    port.ProcessedEventRepo
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
	sub := client.Subscription(subscriptionID)
	ok, err := sub.Exists(ctx)
	if err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("premium-updated subscriber: exists check: %w", err)
	}
	if !ok {
		_ = client.Close()
		return nil, fmt.Errorf("premium-updated subscriber: subscription %q not found", subscriptionID)
	}
	return &PremiumUpdatedSubscriber{
		client:       client,
		subscription: sub,
		playerRepo:   playerRepo,
		txRunner:     txRunner,
		eventRepo:    eventRepo,
	}, nil
}

// Start は ctx がキャンセルされるか Receive がエラーを返すまでブロックします。
func (s *PremiumUpdatedSubscriber) Start(ctx context.Context) error {
	log.Printf("premium-updated subscriber: pulling from %s", s.subscription.ID())
	return s.subscription.Receive(ctx, s.handle)
}

// Close は Pub/Sub クライアントを閉じます。
func (s *PremiumUpdatedSubscriber) Close() error { return s.client.Close() }

func (s *PremiumUpdatedSubscriber) handle(ctx context.Context, msg *pubsub.Message) {
	var ev pubsubevents.PremiumUpdatedEvent
	if err := json.Unmarshal(msg.Data, &ev); err != nil {
		log.Printf("premium-updated subscriber: bad payload (nack): %v", err)
		msg.Nack()
		return
	}
	if ev.EventType != pubsubevents.EventTypePremiumUpdated {
		log.Printf("premium-updated subscriber: unknown event_type %q, acking", ev.EventType)
		msg.Ack()
		return
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
		log.Printf("premium-updated subscriber: handler failed event=%s player=%s: %v",
			ev.EventID, ev.PlayerID, err)
		msg.Nack()
		return
	}
	msg.Ack()
}
