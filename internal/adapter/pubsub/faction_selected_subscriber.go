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

	pubsubevents "github.com/kenyamaneko/overload-party-common/packages/pubsub-events"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// FactionSelectedSubscriber は faction-selected-account-sub からイベントを取得し、
// player_factions への INSERT と初期選択時の players.selected_faction 更新を行います。
type FactionSelectedSubscriber struct {
	client      *pubsub.Client
	subscriber  *pubsub.Subscriber
	playerRepo  port.PlayerRepo
	factionRepo port.FactionRepo
	txRunner    port.TxRunner
	eventRepo   port.ProcessedEventRepo
}

// NewFactionSelectedSubscriber は FactionSelectedSubscriber を生成します。
func NewFactionSelectedSubscriber(
	ctx context.Context,
	projectID, subscriptionID string,
	playerRepo port.PlayerRepo,
	factionRepo port.FactionRepo,
	txRunner port.TxRunner,
	eventRepo port.ProcessedEventRepo,
) (*FactionSelectedSubscriber, error) {
	if projectID == "" || subscriptionID == "" {
		return nil, errors.New("faction-selected subscriber: projectID and subscriptionID are required")
	}
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("faction-selected subscriber: new client: %w", err)
	}
	return &FactionSelectedSubscriber{
		client:      client,
		subscriber:  client.Subscriber(subscriptionID),
		playerRepo:  playerRepo,
		factionRepo: factionRepo,
		txRunner:    txRunner,
		eventRepo:   eventRepo,
	}, nil
}

// Start は ctx がキャンセルされるか Receive がエラーを返すまでブロックします。
func (s *FactionSelectedSubscriber) Start(ctx context.Context) error {
	slog.Info("faction-selected subscriber: pulling", "subscription", s.subscriber.ID())
	return s.subscriber.Receive(ctx, s.handle)
}

// Close は Pub/Sub クライアントを閉じます。
func (s *FactionSelectedSubscriber) Close() error { return s.client.Close() }

func (s *FactionSelectedSubscriber) handle(ctx context.Context, msg *pubsub.Message) {
	var ev pubsubevents.FactionSelectedEvent
	if err := json.Unmarshal(msg.Data, &ev); err != nil {
		slog.Error("faction-selected subscriber: bad payload (nack)", "error", err)
		msg.Nack()
		return
	}
	if ev.EventType != pubsubevents.EventTypeFactionSelected {
		slog.Warn("faction-selected subscriber: unknown event_type, acking", "event_type", ev.EventType)
		msg.Ack()
		return
	}

	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		inserted, err := s.eventRepo.Insert(txCtx, ev.EventID, ev.EventType)
		if err != nil {
			return fmt.Errorf("insert processed_events: %w", err)
		}
		if !inserted {
			return nil // idempotent ack
		}

		dbSource := mapFactionSource(ev.Source)

		if err := s.factionRepo.AddPlayerFaction(txCtx, ev.PlayerID, ev.Faction, dbSource); err != nil {
			return fmt.Errorf("add player_faction: %w", err)
		}

		// 初期選択のみ selected_faction を更新する。
		// ショップ購入はロスターに追加するだけで、切り替えはプレイヤー操作。
		if ev.Source == pubsubevents.FactionSourceScenarioInitial {
			if err := s.playerRepo.UpdateFaction(txCtx, ev.PlayerID, ev.Faction); err != nil {
				return fmt.Errorf("update selected_faction: %w", err)
			}
		}
		return nil
	}); err != nil {
		slog.Error("faction-selected subscriber: handler failed",
			"event_id", ev.EventID, "player_id", ev.PlayerID, "error", err)
		msg.Nack()
		return
	}
	msg.Ack()
}

func mapFactionSource(source string) string {
	switch source {
	case pubsubevents.FactionSourceScenarioInitial:
		return "initial_selection"
	case pubsubevents.FactionSourceShopPurchase:
		return "shop_purchase"
	default:
		return source
	}
}
