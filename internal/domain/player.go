package domain

import (
	"time"

	"cloud.google.com/go/civil"
)

// Player はプレイヤーの基本情報である。レベルと経験値、初期陣営は含まない。
type Player struct {
	PlayerID         string
	FirebaseUID      string
	Name             *string
	IsPremium        bool
	EquippedIconNo   *int64
	OnboardingStatus string
	PremiumExpiresAt *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// PlayerProgression はプレイヤーの累計経験値である。
type PlayerProgression struct {
	PlayerID  string
	Exp       int64
	UpdatedAt time.Time
}

// PlayerDailyBattle はプレイヤーのゲーム日ごとのバトル回数である。
type PlayerDailyBattle struct {
	PlayerID         string
	GameDate         civil.Date
	DailyBattleCount int64
}

// PlayerSettings はプレイヤーの設定である。
type PlayerSettings struct {
	PlayerID      string
	Language      string
	BgmVolume     int64
	SeVolume      int64
	IsPushEnabled bool
	UpdatedAt     time.Time
}

// PlayerView は、プレイヤーの基本情報に、累計経験値と初期陣営を合わせた参照用の情報である。
type PlayerView struct {
	Player         Player
	Exp            int64
	InitialFaction *string
}

// PlayerInfo は、参照用の情報に、累計経験値と経験値係数から求めたレベルと、現在のレベルの中での経験値の進み具合を加えたプレイヤー情報である。
type PlayerInfo struct {
	View          PlayerView
	Level         int64
	LevelProgress LevelProgress
}
