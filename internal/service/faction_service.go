package service

import (
	"context"
	"fmt"

	gamedesign "github.com/kenyamaneko/overload-party-common/packages/game-design-constants"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// FactionSourceInitialSelection は初期選択フローで player_factions に書き込む source 値です。
const FactionSourceInitialSelection = "initial_selection"

// FactionSourceShopPurchase はショップ購入フローで player_factions に書き込む source 値です。
// REST handler / Pub/Sub subscriber / テストでリテラル "shop_purchase" を直書きせず
// 本定数を参照すること (CLAUDE.md "API 契約")。
const FactionSourceShopPurchase = "shop_purchase"

// FactionService は REST 経路 (`POST /players/:playerId/factions/select`) からの
// 初期ファクション選択を管理します。Pub/Sub 起点のオンボード faction-set 処理は
// OnboardingService.ApplyFactionSet が担います。
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
		set, err := s.writeInitialFactionTx(txCtx, playerID, faction)
		if err != nil {
			return err
		}
		selected = set
		return nil
	}); err != nil {
		return err
	}

	if !selected {
		return ErrFactionAlreadySelected
	}

	return nil
}

// writeInitialFactionTx は「selected_faction が NULL のときだけ書き込み、成立したら
// player_factions にも initial_selection で 1 行追加する」共通ロジックを
// トランザクション内で実行します。呼び出し側は必ず TxRunner.RunInTx 配下で呼ぶこと。
//
// 戻り値:
//   - selected=true: 今回のコールで初回選択が成立し player_factions にも反映済み
//   - selected=false, err=nil: 既に選択済み (ショップ先行で player_factions 行があっても
//     selected_faction が埋まっていたケースを含む)
//   - err=ErrNotFound: プレイヤー自体が存在しない
//
// 「未更新の理由が既選択かプレイヤー不在か」のドメイン判断は service 層の責務として
// ここで行う。repo は SetSelectedFactionIfNull / Exists の純プリミティブのみ提供する。
func (s *FactionService) writeInitialFactionTx(
	txCtx context.Context,
	playerID, faction string,
) (selected bool, err error) {
	set, err := s.playerRepo.SetSelectedFactionIfNull(txCtx, playerID, faction)
	if err != nil {
		return false, fmt.Errorf("set selected faction: %w", err)
	}
	if !set {
		exists, err := s.playerRepo.Exists(txCtx, playerID)
		if err != nil {
			return false, fmt.Errorf("check player exists: %w", err)
		}
		if !exists {
			return false, fmt.Errorf("player %s: %w", playerID, port.ErrNotFound)
		}
		return false, nil
	}
	// AddPlayerFaction は (player_id, faction) 複合 PK に対する
	// ON CONFLICT DO NOTHING で冪等。ショップ先行で同 faction を所持していても
	// 衝突しない。
	if err := s.factionRepo.AddPlayerFaction(txCtx, playerID, faction, FactionSourceInitialSelection); err != nil {
		return false, fmt.Errorf("add player faction: %w", err)
	}
	return true, nil
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
