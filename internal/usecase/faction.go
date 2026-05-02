package usecase

import (
	"context"
	"fmt"

	gamedesign "github.com/kenyamaneko/overload-party-common/packages/game-design-constants"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// FactionInteractor は REST 経路 (`POST /players/:playerId/factions/select`) からの
// 初期ファクション選択を管理します。Pub/Sub 起点のオンボード faction-set 処理は
// OnboardingInteractor.ApplyFactionSet が担います。
// カード配布は card サービスの Pub/Sub subscriber が独立して処理します。
type FactionInteractor struct {
	playerRepo  port.PlayerRepo
	factionRepo port.FactionRepo
	txRunner    port.TxRunner
}

// NewFactionInteractor は FactionInteractor を生成します。
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

// SelectInitialFaction はオンボーディングで選択した faction を確定します。
// SSoT は player_factions.is_initial=TRUE の行で、1 プレイヤーに最大 1 つ
// (partial unique index で保証)。ショップ先行で同 faction を所持していても
// is_initial=TRUE への昇格として成立します。
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
		// 「既に initial 確定済みか」のドメイン判定はここで行い、ErrFactionAlreadySelected に翻訳する。
		// repo はプリミティブ (CLAUDE.md: リポジトリ層はロジックを持たない)。
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
