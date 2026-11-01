package system

import "time"

// SystemClock は実行環境の時計から現在時刻を返す。
type SystemClock struct{}

// NewSystemClock は SystemClock を生成する。
func NewSystemClock() *SystemClock {
	return &SystemClock{}
}

// Now は現在時刻を返す。
func (*SystemClock) Now() time.Time {
	return time.Now()
}
