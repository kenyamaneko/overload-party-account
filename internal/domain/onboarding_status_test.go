package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

func TestCanTransitionOnboardingStatus(t *testing.T) {
	t.Run("オンボーディング状態の前進判定", func(t *testing.T) {
		t.Run("現在の状態が4値のいずれでもないとき、エラーを返す", func(t *testing.T) {
			_, err := domain.CanTransitionOnboardingStatus("bogus", domain.OnboardingStatusNameSet)

			require.Error(t, err)
		})

		t.Run("遷移先の状態が4値のいずれでもないとき、エラーを返す", func(t *testing.T) {
			_, err := domain.CanTransitionOnboardingStatus(domain.OnboardingStatusNameSet, "bogus")

			require.Error(t, err)
		})

		t.Run("現在の状態と遷移先の状態が同じとき、遷移は許可される", func(t *testing.T) {
			ok, err := domain.CanTransitionOnboardingStatus(domain.OnboardingStatusNameSet, domain.OnboardingStatusNameSet)

			require.NoError(t, err)
			assert.True(t, ok)
		})

		t.Run("遷移先の状態が現在の状態より前進しているとき、遷移は許可される", func(t *testing.T) {
			ok, err := domain.CanTransitionOnboardingStatus(domain.OnboardingStatusNameSet, domain.OnboardingStatusFactionSet)

			require.NoError(t, err)
			assert.True(t, ok)
		})

		t.Run("遷移先の状態が現在の状態より後退しているとき、遷移は許可されない", func(t *testing.T) {
			ok, err := domain.CanTransitionOnboardingStatus(domain.OnboardingStatusFactionSet, domain.OnboardingStatusNameSet)

			require.NoError(t, err)
			assert.False(t, ok)
		})
	})
}
