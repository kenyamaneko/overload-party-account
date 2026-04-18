package service

import (
	"context"
	"fmt"

	gamedesign "github.com/kenyamaneko/overload-party-common/packages/game-design-constants"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// FactionSourceInitialSelection は初期選択フローで player_factions に書き込む source 値です。
const FactionSourceInitialSelection = "initial_selection"

// FactionService は初期ファクション選択フローを管理します。
// カード配布は card サービスの Pub/Sub subscriber が独立して処理します。
type FactionService struct {
	playerRepo  port.PlayerRepo
	factionRepo port.FactionRepo
	txRunner    port.TxRunner
}

// NewFactionService は FactionService を生成します。
func NewFactionService(
	playerRepo port.PlayerRepo,
	factionRepo port.FactionRepo,
	txRunner port.TxRunner,
) *FactionService {
	return &FactionService{
		playerRepo:  playerRepo,
		factionRepo: factionRepo,
		txRunner:    txRunner,
	}
}

// SelectInitialFaction は player_factions に INSERT し selected_faction を更新します。
// player_factions の複合 PK が冪等性の SSoT であり、INSERT が空なら選択済みと判断します。
func (s *FactionService) SelectInitialFaction(ctx context.Context, playerID, faction string) error {
	if playerID == "" {
		return fmt.Errorf("%w: playerID is empty", ErrInvalidFaction)
	}
	if err := validateInitialFaction(faction); err != nil {
		return err
	}

	var created bool
	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		inserted, err := s.factionRepo.InsertInitial(txCtx, playerID, faction, FactionSourceInitialSelection)
		if err != nil {
			return fmt.Errorf("insert initial faction: %w", err)
		}
		if !inserted {
			return nil
		}
		if err := s.playerRepo.UpdateFaction(txCtx, playerID, faction); err != nil {
			return fmt.Errorf("update selected_faction: %w", err)
		}
		created = true
		return nil
	}); err != nil {
		return err
	}

	if !created {
		return ErrFactionAlreadySelected
	}

	return nil
}

// Neutral は除外する。プレイヤーは Neutral として開始せず、
// Neutral カードは grant-initial-pack で選択ファクションと同時に配布される。
func validateInitialFaction(faction string) error {
	for _, f := range gamedesign.SelectableFactions {
		if f == faction {
			return nil
		}
	}
	return fmt.Errorf("%w: %q is not selectable (expected one of %v)",
		ErrInvalidFaction, faction, gamedesign.SelectableFactions)
}
