package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"cloud.google.com/go/pubsub/v2"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// PlayerOnboardedSubscriber は player-onboarded-account-sub からイベントを取得し、
// プレイヤーがオンボーディング中に入力した表示名を account.players.username に反映します。
//
// scenario は同じオンボーディング完了に対して player-onboarded と faction-selected を
// 独立に publish する契約 (ADR-021 §5.2)。本 subscriber は faction の反映を行わず、
// faction-selected subscriber と event_id 単位の processed_events ガードで冪等性を担保する。
//
// account.players.username カラムは「表示名」として位置付けられており (db/schema.sql の
// コメント参照)、ADR-021 の event payload 上 display_name として送られてくる文字列を
// そのまま書き込む。列名のリネームは行わず SSoT を一本化する。
type PlayerOnboardedSubscriber struct {
	client     *pubsub.Client
	subscriber *pubsub.Subscriber
	playerRepo port.PlayerRepo
	txRunner   port.TxRunner
	eventRepo  port.ProcessedEventRepo
}

// NewPlayerOnboardedSubscriber は PlayerOnboardedSubscriber を生成します。
func NewPlayerOnboardedSubscriber(
	ctx context.Context,
	projectID, subscriptionID string,
	playerRepo port.PlayerRepo,
	txRunner port.TxRunner,
	eventRepo port.ProcessedEventRepo,
) (*PlayerOnboardedSubscriber, error) {
	if projectID == "" || subscriptionID == "" {
		return nil, errors.New("player-onboarded subscriber: projectID and subscriptionID are required")
	}
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("player-onboarded subscriber: new client: %w", err)
	}
	return &PlayerOnboardedSubscriber{
		client:     client,
		subscriber: client.Subscriber(subscriptionID),
		playerRepo: playerRepo,
		txRunner:   txRunner,
		eventRepo:  eventRepo,
	}, nil
}

// Start は ctx がキャンセルされるか Receive がエラーを返すまでブロックします。
func (s *PlayerOnboardedSubscriber) Start(ctx context.Context) error {
	slog.Info("player-onboarded subscriber: pulling", "subscription", s.subscriber.ID())
	return s.subscriber.Receive(ctx, s.handle)
}

// Close は Pub/Sub クライアントを閉じます。
func (s *PlayerOnboardedSubscriber) Close() error { return s.client.Close() }

func (s *PlayerOnboardedSubscriber) handle(ctx context.Context, msg *pubsub.Message) {
	var ev apiscenario.PlayerOnboardedEvent
	if err := json.Unmarshal(msg.Data, &ev); err != nil {
		slog.Error("player-onboarded subscriber: bad payload (nack)", "error", err)
		msg.Nack()
		return
	}
	if ev.EventType != apiscenario.EventTypePlayerOnboarded {
		slog.Warn("player-onboarded subscriber: unknown event_type, acking", "event_type", ev.EventType)
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
		// UpdateUsername は *Player を返すが subscriber 側は不要なので破棄する。
		// player が存在しない場合は port.ErrNotFound が返り、NACK → Pub/Sub リトライに
		// なる。オンボーディングは必ず Register 後に走る前提なので、ErrNotFound は
		// 一時的な race か発行側バグであり握りつぶさない。
		if _, err := s.playerRepo.UpdateUsername(txCtx, ev.PlayerID, ev.DisplayName); err != nil {
			return fmt.Errorf("update username: %w", err)
		}
		return nil
	}); err != nil {
		slog.Error("player-onboarded subscriber: handler failed",
			"event_id", ev.EventID, "player_id", ev.PlayerID, "error", err)
		msg.Nack()
		return
	}
	msg.Ack()
}
