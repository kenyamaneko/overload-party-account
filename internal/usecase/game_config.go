package usecase

import (
	"context"
	"fmt"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// gameConfigKeyExpWin / KeyExpLoss / KeyExpDraw は AwardGameExp の付与ルールで参照する経験値量。
// 0 (= 経験値なし) も業務上有効な値として許す。
const (
	gameConfigKeyExpWin  = "exp_win"
	gameConfigKeyExpLoss = "exp_loss"
	gameConfigKeyExpDraw = "exp_draw"
)

// ValidateGameConfig は account 起動時に必須 game_config キーが妥当な値で投入済みかを確認する。
// usecase 層は本検証通過後の値を信頼し、リクエスト経路では値の妥当性を再検証しない。
// game_config はプレイヤー非依存・低頻度更新の運用設定であり、不正値が入っているなら
// リクエスト到来前に fail-fast させる方が運用上望ましいため、この境界を起動時に置く。
func ValidateGameConfig(ctx context.Context, repo port.GameConfigRepo) error {
	// 正の整数を要求するキー: 0 / 負値は usecase の前提を破る
	// (上限 0 = バトル不可、係数 0 = レベル算出が無限ループ等)。
	positiveKeys := []string{
		configKeyFreeDailyBattleLimit,
		ConfigKeyExpFormulaCoefficient,
	}
	for _, key := range positiveKeys {
		v, err := repo.GetInt64(ctx, key)
		if err != nil {
			return fmt.Errorf("validate game config %q: %w", key, err)
		}
		if v <= 0 {
			return fmt.Errorf("game config %q must be > 0, got %d", key, v)
		}
	}

	// 値 0 を許すが、ドキュメントが存在することは必須なキー。
	requiredKeys := []string{
		gameConfigKeyExpWin,
		gameConfigKeyExpLoss,
		gameConfigKeyExpDraw,
	}
	for _, key := range requiredKeys {
		if _, err := repo.GetInt64(ctx, key); err != nil {
			return fmt.Errorf("validate game config %q: %w", key, err)
		}
	}
	return nil
}
