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

// FactionService は初期ファクション選択フローを管理します。
// カード配布は card サービスの Pub/Sub subscriber が独立して処理します。
type FactionService struct {
	playerRepo  port.PlayerRepo
	factionRepo port.FactionRepo
	eventRepo   port.ProcessedEventRepo
	txRunner    port.TxRunner
}

// NewFactionService は FactionService を生成します。
// eventRepo は Pub/Sub 起源のユースケース (ApplyOnboardingResult) で冪等ガードに
// 使用します。REST 起点のフローでは使用しません。
func NewFactionService(
	playerRepo port.PlayerRepo,
	factionRepo port.FactionRepo,
	eventRepo port.ProcessedEventRepo,
	txRunner port.TxRunner,
) *FactionService {
	return &FactionService{
		playerRepo:  playerRepo,
		factionRepo: factionRepo,
		eventRepo:   eventRepo,
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

// ApplyOnboardingResult は player-onboarded イベント起因で表示名更新と初期ファクション
// 確定、および processed_events 冪等ガードを同一トランザクションに束ねて反映する
// ユースケースです。subscriber は Unmarshal と本メソッド呼び出しのみを担当し、
// ビジネス不変条件 (SelectableFactions 内であること / selected_faction が NULL の
// ときだけ上書きすること / processed_events・username・player_factions・
// selected_faction を同一 Tx で確定すること) は本メソッドが保証します。
//
// 戻り値:
//   - processed=false: 同一 event_id が processed_events に既存 (冪等スキップ)。
//     他の戻り値フィールドは無意味 (副作用なし)。呼び出し側は ACK するだけ。
//   - processed=true, selected=true: 今回のコールで全副作用を確定した。
//   - processed=true, selected=false: selected_faction が既に埋まっていた
//     (二重配信 / race)。username と player_factions は反映済み。ログ警告のみで
//     エラー扱いにしない (ErrFactionAlreadySelected は REST 用意味論なので
//     onboarding パスでは返さない)。
//
// プレイヤーが存在しない場合は port.ErrNotFound を返します。呼び出し側はこのエラーを
// publisher のバグ (Register 未実施プレイヤーへの onboarded 配信) として明示的に
// ハンドリングする責務を負います。
func (s *FactionService) ApplyOnboardingResult(
	ctx context.Context,
	eventID, eventType, playerID, displayName, initialFactionID string,
) (processed, selected bool, err error) {
	if playerID == "" {
		return false, false, fmt.Errorf("%w: playerID is empty", ErrInvalidFaction)
	}
	if displayName == "" {
		return false, false, fmt.Errorf("%w: displayName is empty", ErrInvalidFaction)
	}
	if err := validateInitialFaction(initialFactionID); err != nil {
		return false, false, err
	}

	if txErr := s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		inserted, err := s.eventRepo.Insert(txCtx, eventID, eventType)
		if err != nil {
			return fmt.Errorf("insert processed_events: %w", err)
		}
		if !inserted {
			// 冪等スキップ。後段の書き込みは行わず processed=false で戻す。
			return nil
		}
		processed = true

		// player が存在しない場合は port.ErrNotFound が返り、呼び出し側 (subscriber) が
		// publisher バグとして DLQ 相当の明示ハンドリングを行う。
		if err := s.playerRepo.UpdateName(txCtx, playerID, displayName); err != nil {
			return fmt.Errorf("update name: %w", err)
		}
		set, err := s.writeInitialFactionTx(txCtx, playerID, initialFactionID)
		if err != nil {
			return err
		}
		selected = set
		return nil
	}); txErr != nil {
		return false, false, txErr
	}
	return processed, selected, nil
}

// writeInitialFactionTx は「selected_faction が NULL のときだけ書き込み、成立したら
// player_factions にも initial_selection で 1 行追加する」共通ロジックを
// トランザクション内で実行します。呼び出し側は必ず TxRunner.RunInTx 配下で呼ぶこと。
// selected=false は既に選択済み (or ショップ先行で player_factions 行があっても
// selected_faction が埋まっていたケース) を意味します。
func (s *FactionService) writeInitialFactionTx(
	txCtx context.Context,
	playerID, faction string,
) (selected bool, err error) {
	set, err := s.playerRepo.TrySetInitialFaction(txCtx, playerID, faction)
	if err != nil {
		return false, fmt.Errorf("try set initial faction: %w", err)
	}
	if !set {
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
