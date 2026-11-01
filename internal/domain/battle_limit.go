package domain

// UnlimitedDailyBattleLimit は、1日のバトル回数に上限がないことを示す上限の値である。
const UnlimitedDailyBattleLimit int64 = -1

// BattleLimit はプレイヤーの1日のバトル回数と上限、もう1戦できるかを表す。
type BattleLimit struct {
	DailyBattleCount int64
	DailyBattleLimit int64
	CanBattle        bool
}
