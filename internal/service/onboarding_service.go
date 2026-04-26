package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/kenyamaneko/overload-party-account/internal/model"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// OnboardingService はオンボード進行イベント (onboarding-name-set /
// onboarding-faction-set / player-onboarded) を受けたときの account 内副作用を
// 単一トランザクションで反映するユースケース層。
//
// subscriber 層は Unmarshal と本サービスの Apply* 呼び出しのみを担い、
// 冪等性ガード (processed_events) と業務データの永続化を同一 Tx で束ねる責務は
// 本サービスが持つ。
type OnboardingService struct {
	playerRepo  port.PlayerRepo
	factionRepo port.FactionRepo
	eventRepo   port.ProcessedEventRepo
	txRunner    port.TxRunner
}

// NewOnboardingService は OnboardingService を生成する。
func NewOnboardingService(
	playerRepo port.PlayerRepo,
	factionRepo port.FactionRepo,
	eventRepo port.ProcessedEventRepo,
	txRunner port.TxRunner,
) *OnboardingService {
	return &OnboardingService{
		playerRepo:  playerRepo,
		factionRepo: factionRepo,
		eventRepo:   eventRepo,
		txRunner:    txRunner,
	}
}

// ApplyNameSet は onboarding-name-set イベントの副作用を反映する。
// 単一 Tx 内で processed_events 挿入 + name + onboarding_status 更新を実行する。
//
// processed が false のとき、event_id が既に処理済み (重複配信)。呼び出し側は
// ACK のみで他の副作用は走らない。
func (s *OnboardingService) ApplyNameSet(
	ctx context.Context,
	eventID, eventType, playerID, name string,
) (processed bool, err error) {
	if err := model.ValidateName(name); err != nil {
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
		return s.playerRepo.ApplyOnboardingNameSet(txCtx, playerID, name)
	}); txErr != nil {
		return false, txErr
	}
	return processed, nil
}

// ApplyFactionSet は onboarding-faction-set イベントの副作用を反映する。
// 単一 Tx 内で processed_events 挿入 + selected_faction + onboarding_status 更新 +
// player_factions への initial_selection 行 INSERT を実行する。
func (s *OnboardingService) ApplyFactionSet(
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

		if err := s.playerRepo.ApplyOnboardingFactionSet(txCtx, playerID, initialFactionID); err != nil {
			return err
		}
		return s.factionRepo.AddPlayerFaction(txCtx, playerID, initialFactionID, FactionSourceInitialSelection)
	}); txErr != nil {
		return false, txErr
	}
	return processed, nil
}

// ApplyCompleted は player-onboarded イベントの副作用を反映する。
// 本 ADR 改訂後は status='completed' への遷移のみが responsibility (selected_faction /
// player_factions の永続化は ApplyFactionSet が先行して実行している前提)。
func (s *OnboardingService) ApplyCompleted(
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
		return s.playerRepo.ApplyOnboardingCompleted(txCtx, playerID)
	}); txErr != nil {
		return false, txErr
	}
	return processed, nil
}

// IsPublisherBug は subscriber が publisher 起源の不整合 (Register 未実施プレイヤーへの
// onboarded 配信など) を検出するためのヘルパー。port.ErrNotFound はリトライしても解決
// しないため、subscriber は本判定で err をログに ERROR 記録した上で NACK する責務を負う。
func IsPublisherBug(err error) bool {
	return errors.Is(err, port.ErrNotFound)
}
