package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// AuthInteractor はプレイヤーの登録とログインを行う。
type AuthInteractor struct {
	playerRepo         port.PlayerRepo
	playerViewRepo     port.PlayerViewRepo
	playerSettingsRepo port.PlayerSettingsRepo
	gameConfigRepo     port.GameConfigRepo
	txRunner           port.TxRunner
	clock              port.Clock
	playerIDGenerator  port.PlayerIDGenerator
}

// NewAuthInteractor は AuthInteractor を生成する。
func NewAuthInteractor(
	playerRepo port.PlayerRepo,
	playerViewRepo port.PlayerViewRepo,
	playerSettingsRepo port.PlayerSettingsRepo,
	gameConfigRepo port.GameConfigRepo,
	txRunner port.TxRunner,
	clock port.Clock,
	playerIDGenerator port.PlayerIDGenerator,
) *AuthInteractor {
	return &AuthInteractor{
		playerRepo:         playerRepo,
		playerViewRepo:     playerViewRepo,
		playerSettingsRepo: playerSettingsRepo,
		gameConfigRepo:     gameConfigRepo,
		txRunner:           txRunner,
		clock:              clock,
		playerIDGenerator:  playerIDGenerator,
	}
}

// Register は新しいプレイヤーを登録し、プレイヤー情報を返す。表示名は未設定で登録する。
// 同じ FirebaseUID で登録済みのときは ErrPlayerAlreadyRegistered を返す。
func (ai *AuthInteractor) Register(ctx context.Context, firebaseUID string) (*domain.PlayerInfo, error) {
	// 登録の後にゲーム設定を読めないと、登録済みのまま失敗を返して再登録が二重登録になるため、登録の前に読む。
	coefficient, err := readGameConfig(ctx, ai.gameConfigRepo, gameConfigKeyExpFormulaCoefficient)
	if err != nil {
		return nil, err
	}
	now := ai.clock.Now()
	playerID := ai.playerIDGenerator.GeneratePlayerID()
	progression := &domain.PlayerProgression{
		PlayerID:  playerID,
		Exp:       domain.InitialExp,
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
		PlayerID:      playerID,
		Language:      domain.DefaultLanguage,
		BgmVolume:     domain.DefaultBgmVolume,
		SeVolume:      domain.DefaultSeVolume,
		IsPushEnabled: domain.DefaultIsPushEnabled,
		UpdatedAt:     now,
	}

	view := &domain.PlayerView{
		Player:         *player,
		Exp:            progression.Exp,
		InitialFaction: nil,
	}
	info, err := buildPlayerInfo(view, coefficient)
	if err != nil {
		return nil, err
	}

	if err := ai.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		if err := ai.playerRepo.Create(txCtx, player, progression); err != nil {
			if errors.Is(err, port.ErrAlreadyExists) {
				return ErrPlayerAlreadyRegistered
			}
			return fmt.Errorf("create player: %w", err)
		}
		if err := ai.playerSettingsRepo.InsertPlayerSettings(txCtx, settings); err != nil {
			return fmt.Errorf("create default player settings: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return info, nil
}

// GetByFirebaseUID は FirebaseUID に対応するプレイヤー情報を返す。登録されていなければ port.ErrNotFound を包んだエラーを返す。
func (ai *AuthInteractor) GetByFirebaseUID(ctx context.Context, firebaseUID string) (*domain.PlayerInfo, error) {
	view, err := ai.playerViewRepo.GetByFirebaseUID(ctx, firebaseUID)
	if err != nil {
		return nil, fmt.Errorf("get player: %w", err)
	}
	return assemblePlayerInfo(ctx, ai.gameConfigRepo, view)
}

// Login は FirebaseUID に対応するプレイヤー情報を返す。登録されていなければ port.ErrNotFound を包んだエラーを返す。
func (ai *AuthInteractor) Login(ctx context.Context, firebaseUID string) (*domain.PlayerInfo, error) {
	return ai.GetByFirebaseUID(ctx, firebaseUID)
}
