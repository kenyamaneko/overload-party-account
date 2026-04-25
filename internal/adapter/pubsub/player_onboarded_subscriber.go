package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// OnboardingApplier は player-onboarded イベントを受けたときに呼ぶサービス層の
// ユースケース境界です。subscriber は repo を直接触らず、本インターフェース経由で
// service.FactionService.ApplyOnboardingResult のみを利用します。
// クリーンアーキテクチャの依存方向 (adapter → service → port → repo) を崩さないため、
// adapter 層は service の具象ではなくここで切った最小インターフェースに依存します。
//
// processed_events 冪等ガードと name / player_factions / selected_faction の
// 書き込みは同一 Tx で行う必要があるため、Tx 境界は service 側が保持します。
type OnboardingApplier interface {
	ApplyOnboardingResult(
		ctx context.Context,
		eventID, eventType, playerID, displayName, initialFactionID string,
	) (processed, selected bool, err error)
}

// PlayerOnboardedSubscriber は player-onboarded subscription からイベントを取得し、
// オンボーディング完了時の identity (表示名) と初期 faction 所持 / 選択を
// 単一トランザクションで反映します。
//
// 本 subscriber は initial_selection 相当の副作用 (player_factions INSERT +
// players.selected_faction UPDATE) も担っており、shop 購入起因のロスター追加
// (FactionPurchasedSubscriber) とは別系統として同時並行で走ります。
//
// subscriber 自身は Unmarshal + OnboardingApplier への委譲のみを行い、
// SelectableFactions 検証・冪等ガード・同一 Tx 反映は service 層に任せます。
//
// account.players の表示名はカラム name に書き込みます。
// scenario の event payload は display_name という名前ですが、account の
// DB は「表示名」= name という既存契約のため、列名は変更せず
// subscriber handler 内でマッピングを閉じています (ARCHITECTURE.md §6.3)。
type PlayerOnboardedSubscriber struct {
	stream  port.MessageStream
	applier OnboardingApplier
}

// NewPlayerOnboardedSubscriber は PlayerOnboardedSubscriber を生成します。
// applier は service.FactionService (または OnboardingApplier を満たす fake) を渡す想定です。
func NewPlayerOnboardedSubscriber(stream port.MessageStream, applier OnboardingApplier) *PlayerOnboardedSubscriber {
	return &PlayerOnboardedSubscriber{stream: stream, applier: applier}
}

// Start は ctx がキャンセルされるか stream がエラーを返すまでブロックします。
func (s *PlayerOnboardedSubscriber) Start(ctx context.Context) error {
	slog.Info("player-onboarded subscriber: consuming")
	return s.stream.Consume(ctx, s.processEvent)
}

// processEvent は 1 イベントを処理する。戻り値 nil = ack、非 nil = nack。
func (s *PlayerOnboardedSubscriber) processEvent(ctx context.Context, data []byte) error {
	var ev apiscenario.PlayerOnboardedEvent
	if err := json.Unmarshal(data, &ev); err != nil {
		slog.Error("player-onboarded subscriber: bad payload (nack)", "error", err)
		return fmt.Errorf("player-onboarded: bad payload: %w", err)
	}
	if ev.EventType != apiscenario.EventTypePlayerOnboarded {
		slog.Warn("player-onboarded subscriber: unknown event_type, acking", "event_type", ev.EventType)
		return nil
	}
	if ev.InitialFactionID == "" {
		slog.Error("player-onboarded subscriber: missing initial_faction_id (nack)",
			"event_id", ev.EventID, "player_id", ev.PlayerID)
		return fmt.Errorf("player-onboarded: missing initial_faction_id")
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
			return fmt.Errorf("player-onboarded: player not found: %w", err)
		}
		slog.Error("player-onboarded subscriber: apply onboarding failed",
			"event_id", ev.EventID, "player_id", ev.PlayerID, "error", err)
		return fmt.Errorf("player-onboarded: apply onboarding: %w", err)
	}
	if !processed {
		// 冪等スキップ (processed_events に既存)。副作用なしで ACK。
		return nil
	}
	if !selected {
		// selected_faction は既に埋まっていた。name / player_factions は反映済み。
		// 二重配信 or processed_events を迂回した重複なので警告ログのみ残す。
		slog.Warn("player-onboarded subscriber: selected_faction already set",
			"event_id", ev.EventID, "player_id", ev.PlayerID, "faction", ev.InitialFactionID)
	}
	return nil
}
