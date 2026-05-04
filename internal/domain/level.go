package domain

// LevelProgress は現在レベル内の経験値進捗を保持する。
type LevelProgress struct {
	LevelExpCurrent  int64 `json:"level_exp_current"`
	LevelExpRequired int64 `json:"level_exp_required"`
}

// ComputeLevel は経験値獲得後の新レベルを算出する。
// coeff * (level+1)^2 を逐次比較するため、ゲームデザイン上の妥当な
// coeff (~数百) と level (~数百) の範囲では int64 内で安全に動作する。
// 過大 coeff の検証は config 検証側の責務 (現状は > 0 のみ)。
func ComputeLevel(newExp, currentLevel, coeff int64) int64 {
	level := currentLevel
	if level < 1 {
		level = 1
	}
	for {
		nextLevelExp := coeff * (level + 1) * (level + 1)
		if newExp < nextLevelExp {
			break
		}
		level++
	}
	return level
}

// ComputeLevelProgress は現在レベル内の経験値進捗を計算する。
func ComputeLevelProgress(level, exp, coeff int64) *LevelProgress {
	currentLevelExp := coeff * level * level
	nextLevelExp := coeff * (level + 1) * (level + 1)
	return &LevelProgress{
		LevelExpCurrent:  max(0, exp-currentLevelExp),
		LevelExpRequired: nextLevelExp - currentLevelExp,
	}
}
