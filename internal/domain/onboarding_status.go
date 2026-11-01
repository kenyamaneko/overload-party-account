package domain

import (
	"errors"
	"fmt"
	"slices"
)

// オンボーディングの進行状態。
const (
	OnboardingStatusNotStarted = "not_started"
	OnboardingStatusNameSet    = "name_set"
	OnboardingStatusFactionSet = "faction_set"
	OnboardingStatusCompleted  = "completed"
)

// adjacentOnboardingStageDistance は、隣り合う進行状態の間の段階の差である。
const adjacentOnboardingStageDistance = 1

// onboardingStatusSequence は進行状態を進む順に並べたものである。
var onboardingStatusSequence = []string{
	OnboardingStatusNotStarted,
	OnboardingStatusNameSet,
	OnboardingStatusFactionSet,
	OnboardingStatusCompleted,
}

var (
	// ErrUnknownOnboardingStatus は進行状態が定義された値のいずれでもないことを示す。
	ErrUnknownOnboardingStatus = errors.New("unknown onboarding status")
	// ErrOnboardingStatusSkipped は進行状態が段階を飛ばして進もうとしていることを示す。
	ErrOnboardingStatusSkipped = errors.New("onboarding status skipped a stage")
)

// CanTransitionOnboardingStatus は進行状態を current から next へ変えてよいかを返す。
// 隣の段階への前進と同じ状態への変更は true、前の段階への後退は false を返す。
// 段階を飛ばす前進は ErrOnboardingStatusSkipped を返す。
func CanTransitionOnboardingStatus(current, next string) (bool, error) {
	currentStage := slices.Index(onboardingStatusSequence, current)
	if currentStage < 0 {
		return false, fmt.Errorf("%w: %q", ErrUnknownOnboardingStatus, current)
	}
	nextStage := slices.Index(onboardingStatusSequence, next)
	if nextStage < 0 {
		return false, fmt.Errorf("%w: %q", ErrUnknownOnboardingStatus, next)
	}
	switch {
	case nextStage < currentStage:
		return false, nil
	case nextStage > currentStage+adjacentOnboardingStageDistance:
		return false, fmt.Errorf("%w: %s -> %s", ErrOnboardingStatusSkipped, current, next)
	default:
		return true, nil
	}
}
