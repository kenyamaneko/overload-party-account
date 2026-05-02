package domain

import "errors"

// OnboardingStatus は account.players.onboarding_status カラムの取りうる 4 値の SSoT。
const (
	OnboardingStatusNotStarted = "not_started"
	OnboardingStatusNameSet    = "name_set"
	OnboardingStatusFactionSet = "faction_set"
	OnboardingStatusCompleted  = "completed"
)

// onboardingStatusOrder は前進方向のみ許容する一方向遷移の判定に使う順序マップ。
var onboardingStatusOrder = map[string]int{
	OnboardingStatusNotStarted: 0,
	OnboardingStatusNameSet:    1,
	OnboardingStatusFactionSet: 2,
	OnboardingStatusCompleted:  3,
}

// ErrUnknownOnboardingStatus は OnboardingStatus が定義済み 4 値以外であることを示す。
var ErrUnknownOnboardingStatus = errors.New("unknown onboarding status")

// CanTransitionOnboardingStatus は current から next への前進遷移が許容されるかを返す。
// current == next も許容する (subscriber の冪等性のため)。
func CanTransitionOnboardingStatus(current, next string) (bool, error) {
	c, ok := onboardingStatusOrder[current]
	if !ok {
		return false, ErrUnknownOnboardingStatus
	}
	n, ok := onboardingStatusOrder[next]
	if !ok {
		return false, ErrUnknownOnboardingStatus
	}
	return c <= n, nil
}
