package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// FactionAcquiredSubscriber は faction-acquired イベントを消費し、
// player_factions に is_initial=FALSE 行を追加する。initial への昇格はここでは行わない。
type FactionAcquiredSubscriber struct {
	factionRepo port.FactionRepo
	txRunner    port.TxRunner
	eventRepo   port.ProcessedEventRepo
}

// NewFactionAcquiredSubscriber は FactionAcquiredSubscriber を生成する。
func NewFactionAcquiredSubscriber(
	factionRepo port.FactionRepo,
	txRunner port.TxRunner,
	eventRepo port.ProcessedEventRepo,
) *FactionAcquiredSubscriber {
	return &FactionAcquiredSubscriber{
		factionRepo: factionRepo,
		txRunner:    txRunner,
		eventRepo:   eventRepo,
	}
}

// HandleMessage は 1 件の faction-acquired イベントを処理する。port.MessageHandler を満たし、
// push 受け口 (internal/handler/pubsubpush) から呼ばれる。
func (s *FactionAcquiredSubscriber) HandleMessage(ctx context.Context, data []byte) error {
	var event apishop.FactionAcquiredEvent
	if err := json.Unmarshal(data, &event); err != nil {
		slog.Error("faction-acquired subscriber: bad payload", "error", err)
		return fmt.Errorf("faction-acquired: bad payload: %w", err)
	}
	if event.EventType != apishop.EventTypeFactionAcquired {
		slog.Warn("faction-acquired subscriber: unknown event_type, skipping", "event_type", event.EventType)
		return nil
	}

	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		inserted, err := s.eventRepo.Insert(txCtx, event.EventID, event.EventType)
		if err != nil {
			return fmt.Errorf("insert processed_events: %w", err)
		}
		if !inserted {
			return nil
		}

		if err := s.factionRepo.AddPlayerFaction(txCtx, event.PlayerID, event.Faction); err != nil {
			return fmt.Errorf("add player_faction: %w", err)
		}
		return nil
	}); err != nil {
		slog.Error("faction-acquired subscriber: handler failed",
			"event_id", event.EventID, "player_id", event.PlayerID, "error", err)
		return fmt.Errorf("faction-acquired: handler failed: %w", err)
	}
	return nil
}
