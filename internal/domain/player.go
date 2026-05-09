package domain

import (
	"time"

	"cloud.google.com/go/civil"
)

// Player は account.players テーブルの 1 行を表す Write 集約。Level/Exp/InitialFaction はこの型に含めない (それぞれ player_progression / player_factions の責務)。読み取り時に合成が必要な場合は PlayerView を使う。
type Player struct {
	PlayerID         string     `db:"player_id"`
	FirebaseUID      string     `db:"firebase_uid"`
	Name             *string    `db:"name"`
	IsPremium        bool       `db:"is_premium"`
	EquippedIconNo   *int64     `db:"equipped_icon_no"`
	OnboardingStatus string     `db:"onboarding_status"`
	PremiumExpiresAt *time.Time `db:"premium_expires_at"`
	CreatedAt        time.Time  `db:"created_at"`
	UpdatedAt        time.Time  `db:"updated_at"`
}

// PlayerProgression は account.player_progression の 1 行。バトル毎の高頻度 UPDATE を players から分離するための独立集約。
type PlayerProgression struct {
	PlayerID  string    `db:"player_id"`
	Level     int64     `db:"level"`
	Exp       int64     `db:"exp"`
	UpdatedAt time.Time `db:"updated_at"`
}

// PlayerDailyBattle は account.player_daily_battle の 1 行 (PK: player_id, game_date)。1 プレイヤーにつきゲーム日ごとに 1 行の履歴台帳。
type PlayerDailyBattle struct {
	PlayerID         string     `db:"player_id"`
	GameDate         civil.Date `db:"game_date"`
	DailyBattleCount int64      `db:"daily_battle_count"`
}

// PlayerFaction は account.player_factions の 1 行。IsInitial=TRUE の行はオンボーディングで選択した faction (1 プレイヤーに最大 1 行)。
type PlayerFaction struct {
	PlayerID   string    `db:"player_id"`
	Faction    string    `db:"faction"`
	IsInitial  bool      `db:"is_initial"`
	AcquiredAt time.Time `db:"acquired_at"`
}

// PlayerSettings は account.player_settings の 1 行。
type PlayerSettings struct {
	PlayerID    string    `db:"player_id"`
	Language    string    `db:"language"`
	BgmVolume   int64     `db:"bgm_volume"`
	SeVolume    int64     `db:"se_volume"`
	PushEnabled bool      `db:"push_enabled"`
	UpdatedAt   time.Time `db:"updated_at"`
}

// PlayerView は Read Model: Player (players 行) + PlayerProgression の Level/Exp + player_factions.is_initial=TRUE 行の Faction 名 を 1 つに束ねた、単一の参照向けビュー。リポジトリ層が JOIN で組み立てる読み取り専用ビューであり、書き込み経路では使わない。
type PlayerView struct {
	Player         Player
	Level          int64
	Exp            int64
	InitialFaction *string
}
