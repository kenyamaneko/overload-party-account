package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// FactionPurchasedSubscriber は faction-purchased subscription を消費し、
// player_factions に is_initial=FALSE 行を追加する。initial への昇格はここでは行わない。
type FactionPurchasedSubscriber struct {
	stream      port.MessageStream
	factionRepo port.FactionRepo
	txRunner    port.TxRunner
	eventRepo   port.ProcessedEventRepo
}

// NewFactionPurchasedSubscriber は FactionPurchasedSubscriber を生成する。
func NewFactionPurchasedSubscriber(
	stream port.MessageStream,
	factionRepo port.FactionRepo,
	txRunner port.TxRunner,
	eventRepo port.ProcessedEventRepo,
) *FactionPurchasedSubscriber {
	return &FactionPurchasedSubscriber{
		stream:      stream,
		factionRepo: factionRepo,
		txRunner:    txRunner,
		eventRepo:   eventRepo,
	}
}

// Start は ctx がキャンセルされるか stream がエラーを返すまでブロックする。
func (s *FactionPurchasedSubscriber) Start(ctx context.Context) error {
	slog.Info("faction-purchased subscriber: consuming")
	return s.stream.Consume(ctx, s.processEvent)
}

func (s *FactionPurchasedSubscriber) processEvent(ctx context.Context, data []byte) error {
	var ev apishop.FactionPurchasedEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		slog.Error("faction-purchased subscriber: bad payload (nack)", "error", err)
		return fmt.Errorf("faction-purchased: bad payload: %w", err)
	}
	if ev.EventType != apishop.EventTypeFactionPurchased {
		slog.Warn("faction-purchased subscriber: unknown event_type, acking", "event_type", ev.EventType)
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

		if err := s.factionRepo.AddPlayerFaction(txCtx, ev.PlayerID, ev.Faction); err != nil {
			return fmt.Errorf("add player_faction: %w", err)
		}
		return nil
	}); err != nil {
		slog.Error("faction-purchased subscriber: handler failed",
			"event_id", ev.EventID, "player_id", ev.PlayerID, "error", err)
		return fmt.Errorf("faction-purchased: handler failed: %w", err)
	}
	return nil
}
