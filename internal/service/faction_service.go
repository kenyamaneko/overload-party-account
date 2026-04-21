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

// SelectInitialFaction は初回選択を確定し、所持ファクションにも追加します。
// 「初回選択済みか否か」の SSoT は players.selected_faction であり、NULL の間だけ成立します。
// ショップ先行で player_factions に行があっても selected_faction が NULL なら初回選択できます。
func (s *FactionService) SelectInitialFaction(ctx context.Context, playerID, faction string) error {
	if playerID == "" {
		return fmt.Errorf("%w: playerID is empty", ErrInvalidFaction)
	}
	if err := validateInitialFaction(faction); err != nil {
		return err
	}

	var selected bool
	if err := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		set, err := s.playerRepo.TrySetInitialFaction(txCtx, playerID, faction)
		if err != nil {
			return fmt.Errorf("try set initial faction: %w", err)
		}
		if !set {
			return nil
		}
		if err := s.factionRepo.AddPlayerFaction(txCtx, playerID, faction, FactionSourceInitialSelection); err != nil {
			return fmt.Errorf("add player faction: %w", err)
		}
		selected = true
		return nil
	}); err != nil {
		return err
	}

	if !selected {
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
