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
// 単一 Tx 内で processed_events 挿入 + name 更新 + state machine 前進判定して
// onboarding_status を 'name_set' に進める。後進方向は publisher の out-of-order
// 配信として黙ってスキップする。
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
		if err := s.playerRepo.UpdateName(txCtx, playerID, name); err != nil {
			return err
		}
		return s.advanceOnboardingStatus(txCtx, playerID, model.OnboardingStatusNameSet)
	}); txErr != nil {
		return false, txErr
	}
	return processed, nil
}

// ApplyFactionSet は onboarding-faction-set イベントの副作用を反映する。
// 単一 Tx 内で processed_events 挿入 + initial faction 確定 +
// state machine 前進判定で onboarding_status を 'faction_set' に進める。
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

		// 「既に initial 確定済みか」のドメイン判定は service の責務。
		// 同 faction の再送は冪等扱い (UPSERT が is_initial=TRUE を維持)、
		// 別 faction が確定済みなら publisher のバグとして無視する (state machine が
		// faction_set 以降に進んでいる前提で、次のイベントは completed のみ)。
		existing, err := s.factionRepo.GetInitialFaction(txCtx, playerID)
		if err != nil {
			return fmt.Errorf("get initial faction: %w", err)
		}
		if existing != nil && *existing != initialFactionID {
			return nil
		}
		if existing == nil {
			if err := s.factionRepo.SetInitialFaction(txCtx, playerID, initialFactionID); err != nil {
				return fmt.Errorf("set initial faction: %w", err)
			}
		}
		return s.advanceOnboardingStatus(txCtx, playerID, model.OnboardingStatusFactionSet)
	}); txErr != nil {
		return false, txErr
	}
	return processed, nil
}

// ApplyCompleted は player-onboarded イベントの副作用を反映する。
// 本 ADR 改訂後は status='completed' への遷移のみが responsibility (initial faction の
// 永続化は ApplyFactionSet が先行して実行している前提)。completed は terminal なので
// 一方向順序チェックは不要だが、共通の advanceOnboardingStatus 経由で進めることで
// 「未知の現状態に遭遇したらエラー」のセマンティクスを共有する。
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
		return s.advanceOnboardingStatus(txCtx, playerID, model.OnboardingStatusCompleted)
	}); txErr != nil {
		return false, txErr
	}
	return processed, nil
}

// advanceOnboardingStatus は state machine の一方向遷移を担保しつつ
// onboarding_status を target に進める。後進方向は黙ってスキップする
// (out-of-order 配信に対する防御)。
func (s *OnboardingService) advanceOnboardingStatus(ctx context.Context, playerID, target string) error {
	current, err := s.playerRepo.GetOnboardingStatus(ctx, playerID)
	if err != nil {
		return fmt.Errorf("get onboarding status: %w", err)
	}
	canAdvance, err := model.CanTransitionOnboardingStatus(current, target)
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

// IsPublisherBug は subscriber が publisher 起源の不整合 (Register 未実施プレイヤーへの
// onboarded 配信など) を検出するためのヘルパー。port.ErrNotFound はリトライしても解決
// しないため、subscriber は本判定で err をログに ERROR 記録した上で NACK する責務を負う。
func IsPublisherBug(err error) bool {
	return errors.Is(err, port.ErrNotFound)
}
