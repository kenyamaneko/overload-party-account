package domain

import "fmt"

// LevelProgress は現在レベル内の経験値進捗を保持する。
type LevelProgress struct {
	LevelExpCurrent  int64 `json:"level_exp_current"`
	LevelExpRequired int64 `json:"level_exp_required"`
}

// ComputeLevel は経験値獲得後の新レベルを算出する。
// exp は累計値を SSoT とする設計のためレベルアップ時に減算せず、
// レベル内進捗は ComputeExpProgress で都度導出する。よって本関数は新レベルのみを返す。
// coeff * (level+1)^2 を逐次比較するため、ゲームデザイン上の妥当な
// coeff (~数百) と level (~数百) の範囲では int64 内で安全に動作する。
// 過大 coeff の検証は config 検証側の責務 (現状は > 0 のみ)。
// currentLevel は 1 以上を要求する。
func ComputeLevel(newExp, currentLevel, coeff int64) (int64, error) {
	if currentLevel < 1 {
		return 0, fmt.Errorf("currentLevel must be >= 1, got %d", currentLevel)
	}
	level := currentLevel
	for {
		nextLevelExp := coeff * (level + 1) * (level + 1)
		if newExp < nextLevelExp {
			break
		}
		level++
	}
	return level, nil
}

// ComputeExpProgress は現在レベル内の経験値進捗を計算する。
// level/exp は ComputeLevel で整合済みの累計 exp を前提とするため、
// 不整合な入力 (例: level=3, exp=0) はバグ検知としてエラーを返す。
// level=1 の開始閾値は 0 (プレイヤー初期状態)、level>=2 では coeff * level^2 が
// 開始閾値となる (ComputeLevel の境界式 coeff * (level+1)^2 と整合)。
func ComputeExpProgress(level, exp, coeff int64) (*LevelProgress, error) {
	if level < 1 {
		return nil, fmt.Errorf("level must be >= 1, got %d", level)
	}
	var currentLevelExp int64
	if level > 1 {
		currentLevelExp = coeff * level * level
	}
	if exp < currentLevelExp {
		return nil, fmt.Errorf("exp (%d) is below current level threshold (%d) for level %d", exp, currentLevelExp, level)
	}
	nextLevelExp := coeff * (level + 1) * (level + 1)
	return &LevelProgress{
		LevelExpCurrent:  exp - currentLevelExp,
		LevelExpRequired: nextLevelExp - currentLevelExp,
	}, nil
}
