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
// オンボーディング完了時の identity (表示名) と初期 faction 所持 / 選択を
// 単一トランザクションで反映します。
//
// ADR-022 により FactionSelectedEvent (scenario_initial) が廃止されたため、
// かつて faction-selected subscriber が担っていた initial_selection 相当の
// 副作用 (player_factions INSERT + players.selected_faction UPDATE) は
// 本 subscriber に統合されています。shop 購入起因のロスター追加は
// FactionPurchasedSubscriber が別系統で処理します。
//
// account.players の表示名は既存カラム username に書き込みます。
// scenario の event payload は display_name という名前ですが、account の
// DB は「表示名」= username という既存契約のため、列名は変更せず
// subscriber handler 内でマッピングを閉じています (ARCHITECTURE.md §6.3)。
type PlayerOnboardedSubscriber struct {
	client      *pubsub.Client
	subscriber  *pubsub.Subscriber
	playerRepo  port.PlayerRepo
	factionRepo port.FactionRepo
	txRunner    port.TxRunner
	eventRepo   port.ProcessedEventRepo
}

// NewPlayerOnboardedSubscriber は PlayerOnboardedSubscriber を生成します。
func NewPlayerOnboardedSubscriber(
	ctx context.Context,
	projectID, subscriptionID string,
	playerRepo port.PlayerRepo,
	factionRepo port.FactionRepo,
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
		client:      client,
		subscriber:  client.Subscriber(subscriptionID),
		playerRepo:  playerRepo,
		factionRepo: factionRepo,
		txRunner:    txRunner,
		eventRepo:   eventRepo,
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
	if ack := s.processEvent(ctx, msg.Data); ack {
		msg.Ack()
	} else {
		msg.Nack()
	}
}

// processEvent は Pub/Sub ペイロードを処理し、ack すべきか (true) / nack すべきか
// (false) を返す。*pubsub.Message への依存を handle に閉じ込めるため、ビジネス
// ロジックはこちらに集約する。
func (s *PlayerOnboardedSubscriber) processEvent(ctx context.Context, data []byte) (ack bool) {
	var ev apiscenario.PlayerOnboardedEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		slog.Error("player-onboarded subscriber: bad payload (nack)", "error", err)
		return false
	}
	if ev.EventType != apiscenario.EventTypePlayerOnboarded {
		slog.Warn("player-onboarded subscriber: unknown event_type, acking", "event_type", ev.EventType)
		return true
	}
	if ev.InitialFactionID == "" {
		slog.Error("player-onboarded subscriber: missing initial_faction_id (nack)",
			"event_id", ev.EventID, "player_id", ev.PlayerID)
		return false
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
		// onboarding 起因のロスター追加は source = initial_selection で記録する。
		// AddPlayerFaction は (player_id, faction) 複合 PK に対する
		// ON CONFLICT DO NOTHING で冪等。
		if err := s.factionRepo.AddPlayerFaction(txCtx, ev.PlayerID, ev.InitialFactionID, "initial_selection"); err != nil {
			return fmt.Errorf("add player_faction: %w", err)
		}
		// selected_faction は NULL のときだけ書き込む。既に選択済みなら無視し、
		// onboarding の二重配信を握りつぶさず警告ログのみ残す (processed_events
		// を迂回した重複か発行側バグ)。
		set, err := s.playerRepo.TrySetInitialFaction(txCtx, ev.PlayerID, ev.InitialFactionID)
		if err != nil {
			return fmt.Errorf("try set initial faction: %w", err)
		}
		if !set {
			slog.Warn("player-onboarded subscriber: selected_faction already set",
				"event_id", ev.EventID, "player_id", ev.PlayerID, "faction", ev.InitialFactionID)
		}
		return nil
	}); err != nil {
		slog.Error("player-onboarded subscriber: handler failed",
			"event_id", ev.EventID, "player_id", ev.PlayerID, "error", err)
		return false
	}
	return true
}
