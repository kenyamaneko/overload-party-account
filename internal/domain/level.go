package domain

import (
	"errors"
	"fmt"
)

const (
	// InitialLevel は登録直後のプレイヤーのレベルであり、取りうる最小のレベルである。
	InitialLevel = 1
	// InitialExp は登録直後のプレイヤーの累計経験値である。
	InitialExp = 0
	// MinExpCoefficient は経験値係数の最小値である。
	MinExpCoefficient = 1
)

var (
	// ErrInvalidExpCoefficient は経験値係数が最小値未満であることを示す。
	ErrInvalidExpCoefficient = errors.New("exp coefficient is below the minimum")
	// ErrExpBelowLevelThreshold は累計経験値が最小のレベルの開始値を下回っている (負である) ことを示す。
	ErrExpBelowLevelThreshold = errors.New("exp is below the start of the minimum level")
)

// LevelProgress は現在のレベルの中での経験値の進み具合を表す。
type LevelProgress struct {
	LevelExpCurrent  int64
	LevelExpRequired int64
}

// ComputeLevel は累計経験値と経験値係数から、プレイヤーのレベルを返す。
func ComputeLevel(exp, coefficient int64) (int64, error) {
	if err := validateExpCoefficient(coefficient); err != nil {
		return 0, err
	}
	if minLevelStartExp := levelStartExp(InitialLevel, coefficient); exp < minLevelStartExp {
		return 0, fmt.Errorf("%w: exp %d, level %d starts at %d",
			ErrExpBelowLevelThreshold, exp, InitialLevel, minLevelStartExp)
	}
	level := int64(InitialLevel)
	for exp >= levelStartExp(level+1, coefficient) {
		level++
	}
	return level, nil
}

// ComputeExpProgress は累計経験値と経験値係数から、レベルの中での経験値と、次のレベルまでに必要な経験値を返す。
func ComputeExpProgress(exp, coefficient int64) (*LevelProgress, error) {
	level, err := ComputeLevel(exp, coefficient)
	if err != nil {
		return nil, err
	}
	currentLevelStartExp := levelStartExp(level, coefficient)
	nextLevelStartExp := levelStartExp(level+1, coefficient)
	return &LevelProgress{
		LevelExpCurrent:  exp - currentLevelStartExp,
		LevelExpRequired: nextLevelStartExp - currentLevelStartExp,
	}, nil
}

func validateExpCoefficient(coefficient int64) error {
	if coefficient < MinExpCoefficient {
		return fmt.Errorf("%w: got %d", ErrInvalidExpCoefficient, coefficient)
	}
	return nil
}

// levelStartExp はレベルに到達するために必要な累計経験値を返す。最小のレベルは経験値 0 から始まる。
func levelStartExp(level, coefficient int64) int64 {
	if level <= InitialLevel {
		return 0
	}
	return coefficient * level * level
}
