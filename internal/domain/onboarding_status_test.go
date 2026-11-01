package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

func TestCanTransitionOnboardingStatus(t *testing.T) {
	t.Run("[オンボーディング]オンボーディング進行状態の遷移可否の判定", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			tests := []struct {
				name    string
				current string
				next    string
			}{
				{name: "現在がnot_startedで次がname_setのとき、遷移は許容される", current: "not_started", next: "name_set"},
				{name: "現在がname_setで次がfaction_setのとき、遷移は許容される", current: "name_set", next: "faction_set"},
				{name: "現在がfaction_setで次がcompletedのとき、遷移は許容される", current: "faction_set", next: "completed"},
				{name: "現在も次もnot_startedのとき、遷移は許容される", current: "not_started", next: "not_started"},
				{name: "現在も次もname_setのとき、遷移は許容される", current: "name_set", next: "name_set"},
				{name: "現在も次もfaction_setのとき、遷移は許容される", current: "faction_set", next: "faction_set"},
				{name: "現在も次もcompletedのとき、遷移は許容される", current: "completed", next: "completed"},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					got, err := domain.CanTransitionOnboardingStatus(tt.current, tt.next)

					require.NoError(t, err)
					assert.True(t, got)
				})
			}
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("段階を飛ばして進むとき", func(t *testing.T) {
				tests := []struct {
					name    string
					current string
					next    string
				}{
					{name: "現在がnot_startedで次がfaction_setのとき、段階を飛ばしていることを示すエラーになる", current: "not_started", next: "faction_set"},
					{name: "現在がname_setで次がcompletedのとき、段階を飛ばしていることを示すエラーになる", current: "name_set", next: "completed"},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						_, err := domain.CanTransitionOnboardingStatus(tt.current, tt.next)

						require.ErrorIs(t, err, domain.ErrOnboardingStatusSkipped)
					})
				}
			})

			t.Run("前のオンボーディング進行状態へ戻るとき", func(t *testing.T) {
				t.Run("判定結果を取得すると", func(t *testing.T) {
					tests := []struct {
						name    string
						current string
						next    string
					}{
						{name: "現在がname_setで次がnot_startedのとき、遷移は許容されない", current: "name_set", next: "not_started"},
						{name: "現在がfaction_setで次がname_setのとき、遷移は許容されない", current: "faction_set", next: "name_set"},
						{name: "現在がcompletedで次がfaction_setのとき、遷移は許容されない", current: "completed", next: "faction_set"},
					}
					for _, tt := range tests {
						t.Run(tt.name, func(t *testing.T) {
							got, _ := domain.CanTransitionOnboardingStatus(tt.current, tt.next)

							assert.False(t, got)
						})
					}
				})

				t.Run("判定のエラーを取得すると", func(t *testing.T) {
					tests := []struct {
						name    string
						current string
						next    string
					}{
						{name: "現在がname_setで次がnot_startedのとき、エラーにならない", current: "name_set", next: "not_started"},
						{name: "現在がfaction_setで次がname_setのとき、エラーにならない", current: "faction_set", next: "name_set"},
						{name: "現在がcompletedで次がfaction_setのとき、エラーにならない", current: "completed", next: "faction_set"},
					}
					for _, tt := range tests {
						t.Run(tt.name, func(t *testing.T) {
							_, err := domain.CanTransitionOnboardingStatus(tt.current, tt.next)

							require.NoError(t, err)
						})
					}
				})
			})
		})
	})
}
