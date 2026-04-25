package service

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/civil"
	"github.com/google/uuid"

	"github.com/kenyamaneko/overload-party-account/internal/model"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// AuthService はプレイヤーの登録・ログインを管理します。
type AuthService struct {
	playerRepo         port.PlayerRepo
	playerSettingsRepo port.PlayerSettingsRepo
	txRunner           port.TxRunner
}

// NewAuthService は AuthService を生成します。
func NewAuthService(playerRepo port.PlayerRepo, playerSettingsRepo port.PlayerSettingsRepo, txRunner port.TxRunner) *AuthService {
	return &AuthService{
		playerRepo:         playerRepo,
		playerSettingsRepo: playerSettingsRepo,
		txRunner:           txRunner,
	}
}

// Register は新規プレイヤーを登録します。
func (s *AuthService) Register(ctx context.Context, firebaseUID, name string) (*apiaccount.Player, error) {
	existing, err := s.playerRepo.FindByFirebaseUID(ctx, firebaseUID)
	if err != nil {
		return nil, fmt.Errorf("check existing player: %w", err)
	}
	if existing != nil {
		return nil, ErrPlayerAlreadyRegistered
	}

	now := time.Now()
	player := &apiaccount.Player{
		PlayerID:    uuid.New().String(),
		FirebaseUID: firebaseUID,
		Name:        name,
		Level:       1,
		Exp:         0,
		IsPremium:   false,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	dailyBattle := &apiaccount.PlayerDailyBattle{
		PlayerID:         player.PlayerID,
		DailyBattleCount: 0,
		LastResetDate:    civil.DateOf(time.Now().UTC()),
	}

	progression := &apiaccount.PlayerProgression{
		PlayerID:  player.PlayerID,
		Level:     1,
		Exp:       0,
		UpdatedAt: now,
	}

	settings := &apiaccount.PlayerSettings{
		PlayerID:    player.PlayerID,
		Language:    model.DefaultLanguage,
		BgmVolume:   model.DefaultBgmVolume,
		SeVolume:    model.DefaultSeVolume,
		PushEnabled: model.DefaultPushEnabled,
		UpdatedAt:   now,
	}

	if err := s.txRunner.RunInTx(ctx, func(ctx context.Context) error {
		if err := s.playerRepo.Create(ctx, player, dailyBattle, progression); err != nil {
			return fmt.Errorf("create player: %w", err)
		}
		if err := s.playerSettingsRepo.Insert(ctx, settings); err != nil {
			return fmt.Errorf("create default player settings: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// カードパック配布は登録時には行わない。初回ファクション選択時に
	// gateway がオーケストレーションする（player_factions が冪等性キー）。
	return player, nil
}

// Login は Firebase UID でプレイヤーを検索しログインします。
func (s *AuthService) Login(ctx context.Context, firebaseUID string) (*apiaccount.Player, error) {
	player, err := s.playerRepo.FindByFirebaseUID(ctx, firebaseUID)
	if err != nil {
		return nil, fmt.Errorf("find player: %w", err)
	}
	if player == nil {
		return nil, ErrPlayerNotFound
	}
	return player, nil
}
