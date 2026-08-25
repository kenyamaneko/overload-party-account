package usecase

import (
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/stretchr/testify/assert"
)

func TestGameDayFor(t *testing.T) {
	t.Run("ゲーム日境界計算", func(t *testing.T) {
		jst := time.FixedZone("JST", 9*3600)

		t.Run("時刻がJST05:00:00ちょうどのとき、その時刻が属するゲーム日は当日の日付になる", func(t *testing.T) {
			day := gameDayFor(time.Date(2026, 8, 24, 5, 0, 0, 0, jst))

			assert.Equal(t, civil.Date{Year: 2026, Month: 8, Day: 24}, day)
		})

		t.Run("時刻がJST04:59:59(1秒前)のとき、その時刻が属するゲーム日は前日の日付になる", func(t *testing.T) {
			day := gameDayFor(time.Date(2026, 8, 24, 4, 59, 59, 0, jst))

			assert.Equal(t, civil.Date{Year: 2026, Month: 8, Day: 23}, day)
		})
	})
}
