package usecase

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// OnboardingInteractor はオンボーディングの進行を表すイベントを反映する。
type OnboardingInteractor struct {
	playerRepo     port.PlayerRepo
	onboardingRepo port.PlayerOnboardingRepo
	factionRepo    port.FactionRepo
	eventGuard     *processedEventGuard
}

// NewOnboardingInteractor は OnboardingInteractor を生成する。
func NewOnboardingInteractor(
	playerRepo port.PlayerRepo,
	onboardingRepo port.PlayerOnboardingRepo,
	factionRepo port.FactionRepo,
	eventRepo port.ProcessedEventRepo,
	txRunner port.TxRunner,
) *OnboardingInteractor {
	return &OnboardingInteractor{
		playerRepo:     playerRepo,
		onboardingRepo: onboardingRepo,
		factionRepo:    factionRepo,
		eventGuard:     &processedEventGuard{txRunner: txRunner, eventRepo: eventRepo},
	}
}

// ApplyNameSet は表示名設定イベントを反映して、表示名を設定し、進行状態を name_set に進める。
// 同じイベントIDのイベントを反映済みのときは何もしない。
// 進行状態がすでに先へ進んでいるときは、表示名を変えずに受理する。
func (oi *OnboardingInteractor) ApplyNameSet(
	ctx context.Context,
	eventID, eventType, playerID, name string,
) error {
	if err := domain.ValidateName(name); err != nil {
		return err
	}
	return oi.applyOnboardingStep(ctx, eventID, eventType, playerID, domain.OnboardingStatusNameSet,
		func(txCtx context.Context) error {
			return oi.playerRepo.UpdateName(txCtx, playerID, name)
		})
}

// ApplyFactionSet は陣営設定イベントを反映して、初期陣営を確定し、進行状態を faction_set に進める。
// 同じイベントIDのイベントを反映済みのときは何もしない。
// 初期陣営が別の陣営で確定済みのときは ErrFactionConflict を返す。
// 進行状態がすでに先へ進んでいるときは、初期陣営を変えずに受理する。
func (oi *OnboardingInteractor) ApplyFactionSet(
	ctx context.Context,
	eventID, eventType, playerID, initialFactionID string,
) error {
	if err := validateSelectableFaction(initialFactionID); err != nil {
		return err
	}
	return oi.applyOnboardingStep(ctx, eventID, eventType, playerID, domain.OnboardingStatusFactionSet,
		func(txCtx context.Context) error {
			existing, err := confirmInitialFaction(txCtx, oi.factionRepo, playerID, initialFactionID)
			if err != nil {
				return err
			}
			if existing != nil && *existing != initialFactionID {
				return fmt.Errorf("%w: existing=%s incoming=%s", ErrFactionConflict, *existing, initialFactionID)
			}
			return nil
		})
}

// ApplyCompleted はオンボーディング完了イベントを反映して、進行状態を completed に進める。
// 同じイベントIDのイベントを反映済みのときは何もしない。
func (oi *OnboardingInteractor) ApplyCompleted(
	ctx context.Context,
	eventID, eventType, playerID string,
) error {
	return oi.applyOnboardingStep(ctx, eventID, eventType, playerID, domain.OnboardingStatusCompleted,
		func(context.Context) error { return nil })
}

// applyOnboardingStep は、進行状態を target に進められるときだけ、イベントの内容を書き込んで進行状態を更新する。
// 進行状態が段階を飛ばして進むときは domain.ErrOnboardingStatusSkipped を返し、何も書き込まない。
// 進行状態が後退するときは警告ログを残し、何も書き込まずに受理する。
func (oi *OnboardingInteractor) applyOnboardingStep(
	ctx context.Context,
	eventID, eventType, playerID, target string,
	writeEventContent func(txCtx context.Context) error,
) error {
	return oi.eventGuard.applyOnce(ctx, eventID, eventType, func(txCtx context.Context) error {
		current, err := oi.onboardingRepo.GetOnboardingStatus(txCtx, playerID)
		if err != nil {
			return fmt.Errorf("get onboarding status: %w", err)
		}
		canTransition, err := domain.CanTransitionOnboardingStatus(current, target)
		if err != nil {
			return fmt.Errorf("check onboarding transition %q -> %q: %w", current, target, err)
		}
		if !canTransition {
			slog.WarnContext(txCtx, "onboarding status backward transition skipped",
				"player_id", playerID, "current", current, "target", target)
			return nil
		}
		if err := writeEventContent(txCtx); err != nil {
			return err
		}
		if current == target {
			return nil
		}
		if err := oi.onboardingRepo.UpdateOnboardingStatus(txCtx, playerID, target); err != nil {
			return fmt.Errorf("update onboarding status: %w", err)
		}
		return nil
	})
}

// IsPublisherBug は、イベントの反映に失敗した原因が、イベントの発行元の誤りかを返す。
// 該当するプレイヤーがいない、初期陣営が衝突している、表示名または陣営が不正なときは true を返す。
func IsPublisherBug(err error) bool {
	return errors.Is(err, port.ErrNotFound) ||
		errors.Is(err, ErrFactionConflict) ||
		errors.Is(err, domain.ErrInvalidName) ||
		errors.Is(err, ErrInvalidFaction)
}
