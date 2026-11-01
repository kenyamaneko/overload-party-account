package presenter

import (
	"fmt"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// ToPlayerResponse はプレイヤー情報を API の応答の形に変換する。
func ToPlayerResponse(info *domain.PlayerInfo) (*apiaccount.PlayerResponse, error) {
	view := info.View
	onboardingStatus := apiaccount.OnboardingStatus(view.Player.OnboardingStatus)
	if !onboardingStatus.Valid() {
		return nil, fmt.Errorf("%w: %q", domain.ErrUnknownOnboardingStatus, view.Player.OnboardingStatus)
	}
	return &apiaccount.PlayerResponse{
		PlayerID:         view.Player.PlayerID,
		FirebaseUID:      view.Player.FirebaseUID,
		Name:             view.Player.Name,
		Level:            info.Level,
		Exp:              view.Exp,
		IsPremium:        view.Player.IsPremium,
		EquippedIconNo:   view.Player.EquippedIconNo,
		InitialFaction:   view.InitialFaction,
		OnboardingStatus: onboardingStatus,
		PremiumExpiresAt: view.Player.PremiumExpiresAt,
		CreatedAt:        view.Player.CreatedAt,
		UpdatedAt:        view.Player.UpdatedAt,
		LevelExpCurrent:  info.LevelProgress.LevelExpCurrent,
		LevelExpRequired: info.LevelProgress.LevelExpRequired,
	}, nil
}

// ToPlayerSettingsResponse はプレイヤー設定を API の応答の形に変換する。
func ToPlayerSettingsResponse(settings *domain.PlayerSettings) *apiaccount.PlayerSettingsResponse {
	return &apiaccount.PlayerSettingsResponse{
		PlayerID:    settings.PlayerID,
		Language:    settings.Language,
		BgmVolume:   settings.BgmVolume,
		SeVolume:    settings.SeVolume,
		PushEnabled: settings.IsPushEnabled,
		UpdatedAt:   settings.UpdatedAt,
	}
}

// ToBattleLimitResponse はバトル回数制限を API の応答の形に変換する。
func ToBattleLimitResponse(limit *domain.BattleLimit) *apiaccount.BattleLimitResponse {
	return &apiaccount.BattleLimitResponse{
		DailyBattleCount: limit.DailyBattleCount,
		DailyBattleLimit: limit.DailyBattleLimit,
		CanBattle:        limit.CanBattle,
	}
}
