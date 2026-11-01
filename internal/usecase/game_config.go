package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

const (
	gameConfigKeyFreeDailyBattleLimit  = "free_daily_battle_limit"
	gameConfigKeyExpFormulaCoefficient = "exp_formula_coefficient"
	gameConfigKeyExpWin                = "exp_win"
	gameConfigKeyExpLoss               = "exp_loss"
	gameConfigKeyExpDraw               = "exp_draw"
)

const (
	minFreeDailyBattleLimit       = 1
	minNonNegativeGameConfigValue = 0
)

// ValidateGameConfig は、必要なゲーム設定がすべて存在し、値が有効かを検証する。
// 無料プレイヤーの1日の上限と経験値係数は 1 以上、勝利・敗北・引き分けの経験値は 0 以上でなければならない。
// 最小値を下回る設定は ErrGameConfigBelowMinimum を返す。
func ValidateGameConfig(ctx context.Context, repo port.GameConfigRepo) error {
	if err := validateGameConfigKeys(ctx, repo, []string{gameConfigKeyFreeDailyBattleLimit}, minFreeDailyBattleLimit); err != nil {
		return err
	}
	if err := validateGameConfigKeys(ctx, repo, []string{gameConfigKeyExpFormulaCoefficient}, domain.MinExpCoefficient); err != nil {
		return err
	}
	nonNegativeKeys := []string{
		gameConfigKeyExpWin,
		gameConfigKeyExpLoss,
		gameConfigKeyExpDraw,
	}
	return validateGameConfigKeys(ctx, repo, nonNegativeKeys, minNonNegativeGameConfigValue)
}

func validateGameConfigKeys(ctx context.Context, repo port.GameConfigRepo, keys []string, minValue int64) error {
	for _, key := range keys {
		value, err := repo.GetInt64(ctx, key)
		if err != nil {
			return fmt.Errorf("validate game config %q: %w", key, err)
		}
		if value < minValue {
			return fmt.Errorf("%w: game config %q must be >= %d, got %d", ErrGameConfigBelowMinimum, key, minValue, value)
		}
	}
	return nil
}

// readGameConfig はゲーム設定の整数値を返す。設定が存在しないときは ErrGameConfigNotFound を返す。
func readGameConfig(ctx context.Context, repo port.GameConfigRepo, key string) (int64, error) {
	value, err := repo.GetInt64(ctx, key)
	if errors.Is(err, port.ErrNotFound) {
		// プレイヤーが存在しないエラーと区別するため、port.ErrNotFound を引き継がない。
		return 0, fmt.Errorf("%w: %q", ErrGameConfigNotFound, key)
	}
	if err != nil {
		return 0, fmt.Errorf("read game config %q: %w", key, err)
	}
	return value, nil
}
