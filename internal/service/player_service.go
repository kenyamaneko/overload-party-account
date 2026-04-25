package service

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/civil"

	gamelogic "github.com/kenyamaneko/overload-party-battle/packages/game-logic-constants-go"
	gamedesign "github.com/kenyamaneko/overload-party-common/packages/game-design-constants"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

const (
	configKeyFreeDailyBattleLimit  = "free_daily_battle_limit"
	ConfigKeyExpFormulaCoefficient = "exp_formula_coefficient"

	// gameDayOffset はゲーム日の計算に使用する UTC オフセットです。
	// ゲーム日は JST 05:00 にリセットされます（JST(UTC+9) - 5h = UTC+4h）。
	gameDayOffset = 4 * time.Hour
)

// PlayerService はプレイヤー情報の参照・更新を提供します。
type PlayerService struct {
	playerRepo     port.PlayerRepo
	gameConfigRepo port.GameConfigRepo
	factionRepo    port.FactionRepo
	txRunner       port.TxRunner
}

// NewPlayerService は PlayerService を生成します。
func NewPlayerService(
	playerRepo port.PlayerRepo,
	gameConfigRepo port.GameConfigRepo,
	factionRepo port.FactionRepo,
	txRunner port.TxRunner,
) *PlayerService {
	return &PlayerService{
		playerRepo:     playerRepo,
		gameConfigRepo: gameConfigRepo,
		factionRepo:    factionRepo,
		txRunner:       txRunner,
	}
}

// FindByFirebaseUID は Firebase UID でプレイヤーを検索します。
func (s *PlayerService) FindByFirebaseUID(ctx context.Context, firebaseUID string) (*apiaccount.Player, error) {
	return s.playerRepo.FindByFirebaseUID(ctx, firebaseUID)
}

// UpdatePremium はプレミアムステータスを更新します。
func (s *PlayerService) UpdatePremium(ctx context.Context, playerID string, isPremium bool, expiresAtMillis *int64) error {
	var expiresAt *time.Time
	if expiresAtMillis != nil {
		t := time.UnixMilli(*expiresAtMillis)
		expiresAt = &t
	}
	return s.playerRepo.UpdatePremium(ctx, playerID, isPremium, expiresAt)
}

// UpdateFaction は選択ファクションを更新します。
func (s *PlayerService) UpdateFaction(ctx context.Context, playerID, faction string) error {
	return s.playerRepo.UpdateFaction(ctx, playerID, faction)
}

// GrantFaction はプレイヤーにファクションを付与します。
func (s *PlayerService) GrantFaction(ctx context.Context, playerID, faction, source string) error {
	return s.factionRepo.AddPlayerFaction(ctx, playerID, faction, source)
}

// ListFactions はプレイヤーの所持ファクション一覧を返します。
func (s *PlayerService) ListFactions(ctx context.Context, playerID string) ([]string, error) {
	return s.factionRepo.GetPlayerFactions(ctx, playerID)
}

// UpdateName はプレイヤー名を更新し、更新後の Player アグリゲートを返します。
// repo の write プリミティブは name のみを書き、ここで FindByID して
// サーバー側で組み立てた最新の Player をクライアントに返す
// (「server を信じる」原則。クライアントに整合の責任を押し付けない)。
//
// UPDATE と FindByID は別操作 (TX で囲っていない)。同一プレイヤーが
// 100ms 程度の間に名前変更を多重に投げる挙動は実用上想定しないため、
// read-after-write の整合性は許容する。問題が出たら txRunner で囲む。
func (s *PlayerService) UpdateName(ctx context.Context, playerID string, name string) (*apiaccount.Player, error) {
	if err := s.playerRepo.UpdateName(ctx, playerID, name); err != nil {
		return nil, fmt.Errorf("update name: %w", err)
	}
	player, err := s.playerRepo.FindByID(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("reload player: %w", err)
	}
	return player, nil
}

// GetPlayer はプレイヤー情報を返します。
func (s *PlayerService) GetPlayer(ctx context.Context, playerID string) (*apiaccount.Player, error) {
	player, err := s.playerRepo.FindByID(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("find player: %w", err)
	}
	return player, nil
}

