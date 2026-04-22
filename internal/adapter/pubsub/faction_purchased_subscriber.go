// Package pubsub は account サービスの Pub/Sub subscriber を管理します。
//
// 各 subscriber は exactly-once subscription からイベントを取得し、
// event_id をキーとした冪等トランザクション内で account スキーマに書き込みます。
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
	"github.com/kenyamaneko/overload-party-account/internal/service"
)

// FactionPurchasedSubscriber は faction-purchased-account-sub からイベントを取得し、
// player_factions への INSERT を行います。
//
// ショップ購入は「ロスターへの追加」のみを意味し、players.selected_faction は
// 変更しない (ADR-022)。アクティブ切り替えは PUT /players/:id/faction の
// 独立した REST で行う。
type FactionPurchasedSubscriber struct {
	client      *pubsub.Client
	subscriber  *pubsub.Subscriber
	factionRepo port.FactionRepo
	txRunner    port.TxRunner
	eventRepo   port.ProcessedEventRepo
}

// NewFactionPurchasedSubscriber は FactionPurchasedSubscriber を生成します。
func NewFactionPurchasedSubscriber(
	ctx context.Context,
	projectID, subscriptionID string,
	factionRepo port.FactionRepo,
	txRunner port.TxRunner,
	eventRepo port.ProcessedEventRepo,
) (*FactionPurchasedSubscriber, error) {
	if projectID == "" || subscriptionID == "" {
		return nil, errors.New("faction-purchased subscriber: projectID and subscriptionID are required")
	}
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("faction-purchased subscriber: new client: %w", err)
	}
	return &FactionPurchasedSubscriber{
		client:      client,
		subscriber:  client.Subscriber(subscriptionID),
		factionRepo: factionRepo,
		txRunner:    txRunner,
		eventRepo:   eventRepo,
	}, nil
}

// Start は ctx がキャンセルされるか Receive がエラーを返すまでブロックします。
func (s *FactionPurchasedSubscriber) Start(ctx context.Context) error {
	slog.Info("faction-purchased subscriber: pulling", "subscription", s.subscriber.ID())
	return s.subscriber.Receive(ctx, s.handle)
}

// Close は Pub/Sub クライアントを閉じます。
func (s *FactionPurchasedSubscriber) Close() error { return s.client.Close() }

func (s *FactionPurchasedSubscriber) handle(ctx context.Context, msg *pubsub.Message) {
	if ack := s.processEvent(ctx, msg.Data); ack {
		msg.Ack()
	} else {
		msg.Nack()
	}
}

// processEvent は Pub/Sub ペイロードを処理し、ack すべきか (true) / nack すべきか
// (false) を返す。*pubsub.Message への依存を handle に閉じ込めるため、ビジネス
// ロジックはこちらに集約する。
func (s *FactionPurchasedSubscriber) processEvent(ctx context.Context, data []byte) (ack bool) {
	var ev apishop.FactionPurchasedEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		slog.Error("faction-purchased subscriber: bad payload (nack)", "error", err)
		return false
	}
	if ev.EventType != apishop.EventTypeFactionPurchased {
		slog.Warn("faction-purchased subscriber: unknown event_type, acking", "event_type", ev.EventType)
		return true
	}

	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		inserted, err := s.eventRepo.Insert(txCtx, ev.EventID, ev.EventType)
		if err != nil {
			return fmt.Errorf("insert processed_events: %w", err)
		}
		if !inserted {
			return nil // idempotent ack
		}

		// shop 購入起因のみを扱う契約なので player_factions.source は FactionSourceShopPurchase 固定。
		if err := s.factionRepo.AddPlayerFaction(txCtx, ev.PlayerID, ev.Faction, service.FactionSourceShopPurchase); err != nil {
			return fmt.Errorf("add player_faction: %w", err)
		}
		return nil
	}); err != nil {
		slog.Error("faction-purchased subscriber: handler failed",
			"event_id", ev.EventID, "player_id", ev.PlayerID, "error", err)
		return false
	}
	return true
}
