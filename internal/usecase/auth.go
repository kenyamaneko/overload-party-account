package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// AuthInteractor はプレイヤーの登録・ログインを管理する。
type AuthInteractor struct {
	playerRepo         port.PlayerRepo
	playerViewRepo     port.PlayerViewRepo
	playerSettingsRepo port.PlayerSettingsRepo
	gameConfigRepo     port.GameConfigRepo
	txRunner           port.TxRunner
}

// NewAuthInteractor は AuthInteractor を生成する。
func NewAuthInteractor(
	playerRepo port.PlayerRepo,
	playerViewRepo port.PlayerViewRepo,
	playerSettingsRepo port.PlayerSettingsRepo,
	gameConfigRepo port.GameConfigRepo,
	txRunner port.TxRunner,
) *AuthInteractor {
	return &AuthInteractor{
		playerRepo:         playerRepo,
		playerViewRepo:     playerViewRepo,
		playerSettingsRepo: playerSettingsRepo,
		gameConfigRepo:     gameConfigRepo,
		txRunner:           txRunner,
	}
}

// Register は新規プレイヤーを登録する。表示名は別途 onboarding-name-set
// イベントで確定する (オンボード途中中断後に再登録を要求しない設計)。
func (s *AuthInteractor) Register(ctx context.Context, firebaseUID string) (*apiaccount.PlayerResponse, error) {
	_, err := s.playerRepo.FindByFirebaseUID(ctx, firebaseUID)
	if err == nil {
		return nil, ErrPlayerAlreadyRegistered
	}
	if !errors.Is(err, port.ErrNotFound) {
		return nil, fmt.Errorf("check existing player: %w", err)
	}

	now := time.Now()
	playerID := uuid.New().String()
	progression := &domain.PlayerProgression{
		PlayerID:  playerID,
		Level:     1,
		Exp:       0,
		UpdatedAt: now,
	}
	player := &domain.Player{
		PlayerID:         playerID,
		FirebaseUID:      firebaseUID,
		Name:             nil,
		IsPremium:        false,
		OnboardingStatus: domain.OnboardingStatusNotStarted,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	settings := &domain.PlayerSettings{
		PlayerID:    playerID,
		Language:    domain.DefaultLanguage,
		BgmVolume:   domain.DefaultBgmVolume,
		SeVolume:    domain.DefaultSeVolume,
		PushEnabled: domain.DefaultPushEnabled,
		UpdatedAt:   now,
	}

	if err := s.txRunner.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.playerRepo.Create(ctx, player, progression); err != nil {
			return fmt.Errorf("create player: %w", err)
		}
		if err := s.playerSettingsRepo.Insert(ctx, settings); err != nil {
			return fmt.Errorf("create default player settings: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	view := &domain.PlayerView{
		Player:         *player,
		Level:          progression.Level,
		Exp:            progression.Exp,
		// 登録時点では initial_faction 未選択。onboarding-faction-set イベントの
		// subscriber が player_factions に is_initial=TRUE 行を入れて確定させる。
		InitialFaction: nil,
	}
	return s.toResponse(ctx, view)
}

// FindByFirebaseUID は Firebase UID でプレイヤーを検索する。
// gateway 等の内部 API 用で、Login と異なり業務イベントを伴わない参照系。
func (s *AuthInteractor) FindByFirebaseUID(ctx context.Context, firebaseUID string) (*apiaccount.PlayerResponse, error) {
	view, err := s.playerViewRepo.FindByFirebaseUID(ctx, firebaseUID)
	if err != nil {
		return nil, err
	}
	return s.toResponse(ctx, view)
}

// Login は Firebase UID でプレイヤーを検索しログインする。
func (s *AuthInteractor) Login(ctx context.Context, firebaseUID string) (*apiaccount.PlayerResponse, error) {
	view, err := s.playerViewRepo.FindByFirebaseUID(ctx, firebaseUID)
	if errors.Is(err, port.ErrNotFound) {
		return nil, ErrPlayerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find player: %w", err)
	}
	return s.toResponse(ctx, view)
}

func (s *AuthInteractor) toResponse(ctx context.Context, view *domain.PlayerView) (*apiaccount.PlayerResponse, error) {
	coeff, err := s.gameConfigRepo.GetInt64(ctx, ConfigKeyExpFormulaCoefficient)
	if err != nil {
		return nil, fmt.Errorf("get exp_formula_coefficient: %w", err)
	}
	return BuildPlayerResponse(view, coeff), nil
}
