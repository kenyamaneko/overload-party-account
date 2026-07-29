package usecase

import (
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/stretchr/testify/assert"
)

func TestGameDayFor(t *testing.T) {
	t.Run("ゲーム日境界の判定", func(t *testing.T) {
		tests := []struct {
			name string
			t    time.Time
			want civil.Date
		}{
			{
				name: "UTC 19:59 (JST 04:59) のとき、前日がゲーム日になる",
				t:    time.Date(2024, 1, 1, 19, 59, 0, 0, time.UTC),
				want: civil.Date{Year: 2024, Month: 1, Day: 1},
			},
			{
				name: "UTC 20:00 (JST 05:00) のとき、当日がゲーム日になる",
				t:    time.Date(2024, 1, 1, 20, 0, 0, 0, time.UTC),
				want: civil.Date{Year: 2024, Month: 1, Day: 2},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				assert.Equal(t, tt.want, gameDayFor(tt.t))
			})
		}
	})
}
