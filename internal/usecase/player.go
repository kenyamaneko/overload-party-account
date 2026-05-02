package usecase

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/civil"

	gamelogic "github.com/kenyamaneko/overload-party-battle/packages/game-logic-constants-go"
	gamedesign "github.com/kenyamaneko/overload-party-common/packages/game-design-constants"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

const (
	configKeyFreeDailyBattleLimit  = "free_daily_battle_limit"
	ConfigKeyExpFormulaCoefficient = "exp_formula_coefficient"

	// gameDayOffset はゲーム日の計算に使用する UTC オフセット (JST 05:00 リセット = UTC+4h)。
	gameDayOffset = 4 * time.Hour
)

// PlayerInteractor はプレイヤー情報の参照・更新を提供する。
type PlayerInteractor struct {
	playerRepo     port.PlayerRepo
	playerViewRepo port.PlayerViewRepo
	gameConfigRepo port.GameConfigRepo
	factionRepo    port.FactionRepo
	txRunner       port.TxRunner
}

// NewPlayerInteractor は PlayerInteractor を生成する。
func NewPlayerInteractor(
	playerRepo port.PlayerRepo,
	playerViewRepo port.PlayerViewRepo,
	gameConfigRepo port.GameConfigRepo,
	factionRepo port.FactionRepo,
	txRunner port.TxRunner,
) *PlayerInteractor {
	return &PlayerInteractor{
		playerRepo:     playerRepo,
		playerViewRepo: playerViewRepo,
		gameConfigRepo: gameConfigRepo,
		factionRepo:    factionRepo,
		txRunner:       txRunner,
	}
}

// UpdatePremium はプレミアムステータスを更新する。
func (s *PlayerInteractor) UpdatePremium(ctx context.Context, playerID string, isPremium bool, expiresAtMillis *int64) error {
	var expiresAt *time.Time
	if expiresAtMillis != nil {
		t := time.UnixMilli(*expiresAtMillis)
		expiresAt = &t
	}
	return s.playerRepo.UpdatePremium(ctx, playerID, isPremium, expiresAt)
}

// GrantFaction はプレイヤーにファクションを付与する。
func (s *PlayerInteractor) GrantFaction(ctx context.Context, playerID, faction string) error {
	return s.factionRepo.AddPlayerFaction(ctx, playerID, faction)
}

// ListFactions はプレイヤーの所持ファクション一覧を返す。
func (s *PlayerInteractor) ListFactions(ctx context.Context, playerID string) ([]string, error) {
	return s.factionRepo.GetPlayerFactions(ctx, playerID)
}

// UpdateName はプレイヤー名を更新し、更新後の PlayerResponse を返す。
// UPDATE と PlayerView 取得は同 Tx で囲まない: 同一プレイヤーが 100ms 程度の間に
// 名前変更を多重に投げる挙動は実用上想定しないため、read-after-write の整合性は許容する。
func (s *PlayerInteractor) UpdateName(ctx context.Context, playerID string, name string) (*apiaccount.PlayerResponse, error) {
	if err := domain.ValidateName(name); err != nil {
		return nil, err
	}
	if err := s.playerRepo.UpdateName(ctx, playerID, name); err != nil {
		return nil, fmt.Errorf("update name: %w", err)
	}
	return s.GetPlayerResponse(ctx, playerID)
}

// ValidateOnboardingName は表示名のバリデーションのみを行う。書き込みはしない。
// scenario が onboarding-name-set publish 前に呼んで 4xx を同期にユーザーへ返すための専用エントリ。
func (s *PlayerInteractor) ValidateOnboardingName(ctx context.Context, playerID, name string) error {
	exists, err := s.playerRepo.Exists(ctx, playerID)
	if err != nil {
		return fmt.Errorf("check player exists: %w", err)
	}
	if !exists {
		return fmt.Errorf("player %s: %w", playerID, port.ErrNotFound)
	}
	return domain.ValidateName(name)
}

