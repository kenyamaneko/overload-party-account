package usecase

import (
	"context"
	"errors"
	"fmt"
	"slices"

	gamedesign "github.com/kenyamaneko/overload-party-common/packages/game-design-constants"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// FactionInteractor はプレイヤーの所持陣営と初期陣営を扱う。
type FactionInteractor struct {
	playerRepo  port.PlayerRepo
	factionRepo port.FactionRepo
	txRunner    port.TxRunner
	eventGuard  *processedEventGuard
}

// NewFactionInteractor は FactionInteractor を生成する。
func NewFactionInteractor(
	playerRepo port.PlayerRepo,
	factionRepo port.FactionRepo,
	eventRepo port.ProcessedEventRepo,
	txRunner port.TxRunner,
) *FactionInteractor {
	return &FactionInteractor{
		playerRepo:  playerRepo,
		factionRepo: factionRepo,
		txRunner:    txRunner,
		eventGuard:  &processedEventGuard{txRunner: txRunner, eventRepo: eventRepo},
	}
}

// GrantFaction は選択できる陣営を所持陣営に加える。
// 選択できない陣営は ErrInvalidFaction、プレイヤーが登録されていなければ port.ErrNotFound を包んだエラーを返す。
func (fi *FactionInteractor) GrantFaction(ctx context.Context, playerID, faction string) error {
	if err := validateSelectableFaction(faction); err != nil {
		return err
	}
	if err := ensurePlayerExists(ctx, fi.playerRepo, playerID); err != nil {
		return err
	}
	return fi.factionRepo.AddPlayerFaction(ctx, playerID, faction)
}

// ListFactions は所持陣営の一覧を返す。プレイヤーが登録されていなければ port.ErrNotFound を包んだエラーを返す。
func (fi *FactionInteractor) ListFactions(ctx context.Context, playerID string) ([]string, error) {
	if err := ensurePlayerExists(ctx, fi.playerRepo, playerID); err != nil {
		return nil, err
	}
	return fi.factionRepo.GetPlayerFactions(ctx, playerID)
}

// SelectInitialFaction は選択した陣営を初期陣営にする。
// 選択できない陣営は ErrInvalidFaction、初期陣営を選択済みのときは ErrFactionAlreadySelected、
// プレイヤーが登録されていなければ port.ErrNotFound を包んだエラーを返す。
func (fi *FactionInteractor) SelectInitialFaction(ctx context.Context, playerID, faction string) error {
	if err := validateSelectableFaction(faction); err != nil {
		return err
	}

	return fi.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		if err := ensurePlayerExists(txCtx, fi.playerRepo, playerID); err != nil {
			return err
		}
		existing, err := confirmInitialFaction(txCtx, fi.factionRepo, playerID, faction)
		if errors.Is(err, port.ErrAlreadyExists) {
			return ErrFactionAlreadySelected
		}
		if err != nil {
			return err
		}
		if existing != nil {
			return ErrFactionAlreadySelected
		}
		return nil
	})
}

// ApplyFactionAcquired は陣営獲得イベントを反映して、陣営を所持陣営に加える。
// 同じイベントIDのイベントを反映済みのときは何もしない。
// 所持陣営として存在しない陣営は ErrUnknownFaction を返し、イベントIDも記録しない。
// プレイヤーが登録されていなければ port.ErrNotFound を包んだエラーを返す。
func (fi *FactionInteractor) ApplyFactionAcquired(ctx context.Context, eventID, eventType, playerID, faction string) error {
	if err := validateKnownFaction(faction); err != nil {
		return err
	}
	return fi.eventGuard.applyOnce(ctx, eventID, eventType, func(txCtx context.Context) error {
		if err := fi.factionRepo.AddPlayerFaction(txCtx, playerID, faction); err != nil {
			return fmt.Errorf("add player_faction: %w", err)
		}
		return nil
	})
}

// confirmInitialFaction は、初期陣営が未設定のときだけ faction を初期陣営に設定する。
// すでに設定済みの初期陣営があるときは、設定を変えずにその陣営を返す。設定したときは nil を返す。
// 同時に別の陣営が初期陣営に設定されたときは、port.ErrAlreadyExists を包んだエラーを返す。
func confirmInitialFaction(ctx context.Context, factionRepo port.FactionRepo, playerID, faction string) (existing *string, err error) {
	existing, err = factionRepo.GetInitialFaction(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("get initial faction: %w", err)
	}
	if existing != nil {
		return existing, nil
	}
	if err := factionRepo.SetInitialFaction(ctx, playerID, faction); err != nil {
		return nil, fmt.Errorf("set initial faction: %w", err)
	}
	return nil, nil
}

// validateKnownFaction は陣営が所持陣営として存在する陣営かを検証する。存在しない陣営は ErrUnknownFaction を返す。
func validateKnownFaction(faction string) error {
	isKnown := slices.ContainsFunc(gamedesign.FactionsMetadata, func(metadata gamedesign.FactionMetadata) bool {
		return metadata.ID == faction
	})
	if !isKnown {
		return fmt.Errorf("%w: %q", ErrUnknownFaction, faction)
	}
	return nil
}

// validateSelectableFaction は陣営が選択できる陣営かを検証する。選択できない陣営は ErrInvalidFaction を返す。
func validateSelectableFaction(faction string) error {
	if slices.Contains(gamedesign.SelectableFactions, faction) {
		return nil
	}
	return fmt.Errorf("%w: %q is not selectable (expected one of %v)",
		ErrInvalidFaction, faction, gamedesign.SelectableFactions)
}
