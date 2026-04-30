// Package pubsub は account サービスの Pub/Sub subscriber を管理します。
//
// 各 subscriber は exactly-once subscription からイベントを取得し、
// event_id をキーとした冪等トランザクション内で account スキーマに書き込みます。
package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// FactionPurchasedSubscriber は faction-purchased subscription からイベントを取得し、
// player_factions への INSERT (is_initial=FALSE 固定) を行います。
//
// ショップ購入は「ロスターへの追加」のみを意味し、initial faction の確定とは独立。
// initial の昇格は scenario の onboarding-faction-set のみが行う。
type FactionPurchasedSubscriber struct {
	stream      port.MessageStream
	factionRepo port.FactionRepo
	txRunner    port.TxRunner
	eventRepo   port.ProcessedEventRepo
}

// NewFactionPurchasedSubscriber は FactionPurchasedSubscriber を生成します。
// subscription 接続は stream に委ねるため、本コンストラクタでは Cloud Pub/Sub SDK に
// 触れない (クリーンアーキテクチャの依存方向遵守)。
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

// Start は ctx がキャンセルされるか stream がエラーを返すまでブロックします。
func (s *FactionPurchasedSubscriber) Start(ctx context.Context) error {
	slog.Info("faction-purchased subscriber: consuming")
	return s.stream.Consume(ctx, s.processEvent)
}

// processEvent は 1 イベントを処理する。戻り値 nil = ack、非 nil = nack。
//
// bad payload / handler 失敗は nack で再配信させる。unknown event_type は
// 責務外として ack し、pub/sub に次のメッセージを渡す。
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
			return nil // idempotent ack
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
