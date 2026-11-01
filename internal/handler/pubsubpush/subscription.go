package pubsubpush

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

// 購読の名前。ログの文面に使う。
const (
	FactionAcquiredSubscription      = "faction-acquired"
	PremiumUpdatedSubscription       = "premium-updated"
	PlayerOnboardedSubscription      = "player-onboarded"
	OnboardingNameSetSubscription    = "onboarding-name-set"
	OnboardingFactionSetSubscription = "onboarding-faction-set"
)

// 購読が失敗を記録するときの原因。ログの文面に使う。
const (
	failureHandlerFailed        = "handler failed"
	failureApplyFailed          = "apply failed"
	failurePublisherBug         = "publisher bug"
	failureMissingRequiredField = "missing required field"
)

var (
	errUnexpectedEventType = errors.New("unexpected event_type")
	errEmptyRequiredField  = errors.New("required field is empty")
	errInvalidUUIDFormat   = errors.New("invalid UUID format")
)

// eventIDs は購読が受け取ったイベントのイベントIDとプレイヤーIDを UUID として読んだ結果である。
type eventIDs struct {
	eventID  string
	playerID string
}

// parseEventIDs はイベントIDとプレイヤーIDを UUID として読み、正規の表記で返す。
func parseEventIDs(eventID, playerID string) (eventIDs, error) {
	parsedEventID, err := uuid.Parse(eventID)
	if err != nil {
		return eventIDs{}, fmt.Errorf("event_id: %w: %w", errInvalidUUIDFormat, err)
	}
	parsedPlayerID, err := uuid.Parse(playerID)
	if err != nil {
		return eventIDs{}, fmt.Errorf("player_id: %w: %w", errInvalidUUIDFormat, err)
	}
	return eventIDs{eventID: parsedEventID.String(), playerID: parsedPlayerID.String()}, nil
}

// verifyRequiredField は必須項目 field の値 value が空でないことを確かめる。
// 空のときは項目名を付けたエラーを返す。
func verifyRequiredField(field, value string) error {
	if value == "" {
		return fmt.Errorf("%s: %w", field, errEmptyRequiredField)
	}
	return nil
}

// verifyEventType はイベント種別が購読が扱う種別と一致することを確かめる。
// 一致しないときは失敗をログに残し、エラーを返す。
func verifyEventType(ctx context.Context, subscription, gotEventType, wantEventType, eventID string) error {
	if gotEventType == wantEventType {
		return nil
	}
	slog.ErrorContext(ctx, subscription+" subscriber: unknown event_type",
		"event_type", gotEventType, "event_id", eventID)
	return fmt.Errorf("%s: %w %q", subscription, errUnexpectedEventType, gotEventType)
}

// reportFailure は購読の失敗をログに残し、購読名と原因を付けたエラーを返す。
func reportFailure(ctx context.Context, subscription, failure string, err error, eventID, playerID string) error {
	slog.ErrorContext(ctx, subscription+" subscriber: "+failure,
		"event_id", eventID, "player_id", playerID, "error", err)
	return fmt.Errorf("%s: %s: %w", subscription, failure, err)
}

// reportApplyFailure はオンボーディングの進行を反映できなかった失敗をログに残す。
// 原因がイベントの発行元の誤りのときは、そうでない失敗と区別して記録する。
func reportApplyFailure(ctx context.Context, subscription string, err error, eventID, playerID string) error {
	if usecase.IsPublisherBug(err) {
		return reportFailure(ctx, subscription, failurePublisherBug, err, eventID, playerID)
	}
	return reportFailure(ctx, subscription, failureApplyFailed, err, eventID, playerID)
}