// GetBattleLimit はプレイヤーの日次バトル制限情報を返す。
func (s *PlayerInteractor) GetBattleLimit(ctx context.Context, playerID string) (*apiaccount.BattleLimitResponse, error) {
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

	today := gameDay()
	db, err := s.playerRepo.GetDailyBattle(ctx, playerID, today)
	if err != nil {
		return nil, fmt.Errorf("get daily battle: %w", err)
	}

	var count int64
	if db != nil {
		count = db.DailyBattleCount
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

// IncrementBattleCount は当日のバトル回数を 1 加算する。
// 仕様は FEATURE_SPEC §4.3、TOCTOU の判断は ARCHITECTURE.md §4.3 を参照。
func (s *PlayerInteractor) IncrementBattleCount(ctx context.Context, playerID string) error {
	player, err := s.playerRepo.FindByID(ctx, playerID)
	if err != nil {
		return fmt.Errorf("find player: %w", err)
	}

	today := gameDay()

	if !player.IsPremium {
		freeLimit, err := s.gameConfigRepo.GetInt64(ctx, configKeyFreeDailyBattleLimit)
		if err != nil {
			return fmt.Errorf("get free battle limit: %w", err)
		}
		if freeLimit <= 0 {
			return fmt.Errorf("game config %q is not set", configKeyFreeDailyBattleLimit)
		}
		db, err := s.playerRepo.GetDailyBattle(ctx, playerID, today)
		if err != nil {
			return fmt.Errorf("get daily battle: %w", err)
		}
		var current int64
		if db != nil {
			current = db.DailyBattleCount
		}
		if current+1 > freeLimit {
			return ErrBattleLimitExceeded
		}
	}

	if _, err := s.playerRepo.IncrementDailyBattleCount(ctx, playerID, today); err != nil {
		return fmt.Errorf("increment daily battle: %w", err)
	}
	return nil
}

// AwardExp はプレイヤーに経験値を付与しレベルを再計算する。
// 並行 AwardExp によるロストアップデートを SELECT FOR UPDATE で防ぐため、
// 取得・計算・書き込みを単一 Tx で直列化する。
func (s *PlayerInteractor) AwardExp(ctx context.Context, playerID string, expGain int64) error {
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

// ComputeLevel は経験値獲得後の新レベルを算出する。契約は FEATURE_SPEC §6.3 を参照。
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

// AwardGameExp はゲーム終了後に両プレイヤーに経験値を付与する唯一の入口。
// 付与ルール (winnerNum / reason / matchType による分岐) は FEATURE_SPEC §6.1 を参照。
func (s *PlayerInteractor) AwardGameExp(ctx context.Context, player1ID, player2ID string, winnerNum int64, reason, matchType string) error {
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

// LevelProgress は現在レベル内の経験値進捗を保持する。
type LevelProgress struct {
	LevelExpCurrent  int64 `json:"level_exp_current"`
	LevelExpRequired int64 `json:"level_exp_required"`
}

// GetPlayerResponse はレベル進捗を付与したプレイヤー情報を返す。
func (s *PlayerInteractor) GetPlayerResponse(ctx context.Context, playerID string) (*apiaccount.PlayerResponse, error) {
	view, err := s.playerViewRepo.FindByID(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("find player view: %w", err)
	}
	coeff, err := s.gameConfigRepo.GetInt64(ctx, ConfigKeyExpFormulaCoefficient)
	if err != nil {
		return nil, fmt.Errorf("get exp_formula_coefficient: %w", err)
	}
	if coeff <= 0 {
		return nil, fmt.Errorf("exp_formula_coefficient not configured in game_config")
	}
	return BuildPlayerResponse(view, coeff), nil
}

// GetLevelProgress は指定レベル・経験値のレベル進捗を返す。
func (s *PlayerInteractor) GetLevelProgress(ctx context.Context, level, exp int64) (*LevelProgress, error) {
	coeff, err := s.gameConfigRepo.GetInt64(ctx, ConfigKeyExpFormulaCoefficient)
	if err != nil {
		return nil, fmt.Errorf("get exp_formula_coefficient: %w", err)
	}
	if coeff <= 0 {
		return nil, fmt.Errorf("exp_formula_coefficient not configured in game_config")
	}
	return ComputeLevelProgress(level, exp, coeff), nil
}

// ComputeLevelProgress は現在レベル内の経験値進捗を計算する。
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
