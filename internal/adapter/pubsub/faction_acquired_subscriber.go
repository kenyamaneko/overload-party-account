package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// FactionAcquiredSubscriber は faction-acquired subscription を消費し、
// player_factions に is_initial=FALSE 行を追加する。initial への昇格はここでは行わない。
type FactionAcquiredSubscriber struct {
	stream      port.MessageStream
	factionRepo port.FactionRepo
	txRunner    port.TxRunner
	eventRepo   port.ProcessedEventRepo
}

// NewFactionAcquiredSubscriber は FactionAcquiredSubscriber を生成する。
func NewFactionAcquiredSubscriber(
	stream port.MessageStream,
	factionRepo port.FactionRepo,
	txRunner port.TxRunner,
	eventRepo port.ProcessedEventRepo,
) *FactionAcquiredSubscriber {
	return &FactionAcquiredSubscriber{
		stream:      stream,
		factionRepo: factionRepo,
		txRunner:    txRunner,
		eventRepo:   eventRepo,
	}
}

// Start は ctx がキャンセルされるか stream がエラーを返すまでブロックする。
func (s *FactionAcquiredSubscriber) Start(ctx context.Context) error {
	slog.Info("faction-acquired subscriber: consuming")
	return s.stream.Consume(ctx, s.processEvent)
}

func (s *FactionAcquiredSubscriber) processEvent(ctx context.Context, data []byte) error {
	var ev apishop.FactionAcquiredEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		slog.Error("faction-acquired subscriber: bad payload (nack)", "error", err)
		return fmt.Errorf("faction-acquired: bad payload: %w", err)
	}
	if ev.EventType != apishop.EventTypeFactionAcquired {
		slog.Warn("faction-acquired subscriber: unknown event_type, acking", "event_type", ev.EventType)
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
		slog.Error("faction-acquired subscriber: handler failed",
			"event_id", ev.EventID, "player_id", ev.PlayerID, "error", err)
		return fmt.Errorf("faction-acquired: handler failed: %w", err)
	}
	return nil
}
