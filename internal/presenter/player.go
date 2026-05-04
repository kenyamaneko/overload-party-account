package presenter

import (
	"github.com/kenyamaneko/overload-party-account/internal/domain"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// BuildPlayerResponse は Read Model (PlayerView) と exp 係数から API DTO を組み立てる。
// レベル進捗の計算は domain.ComputeLevelProgress に委譲し、presenter は wire 形への詰め替えのみ担う。
func BuildPlayerResponse(view *domain.PlayerView, coeff int64) *apiaccount.PlayerResponse {
	progress := domain.ComputeLevelProgress(view.Level, view.Exp, coeff)
	return &apiaccount.PlayerResponse{
		PlayerID:         view.Player.PlayerID,
		FirebaseUID:      view.Player.FirebaseUID,
		Name:             view.Player.Name,
		Level:            view.Level,
		Exp:              view.Exp,
		IsPremium:        view.Player.IsPremium,
		EquippedIconNo:   view.Player.EquippedIconNo,
		InitialFaction:   view.InitialFaction,
		OnboardingStatus: view.Player.OnboardingStatus,
		PremiumExpiresAt: view.Player.PremiumExpiresAt,
		CreatedAt:        view.Player.CreatedAt,
		UpdatedAt:        view.Player.UpdatedAt,
		LevelExpCurrent:  progress.LevelExpCurrent,
		LevelExpRequired: progress.LevelExpRequired,
	}
}

// BuildPlayerSettingsResponse は永続化された PlayerSettings を API DTO に写像する。
func BuildPlayerSettingsResponse(s *domain.PlayerSettings) *apiaccount.PlayerSettingsResponse {
	return &apiaccount.PlayerSettingsResponse{
		PlayerID:    s.PlayerID,
		Language:    s.Language,
		BgmVolume:   s.BgmVolume,
		SeVolume:    s.SeVolume,
		PushEnabled: s.PushEnabled,
		UpdatedAt:   s.UpdatedAt,
	}
}
