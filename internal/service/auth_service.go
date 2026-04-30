package service

import (
	"context"
	"errors"
	"fmt"
	"time"

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

// Register は新規プレイヤーを登録します。表示名は Register では取得せず、
// オンボーディング完了時の player-onboarded イベントで初めて確定します
// (オンボーディング途中での中断後に再登録を要求しないための分離)。
func (s *AuthService) Register(ctx context.Context, firebaseUID string) (*apiaccount.Player, error) {
	_, err := s.playerRepo.FindByFirebaseUID(ctx, firebaseUID)
	if err == nil {
		return nil, ErrPlayerAlreadyRegistered
	}
	if !errors.Is(err, port.ErrNotFound) {
		return nil, fmt.Errorf("check existing player: %w", err)
	}

	now := time.Now()
	player := &apiaccount.Player{
		PlayerID:         uuid.New().String(),
		FirebaseUID:      firebaseUID,
		Name:             nil,
		Level:            1,
		Exp:              0,
		IsPremium:        false,
		OnboardingStatus: model.OnboardingStatusNotStarted,
		CreatedAt:        now,
		UpdatedAt:        now,
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

	// カードパック配布は登録時には行わない。初回ファクション選択時に
	// gateway がオーケストレーションする（player_factions が冪等性キー）。
	return player, nil
}

// FindByFirebaseUID は Firebase UID でプレイヤーを検索します。
// 内部 API (gateway などサービス間の UID→Player ルックアップ) 用の純粋な参照系で、
// ログインという業務イベントを伴わない点が Login との違いです。
func (s *AuthService) FindByFirebaseUID(ctx context.Context, firebaseUID string) (*apiaccount.Player, error) {
	return s.playerRepo.FindByFirebaseUID(ctx, firebaseUID)
}

// Login は Firebase UID でプレイヤーを検索しログインします。
func (s *AuthService) Login(ctx context.Context, firebaseUID string) (*apiaccount.Player, error) {
	player, err := s.playerRepo.FindByFirebaseUID(ctx, firebaseUID)
	if errors.Is(err, port.ErrNotFound) {
		return nil, ErrPlayerNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find player: %w", err)
	}
	return player, nil
}
