package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

func TestCanTransitionOnboardingStatus(t *testing.T) {
	t.Run("オンボード状態の遷移規則", func(t *testing.T) {
		tests := []struct {
			name    string
			current string
			next    string
			want    bool
			wantErr error
		}{
			{
				name:    "not_startedからname_setへは進める",
				current: domain.OnboardingStatusNotStarted,
				next:    domain.OnboardingStatusNameSet,
				want:    true,
			},
			{
				name:    "name_setからfaction_setへは進める",
				current: domain.OnboardingStatusNameSet,
				next:    domain.OnboardingStatusFactionSet,
				want:    true,
			},
			{
				name:    "faction_setからcompletedへは進める",
				current: domain.OnboardingStatusFactionSet,
				next:    domain.OnboardingStatusCompleted,
				want:    true,
			},
			{
				name:    "not_startedからcompletedへ段飛ばしでも進める",
				current: domain.OnboardingStatusNotStarted,
				next:    domain.OnboardingStatusCompleted,
				want:    true,
			},
			{
				name:    "completedからcompletedの同値は許容される",
				current: domain.OnboardingStatusCompleted,
				next:    domain.OnboardingStatusCompleted,
				want:    true,
			},
			{
				name:    "name_setからnot_startedへは戻れない",
				current: domain.OnboardingStatusNameSet,
				next:    domain.OnboardingStatusNotStarted,
				want:    false,
			},
			{
				name:    "completedからfaction_setへは戻れない",
				current: domain.OnboardingStatusCompleted,
				next:    domain.OnboardingStatusFactionSet,
				want:    false,
			},
			{
				name:    "現在値が未知のとき、ErrUnknownOnboardingStatusになる",
				current: "TST-unknown",
				next:    domain.OnboardingStatusNameSet,
				wantErr: domain.ErrUnknownOnboardingStatus,
			},
			{
				name:    "遷移先が未知のとき、ErrUnknownOnboardingStatusになる",
				current: domain.OnboardingStatusNameSet,
				next:    "TST-unknown",
				wantErr: domain.ErrUnknownOnboardingStatus,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := domain.CanTransitionOnboardingStatus(tt.current, tt.next)
				require.ErrorIs(t, err, tt.wantErr)
				assert.Equal(t, tt.want, got)
			})
		}
	})
}
