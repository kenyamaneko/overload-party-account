package usecase

import (
	"context"
	"fmt"

	gamedesign "github.com/kenyamaneko/overload-party-common/packages/game-design-constants"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// FactionInteractor は REST 経路の初期ファクション選択を管理する。
// Pub/Sub 起点のオンボード faction-set 処理は OnboardingInteractor.ApplyFactionSet が担う。
type FactionInteractor struct {
	playerRepo  port.PlayerRepo
	factionRepo port.FactionRepo
	txRunner    port.TxRunner
}

// NewFactionInteractor は FactionInteractor を生成する。
func NewFactionInteractor(
	playerRepo port.PlayerRepo,
	factionRepo port.FactionRepo,
	txRunner port.TxRunner,
) *FactionInteractor {
	return &FactionInteractor{
		playerRepo:  playerRepo,
		factionRepo: factionRepo,
		txRunner:    txRunner,
	}
}

// SelectInitialFaction はオンボーディングで選択した faction を確定する。
func (s *FactionInteractor) SelectInitialFaction(ctx context.Context, playerID, faction string) error {
	if playerID == "" {
		return fmt.Errorf("%w: playerID is empty", ErrInvalidFaction)
	}
	if err := validateInitialFaction(faction); err != nil {
		return err
	}

	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		exists, err := s.playerRepo.Exists(txCtx, playerID)
		if err != nil {
			return fmt.Errorf("check player exists: %w", err)
		}
		if !exists {
			return fmt.Errorf("player %s: %w", playerID, port.ErrNotFound)
		}
		existing, err := s.factionRepo.GetInitialFaction(txCtx, playerID)
		if err != nil {
			return fmt.Errorf("get initial faction: %w", err)
		}
		if existing != nil {
			return ErrFactionAlreadySelected
		}
		if err := s.factionRepo.SetInitialFaction(txCtx, playerID, faction); err != nil {
			return fmt.Errorf("set initial faction: %w", err)
		}
		return nil
	})
}

// 許可集合の SSoT は overload-party-common/data/factions.yaml で、
// is_collectible=true のものが SelectableFactions に自動導出される。
// account 側で独自に許可リストを持たない。
func validateInitialFaction(faction string) error {
	for _, f := range gamedesign.SelectableFactions {
		if f == faction {
			return nil
		}
	}
	return fmt.Errorf("%w: %q is not selectable (expected one of %v)",
		ErrInvalidFaction, faction, gamedesign.SelectableFactions)
}