// GetBattleLimit はプレイヤーの日次バトル制限情報を返します。
func (s *PlayerService) GetBattleLimit(ctx context.Context, playerID string) (*apiaccount.BattleLimitResponse, error) {
	player, err := s.playerRepo.FindByID(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("find player: %w", err)
	}

	if player.IsPremium {
		return &apiaccount.BattleLimitResponse{
			DailyBattleCount: 0,
			DailyBattleLimit: -1,
			CanBattle:        true,
		}, nil
	}

	db, err := s.playerRepo.GetDailyBattle(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("get daily battle: %w", err)
	}

	today := gameDay()
	count := db.DailyBattleCount
	if db.LastResetDate != today {
		count = 0 // Will be reset on next increment
	}

	freeLimit, err := s.gameConfigRepo.GetInt64(ctx, configKeyFreeDailyBattleLimit)
	if err != nil {
		return nil, fmt.Errorf("get free battle limit: %w", err)
	}
	if freeLimit == 0 {
		return nil, fmt.Errorf("game config %q is not set", configKeyFreeDailyBattleLimit)
	}

	return &apiaccount.BattleLimitResponse{
		DailyBattleCount: count,
		DailyBattleLimit: freeLimit,
		CanBattle:        count < freeLimit,
	}, nil
}

// IncrementBattleCount は日次バトル回数を 1 加算して書き込みます。
// account が daily_battle の authoritative owner なので、上限不変条件はここで守ります。
//
// 仕様:
//   - last_reset_date がゲーム日と異なる場合、カウントを 1 にリセットする
//   - free プレイヤーは加算後のカウントが上限を超える場合 ErrBattleLimitExceeded を返す
//   - premium プレイヤーは上限判定をスキップする（カウント自体は記録する）
//
// TOCTOU: GetDailyBattle と UpdateDailyBattleCount の間で別リクエストの書き込みが
// 入る可能性はあるが、同一アカウントの並行バトルは極めて稀なエッジケースとして許容する。
func (s *PlayerService) IncrementBattleCount(ctx context.Context, playerID string) error {
	player, err := s.playerRepo.FindByID(ctx, playerID)
	if err != nil {
		return fmt.Errorf("find player: %w", err)
	}

	db, err := s.playerRepo.GetDailyBattle(ctx, playerID)
	if err != nil {
		return fmt.Errorf("get daily battle: %w", err)
	}
	if db == nil {
		return fmt.Errorf("daily battle for player %s: %w", playerID, port.ErrNotFound)
	}

	today := gameDay()
	nextCount := db.DailyBattleCount + 1
	if db.LastResetDate != today {
		nextCount = 1
	}

	if !player.IsPremium {
		freeLimit, err := s.gameConfigRepo.GetInt64(ctx, configKeyFreeDailyBattleLimit)
		if err != nil {
			return fmt.Errorf("get free battle limit: %w", err)
		}
		if freeLimit <= 0 {
			return fmt.Errorf("game config %q is not set", configKeyFreeDailyBattleLimit)
		}
		if nextCount > freeLimit {
			return ErrBattleLimitExceeded
		}
	}

	if err := s.playerRepo.UpdateDailyBattleCount(ctx, playerID, nextCount, today); err != nil {
		return fmt.Errorf("update daily battle: %w", err)
	}
	return nil
}

// AwardExp はプレイヤーに経験値を付与しレベルを再計算します。
// 取得 → 計算 → 書き込み を同一トランザクションで直列化し、並行 AwardExp による
// ロストアップデートを SELECT FOR UPDATE の行ロックで防ぎます。
func (s *PlayerService) AwardExp(ctx context.Context, playerID string, expGain int64) error {
	if expGain <= 0 {
		return nil
	}
	coeff, err := s.gameConfigRepo.GetInt64(ctx, ConfigKeyExpFormulaCoefficient)
	if err != nil {
		return fmt.Errorf("get exp_formula_coefficient: %w", err)
	}
	if coeff <= 0 {
		return fmt.Errorf("exp_formula_coefficient not configured in game_config")
	}

	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		prog, err := s.playerRepo.GetProgressionForUpdate(txCtx, playerID)
		if err != nil {
			return fmt.Errorf("load progression: %w", err)
		}
		newExp := prog.Exp + expGain
		newLevel := ComputeLevel(newExp, prog.Level, coeff)
		if _, err := s.playerRepo.UpdateProgression(txCtx, playerID, newExp, newLevel); err != nil {
			return fmt.Errorf("persist progression: %w", err)
		}
		return nil
	})
}

