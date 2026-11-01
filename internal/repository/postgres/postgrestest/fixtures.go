package postgrestest

import (
	"time"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

const (
	// PlayerID1 は一人目のテスト用プレイヤーID。
	PlayerID1 = "11111111-1111-4111-8111-111111111111"
	// PlayerID2 は二人目のテスト用プレイヤーID。
	PlayerID2 = "22222222-2222-4222-8222-222222222222"
	// PlayerID3 は三人目のテスト用プレイヤーID。
	PlayerID3 = "33333333-3333-4333-8333-333333333333"
	// UnregisteredPlayerID は登録されないテスト用プレイヤーID。
	UnregisteredPlayerID = "99999999-9999-4999-8999-999999999999"

	// FirebaseUID1 は一人目のテスト用 FirebaseUID。
	FirebaseUID1 = "test-uid-1"
	// FirebaseUID2 は二人目のテスト用 FirebaseUID。
	FirebaseUID2 = "test-uid-2"
	// FirebaseUID3 は三人目のテスト用 FirebaseUID。
	FirebaseUID3 = "test-uid-3"

	// EventID1 は一つ目のテスト用イベントID。
	EventID1 = "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"
)

// NewUTCTime は UTC の時刻を返す。
func NewUTCTime(year int, month time.Month, day, hour, minute, second int) time.Time {
	return time.Date(year, month, day, hour, minute, second, 0, time.UTC)
}

// NewPointer は値へのポインタを返す。
func NewPointer[T any](v T) *T {
	return &v
}

// NewPlayer はオンボーディング前で作成日時と更新日時が 2026-01-01T00:00:00Z のプレイヤーを返す。
func NewPlayer(playerID, firebaseUID string) *domain.Player {
	return &domain.Player{
		PlayerID:         playerID,
		FirebaseUID:      firebaseUID,
		OnboardingStatus: "not_started",
		CreatedAt:        NewUTCTime(2026, time.January, 1, 0, 0, 0),
		UpdatedAt:        NewUTCTime(2026, time.January, 1, 0, 0, 0),
	}
}

// NewPlayerWith は NewPlayer のプレイヤーに configure で項目を設定して返す。
func NewPlayerWith(playerID, firebaseUID string, configure func(player *domain.Player)) *domain.Player {
	player := NewPlayer(playerID, firebaseUID)
	configure(player)
	return player
}

// NewProgression は経験値 0 で更新日時が 2026-01-01T00:00:00Z の進行状況を返す。
func NewProgression(playerID string) *domain.PlayerProgression {
	return &domain.PlayerProgression{
		PlayerID:  playerID,
		Exp:       0,
		UpdatedAt: NewUTCTime(2026, time.January, 1, 0, 0, 0),
	}
}

// NewProgressionWithExp は NewProgression の進行状況の経験値を指定して返す。
func NewProgressionWithExp(playerID string, exp int64) *domain.PlayerProgression {
	progression := NewProgression(playerID)
	progression.Exp = exp
	return progression
}

// NewSettings は言語「ja」、BGM音量 50、SE音量 50、通知許可有効で更新日時が 2026-01-01T00:00:00Z の設定を返す。
func NewSettings(playerID string) *domain.PlayerSettings {
	return &domain.PlayerSettings{
		PlayerID:      playerID,
		Language:      "ja",
		BgmVolume:     50,
		SeVolume:      50,
		IsPushEnabled: true,
		UpdatedAt:     NewUTCTime(2026, time.January, 1, 0, 0, 0),
	}
}

// NewSettingsWithValues は NewSettings の設定の言語・音量・通知許可を指定して返す。
func NewSettingsWithValues(playerID, language string, bgmVolume, seVolume int64, isPushEnabled bool) *domain.PlayerSettings {
	settings := NewSettings(playerID)
	settings.Language = language
	settings.BgmVolume = bgmVolume
	settings.SeVolume = seVolume
	settings.IsPushEnabled = isPushEnabled
	return settings
}
