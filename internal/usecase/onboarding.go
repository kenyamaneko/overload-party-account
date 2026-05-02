package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// OnboardingInteractor はオンボード進行イベントの副作用を 1 Tx で反映する。
// 冪等ガード (processed_events) と業務データの永続化を同 Tx で束ねる責務はここが持つ。
type OnboardingInteractor struct {
	playerRepo  port.PlayerRepo
	factionRepo port.FactionRepo
	eventRepo   port.ProcessedEventRepo
	txRunner    port.TxRunner
}

// NewOnboardingInteractor は OnboardingInteractor を生成する。
func NewOnboardingInteractor(
	playerRepo port.PlayerRepo,
	factionRepo port.FactionRepo,
	eventRepo port.ProcessedEventRepo,
	txRunner port.TxRunner,
) *OnboardingInteractor {
	return &OnboardingInteractor{
		playerRepo:  playerRepo,
		factionRepo: factionRepo,
		eventRepo:   eventRepo,
		txRunner:    txRunner,
	}
}

// ApplyNameSet は onboarding-name-set イベントの副作用を反映する。
// processed=false は重複配信で、呼び出し側は ACK のみで他の副作用を起こさない。
func (s *OnboardingInteractor) ApplyNameSet(
	ctx context.Context,
	eventID, eventType, playerID, name string,
) (processed bool, err error) {
	if err := domain.ValidateName(name); err != nil {
		return false, err
	}

	if txErr := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		inserted, err := s.eventRepo.Insert(txCtx, eventID, eventType)
		if err != nil {
			return fmt.Errorf("insert processed_events: %w", err)
		}
		if !inserted {
			return nil
		}
		processed = true
		if err := s.playerRepo.UpdateName(txCtx, playerID, name); err != nil {
			return err
		}
		return s.advanceOnboardingStatus(txCtx, playerID, domain.OnboardingStatusNameSet)
	}); txErr != nil {
		return false, txErr
	}
	return processed, nil
}

// ApplyFactionSet は onboarding-faction-set イベントの副作用を反映する。
func (s *OnboardingInteractor) ApplyFactionSet(
	ctx context.Context,
	eventID, eventType, playerID, initialFactionID string,
) (processed bool, err error) {
	if err := validateInitialFaction(initialFactionID); err != nil {
		return false, err
	}

	if txErr := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		inserted, err := s.eventRepo.Insert(txCtx, eventID, eventType)
		if err != nil {
			return fmt.Errorf("insert processed_events: %w", err)
		}
		if !inserted {
			return nil
		}
		processed = true

		existing, err := s.factionRepo.GetInitialFaction(txCtx, playerID)
		if err != nil {
			return fmt.Errorf("get initial faction: %w", err)
		}
		// オンボードフロー上、faction-set はプレイヤーにつき 1 度しか届かない想定。
		// 異なる faction で再到着するのは publisher バグなので、上書きせずに
		// ErrFactionConflict を返して subscriber 側で ERROR ログ + NACK させる。
		if existing != nil && *existing != initialFactionID {
			return fmt.Errorf("%w: existing=%s incoming=%s", ErrFactionConflict, *existing, initialFactionID)
		}
		if existing == nil {
			if err := s.factionRepo.SetInitialFaction(txCtx, playerID, initialFactionID); err != nil {
				return fmt.Errorf("set initial faction: %w", err)
			}
		}
		return s.advanceOnboardingStatus(txCtx, playerID, domain.OnboardingStatusFactionSet)
	}); txErr != nil {
		return false, txErr
	}
	return processed, nil
}

// ApplyCompleted は player-onboarded イベントの副作用を反映する。
func (s *OnboardingInteractor) ApplyCompleted(
	ctx context.Context,
	eventID, eventType, playerID string,
) (processed bool, err error) {
	if txErr := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		inserted, err := s.eventRepo.Insert(txCtx, eventID, eventType)
		if err != nil {
			return fmt.Errorf("insert processed_events: %w", err)
		}
		if !inserted {
			return nil
		}
		processed = true
		return s.advanceOnboardingStatus(txCtx, playerID, domain.OnboardingStatusCompleted)
	}); txErr != nil {
		return false, txErr
	}
	return processed, nil
}

// advanceOnboardingStatus は onboarding_status を target に進める。
// 後進方向は out-of-order 配信に対する防御として黙ってスキップする。
func (s *OnboardingInteractor) advanceOnboardingStatus(ctx context.Context, playerID, target string) error {
	current, err := s.playerRepo.GetOnboardingStatus(ctx, playerID)
	if err != nil {
		return fmt.Errorf("get onboarding status: %w", err)
	}
	canAdvance, err := domain.CanTransitionOnboardingStatus(current, target)
	if err != nil {
		return fmt.Errorf("check onboarding transition %q -> %q: %w", current, target, err)
	}
	if !canAdvance || current == target {
		return nil
	}
	if err := s.playerRepo.UpdateOnboardingStatus(ctx, playerID, target); err != nil {
		return fmt.Errorf("update onboarding status: %w", err)
	}
	return nil
}

// IsPublisherBug は subscriber が publisher 起源の不整合を検出するためのヘルパー。
// 再処理しても解決しないため、subscriber は ERROR ログ + NACK する責務を負う。
func IsPublisherBug(err error) bool {
	return errors.Is(err, port.ErrNotFound) || errors.Is(err, ErrFactionConflict)
}
