package usecase

import (
	"context"
	"fmt"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// assemblePlayerInfo は、ゲーム設定の経験値係数を読んで、参照用の情報をプレイヤー情報にする。
// ゲーム設定が存在しないときは ErrGameConfigNotFound を返す。
func assemblePlayerInfo(ctx context.Context, gameConfigRepo port.GameConfigRepo, view *domain.PlayerView) (*domain.PlayerInfo, error) {
	coefficient, err := readGameConfig(ctx, gameConfigRepo, gameConfigKeyExpFormulaCoefficient)
	if err != nil {
		return nil, err
	}
	return buildPlayerInfo(view, coefficient)
}

// buildPlayerInfo は、参照用の情報の累計経験値と経験値係数からレベルを求め、レベル内の経験値の進み具合を加えてプレイヤー情報にする。
func buildPlayerInfo(view *domain.PlayerView, coefficient int64) (*domain.PlayerInfo, error) {
	level, err := domain.ComputeLevel(view.Exp, coefficient)
	if err != nil {
		return nil, fmt.Errorf("compute level: %w", err)
	}
	levelProgress, err := domain.ComputeExpProgress(view.Exp, coefficient)
	if err != nil {
		return nil, fmt.Errorf("compute exp progress: %w", err)
	}
	return &domain.PlayerInfo{View: *view, Level: level, LevelProgress: *levelProgress}, nil
}
