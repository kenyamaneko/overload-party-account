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

// OnboardingApplier は player-onboarded イベントを受けたときに呼ぶサービス層の
// ユースケース境界です。subscriber は repo を直接触らず、本インターフェース経由で
// service.FactionService.ApplyOnboardingResult のみを利用します。
// クリーンアーキテクチャの依存方向 (adapter → service → port → repo) を崩さないため、
// adapter 層は service の具象ではなくここで切った最小インターフェースに依存します。
//
// processed_events 冪等ガードと username / player_factions / selected_faction の
// 書き込みは同一 Tx で行う必要があるため、Tx 境界は service 側が保持します。
type OnboardingApplier interface {
	ApplyOnboardingResult(
		ctx context.Context,
		eventID, eventType, playerID, displayName, initialFactionID string,
	) (processed, selected bool, err error)
}

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
// subscriber 自身は Unmarshal + OnboardingApplier への委譲のみを行い、
// SelectableFactions 検証・冪等ガード・同一 Tx 反映は service 層に任せます。
//
// account.players の表示名は既存カラム username に書き込みます。
// scenario の event payload は display_name という名前ですが、account の
// DB は「表示名」= username という既存契約のため、列名は変更せず
// subscriber handler 内でマッピングを閉じています (ARCHITECTURE.md §6.3)。
type PlayerOnboardedSubscriber struct {
	client     *pubsub.Client
	subscriber *pubsub.Subscriber
	applier    OnboardingApplier
}

// NewPlayerOnboardedSubscriber は PlayerOnboardedSubscriber を生成します。
// applier は service.FactionService (または OnboardingApplier を満たす fake) を渡す想定です。
func NewPlayerOnboardedSubscriber(
	ctx context.Context,
	projectID, subscriptionID string,
	applier OnboardingApplier,
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
		applier:    applier,
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
// (false) を返す。*pubsub.Message への依存を handle に閉じ込めるため、
// 事前バリデーションと service 呼び出しはこちらに集約する。
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

	processed, selected, err := s.applier.ApplyOnboardingResult(
		ctx, ev.EventID, ev.EventType, ev.PlayerID, ev.DisplayName, ev.InitialFactionID,
	)
	if err != nil {
		// Register 未実施プレイヤーに対する onboarded event は publisher 側 (scenario)
		// のバグであり、リトライしても解決しない構造。将来的には DLQ に流して運用
		// 監視する方針だが、現状 DLQ 未整備のため NACK で明示的に失敗を可視化する
		// (silent drop だと SLO 的にも発見が遅れる)。ログには必ず event_id と
		// player_id を含め、publisher 起源の不整合であることを severity=ERROR で
		// 記録する。
		if errors.Is(err, port.ErrNotFound) {
			slog.Error(
				"player-onboarded subscriber: player not found (publisher bug: onboarded for unregistered player)",
				"event_id", ev.EventID,
				"player_id", ev.PlayerID,
				"error", err,
			)
			return false
		}
		slog.Error("player-onboarded subscriber: apply onboarding failed",
			"event_id", ev.EventID, "player_id", ev.PlayerID, "error", err)
		return false
	}
	if !processed {
		// 冪等スキップ (processed_events に既存)。副作用なしで ACK。
		return true
	}
	if !selected {
		// selected_faction は既に埋まっていた。username / player_factions は反映済み。
		// 二重配信 or processed_events を迂回した重複なので警告ログのみ残す。
		slog.Warn("player-onboarded subscriber: selected_faction already set",
			"event_id", ev.EventID, "player_id", ev.PlayerID, "faction", ev.InitialFactionID)
	}
	return true
}