// ComputeLevel は経験値獲得後の新レベルを算出します。
// 現在レベルからの増加のみ行い、減少はしません。これにより係数変更が既存レベルに遡及しません。
func ComputeLevel(newExp, currentLevel, coeff int64) int64 {
	level := currentLevel
	if level < 1 {
		level = 1
	}
	for {
		nextLevelExp := coeff * (level + 1) * (level + 1)
		if newExp < nextLevelExp {
			break
		}
		level++
	}
	return level
}

// AwardGameExp はゲーム終了後に両プレイヤーに経験値を付与します。
// NPC 戦（matchType == "npc"）では人間プレイヤー（player1）のみに付与します。
func (s *PlayerService) AwardGameExp(ctx context.Context, player1ID, player2ID string, winnerNum int64, reason, matchType string) error {
	expWin, err := s.gameConfigRepo.GetInt64(ctx, "exp_win")
	if err != nil {
		return fmt.Errorf("read exp_win: %w", err)
	}
	expLoss, err := s.gameConfigRepo.GetInt64(ctx, "exp_loss")
	if err != nil {
		return fmt.Errorf("read exp_loss: %w", err)
	}
	expDraw, err := s.gameConfigRepo.GetInt64(ctx, "exp_draw")
	if err != nil {
		return fmt.Errorf("read exp_draw: %w", err)
	}

	isNpc := matchType == gamedesign.MatchTypeNpc

	award := func(playerID string, exp int64, isNpcSide bool) error {
		if isNpcSide {
			return nil
		}
		return s.AwardExp(ctx, playerID, exp)
	}

	switch {
	case reason == gamelogic.WinReasonDraw || winnerNum == 0:
		if err := award(player1ID, expDraw, false); err != nil {
			return err
		}
		return award(player2ID, expDraw, isNpc)
	case winnerNum == 1:
		if err := award(player1ID, expWin, false); err != nil {
			return err
		}
		return award(player2ID, expLoss, isNpc)
	case winnerNum == 2:
		if err := award(player1ID, expLoss, false); err != nil {
			return err
		}
		return award(player2ID, expWin, isNpc)
	}
	return nil
}

// LevelProgress は現在レベル内の経験値進捗を保持します。
type LevelProgress struct {
	LevelExpCurrent  int64 `json:"level_exp_current"`
	LevelExpRequired int64 `json:"level_exp_required"`
}

// GetPlayerResponse はレベル進捗を付与したプレイヤー情報を返します。
func (s *PlayerService) GetPlayerResponse(ctx context.Context, playerID string) (*apiaccount.PlayerResponse, error) {
	player, err := s.GetPlayer(ctx, playerID)
	if err != nil {
		return nil, err
	}
	progress, err := s.GetLevelProgress(ctx, player.Level, player.Exp)
	if err != nil {
		return nil, err
	}
	return &apiaccount.PlayerResponse{
		PlayerID:         player.PlayerID,
		FirebaseUID:      player.FirebaseUID,
		Name:             player.Name,
		Level:            player.Level,
		Exp:              player.Exp,
		IsPremium:        player.IsPremium,
		EquippedIconNo:   player.EquippedIconNo,
		SelectedFaction:  player.SelectedFaction,
		PremiumExpiresAt: player.PremiumExpiresAt,
		CreatedAt:        player.CreatedAt,
		UpdatedAt:        player.UpdatedAt,
		LevelExpCurrent:  progress.LevelExpCurrent,
		LevelExpRequired: progress.LevelExpRequired,
	}, nil
}

// GetLevelProgress は指定レベル・経験値のレベル進捗を返します。
func (s *PlayerService) GetLevelProgress(ctx context.Context, level, exp int64) (*LevelProgress, error) {
	coeff, err := s.gameConfigRepo.GetInt64(ctx, ConfigKeyExpFormulaCoefficient)
	if err != nil {
		return nil, fmt.Errorf("get exp_formula_coefficient: %w", err)
	}
	if coeff <= 0 {
		return nil, fmt.Errorf("exp_formula_coefficient not configured in game_config")
	}
	return ComputeLevelProgress(level, exp, coeff), nil
}

// ComputeLevelProgress は現在レベル内の経験値進捗を計算します。
func ComputeLevelProgress(level, exp, coeff int64) *LevelProgress {
	currentLevelExp := coeff * level * level
	nextLevelExp := coeff * (level + 1) * (level + 1)
	return &LevelProgress{
		LevelExpCurrent:  max(0, exp-currentLevelExp),
		LevelExpRequired: nextLevelExp - currentLevelExp,
	}
}

func gameDay() civil.Date {
	return civil.DateOf(time.Now().UTC().Add(gameDayOffset))
}
