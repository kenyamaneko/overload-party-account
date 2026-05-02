package domain

import "errors"

// OnboardingStatus はオンボード進行ステートマシンの 4 値。
// account.players.onboarding_status カラムの取りうる値の SSoT。
const (
	OnboardingStatusNotStarted = "not_started"
	OnboardingStatusNameSet    = "name_set"
	OnboardingStatusFactionSet = "faction_set"
	OnboardingStatusCompleted  = "completed"
)

// onboardingStatusOrder はステートマシンの一方向遷移を保証するための
// 順序マップ。subscriber が再配信や逆方向遷移を構造的に弾くために参照する。
var onboardingStatusOrder = map[string]int{
	OnboardingStatusNotStarted: 0,
	OnboardingStatusNameSet:    1,
	OnboardingStatusFactionSet: 2,
	OnboardingStatusCompleted:  3,
}

// ErrUnknownOnboardingStatus は OnboardingStatus が定義済み 4 値以外であることを示す。
var ErrUnknownOnboardingStatus = errors.New("unknown onboarding status")

// CanTransitionOnboardingStatus は current から next への遷移が一方向順序として
// 許容されるかを返す。current == next も許容する (subscriber の冪等性のため)。
// 未知の値は ErrUnknownOnboardingStatus を返し、サイレントな黙認をしない。
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
