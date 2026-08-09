package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/civil"

	gamelogic "github.com/kenyamaneko/overload-party-battle/packages/game-logic-constants-go"
	gamedesign "github.com/kenyamaneko/overload-party-common/packages/game-design-constants"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/presenter"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

const (
	configKeyFreeDailyBattleLimit  = "free_daily_battle_limit"
	ConfigKeyExpFormulaCoefficient = "exp_formula_coefficient"

	// gameDayOffset はゲーム日リセット境界を JST 05:00 に揃えるための UTC オフセット。
	// time.Now().UTC().Add(gameDayOffset) の日付部分が「ゲーム日」になり、
	// 各プレイヤーの daily_battle_count はこのゲーム日単位で集計される。
	// リセット時刻の根拠は ARCHITECTURE.md を参照。
	gameDayOffset = 4 * time.Hour
)

// PlayerInteractor はプレイヤー情報の参照・更新を提供する。
type PlayerInteractor struct {
	playerRepo              port.PlayerRepo
	premiumRepo             port.PlayerPremiumRepo
	progressionRepo         port.PlayerProgressionRepo
	battleRepo              port.PlayerBattleRepo
	battleCountReversalRepo port.BattleCountReversalRepo
	playerViewRepo          port.PlayerViewRepo
	gameConfigRepo          port.GameConfigRepo
	txRunner                port.TxRunner
}

// NewPlayerInteractor は PlayerInteractor を生成する。
func NewPlayerInteractor(
	playerRepo port.PlayerRepo,
	premiumRepo port.PlayerPremiumRepo,
	progressionRepo port.PlayerProgressionRepo,
	battleRepo port.PlayerBattleRepo,
	battleCountReversalRepo port.BattleCountReversalRepo,
	playerViewRepo port.PlayerViewRepo,
	gameConfigRepo port.GameConfigRepo,
	txRunner port.TxRunner,
) *PlayerInteractor {
	return &PlayerInteractor{
		playerRepo:              playerRepo,
		premiumRepo:             premiumRepo,
		progressionRepo:         progressionRepo,
		battleRepo:              battleRepo,
		battleCountReversalRepo: battleCountReversalRepo,
		playerViewRepo:          playerViewRepo,
		gameConfigRepo:          gameConfigRepo,
		txRunner:                txRunner,
	}
}

// UpdatePremium はプレミアムステータスを更新する。
func (s *PlayerInteractor) UpdatePremium(ctx context.Context, playerID string, isPremium bool, expiresAtMillis *int64) error {
	var expiresAt *time.Time
	if expiresAtMillis != nil {
		t := time.UnixMilli(*expiresAtMillis)
		expiresAt = &t
	}
	return s.premiumRepo.UpdatePremium(ctx, playerID, isPremium, expiresAt)
}

// UpdateName はプレイヤー名を更新し、更新後の PlayerResponse を返す。
// UPDATE と PlayerView 取得は同 Tx で囲まない: 同一プレイヤーが 100ms 程度の間に
// 名前変更を多重に投げる挙動は実用上想定しないため、Tx 一体化のコストを払わない。
// (PostgreSQL の Read Committed では別 connection でも commit 済みは見えるので
// read-after-write が壊れる訳ではなく、あくまで「同 Tx で読み直す保証は取らない」設計判断)。
func (s *PlayerInteractor) UpdateName(ctx context.Context, playerID string, name string) (*apiaccount.PlayerResponse, error) {
	if err := domain.ValidateName(name); err != nil {
		return nil, err
	}
	if err := s.playerRepo.UpdateName(ctx, playerID, name); err != nil {
		return nil, fmt.Errorf("update name: %w", err)
	}
	return s.GetPlayerResponse(ctx, playerID)
}

// ValidateNameForOnboarding は表示名のバリデーションのみを行う。書き込みはしない。
// scenario が onboarding-name-set publish 前に呼んで 4xx を同期にユーザーへ返すための専用エントリ。
func (s *PlayerInteractor) ValidateNameForOnboarding(ctx context.Context, playerID, name string) error {
	isFound, err := s.playerRepo.Exists(ctx, playerID)
	if err != nil {
		return fmt.Errorf("check player exists: %w", err)
	}
	if !isFound {
		return fmt.Errorf("player %s: %w", playerID, port.ErrNotFound)
	}
	return domain.ValidateName(name)
}

// GetBattleLimit はプレイヤーの日次バトル制限情報を返す。
// プレミアム会員でも実カウントを返す: 上限を持たない (limit=-1) ため CanBattle 判定では
// 不要だが、データ分析のためカウント自体は free と同じく集計対象として読み出す。
func (s *PlayerInteractor) GetBattleLimit(ctx context.Context, playerID string) (*apiaccount.BattleLimitResponse, error) {
	player, err := s.playerRepo.FindByID(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("find player: %w", err)
	}

	today := computeCurrentGameDay()
	db, err := s.battleRepo.GetDailyBattle(ctx, playerID, today)
	if err != nil {
		return nil, fmt.Errorf("get daily battle: %w", err)
	}
	var count int64
	if db != nil {
		count = db.DailyBattleCount
	}

	if player.IsPremium {
		return &apiaccount.BattleLimitResponse{
			DailyBattleCount: count,
			DailyBattleLimit: -1,
			CanBattle:        true,
		}, nil
	}

	freeLimit, err := s.gameConfigRepo.GetInt64(ctx, configKeyFreeDailyBattleLimit)
	if err != nil {
		return nil, fmt.Errorf("get free battle limit: %w", err)
	}

	return &apiaccount.BattleLimitResponse{
		DailyBattleCount: count,
		DailyBattleLimit: freeLimit,
		CanBattle:        count < freeLimit,
	}, nil
}

// IncrementBattleCount は当日のバトル回数を 1 加算する。
// プレミアム会員も含め全プレイヤーで加算する (集計用)。free 上限の判定は
// ensureWithinFreeBattleLimit に分離。TOCTOU の判断は ARCHITECTURE.md を参照。
// FindByID (premium 判定用) と ensureWithinFreeBattleLimit 内の GetDailyBattle
// (上限判定用) で SELECT が 2 回走るのは TOCTOU を許容する代わりに事前判定の
// 簡潔さを優先したため。Tx で囲んで 1 回にまとめる価値はないと判断している。
func (s *PlayerInteractor) IncrementBattleCount(ctx context.Context, playerID string) error {
	player, err := s.playerRepo.FindByID(ctx, playerID)
	if err != nil {
		return fmt.Errorf("find player: %w", err)
	}
	today := computeCurrentGameDay()

	if err := s.ensureWithinFreeBattleLimit(ctx, player, today); err != nil {
		return err
	}

	if _, err := s.battleRepo.IncrementDailyBattleCount(ctx, playerID, today); err != nil {
		return fmt.Errorf("increment daily battle: %w", err)
	}
	return nil
}

// ensureWithinFreeBattleLimit は free プレイヤーが当日もう 1 戦できるかを検証する。
// プレミアム会員は短絡で許可。上限到達時は ErrBattleLimitExceeded を返す。
func (s *PlayerInteractor) ensureWithinFreeBattleLimit(ctx context.Context, player *domain.Player, today civil.Date) error {
	if player.IsPremium {
		return nil
	}
	freeLimit, err := s.gameConfigRepo.GetInt64(ctx, configKeyFreeDailyBattleLimit)
	if err != nil {
		return fmt.Errorf("get free battle limit: %w", err)
	}
	db, err := s.battleRepo.GetDailyBattle(ctx, player.PlayerID, today)
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
	return nil
}

// RevertBattleCount は停止で無効になった対戦の消費バトル回数を両プレイヤーに戻す。
// 同一 gameID の呼び出しは 1 回のみ適用され、以降は idempotent に成功を返す。
func (s *PlayerInteractor) RevertBattleCount(ctx context.Context, gameID string, consumedAtMillis int64, player1ID, player2ID string) error {
	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		isNew, err := s.battleCountReversalRepo.MarkReverted(txCtx, gameID)
		if err != nil {
			return fmt.Errorf("mark battle count reverted: %w", err)
		}
		if !isNew {
			return nil
		}

		gameDate := gameDayFor(time.UnixMilli(consumedAtMillis))
		if err := s.revertPlayerBattleCount(txCtx, player1ID, gameDate, gameID); err != nil {
			return err
		}
		return s.revertPlayerBattleCount(txCtx, player2ID, gameDate, gameID)
	})
}

// revertPlayerBattleCount は 1 プレイヤー分の消費バトル回数を戻す。対象日の加算記録が
// 見つからなければ、想定した日付と実際の加算日がずれている不整合の可能性として Warn ログを
// 残す (呼び出し自体は idempotent な成功として扱う)。
func (s *PlayerInteractor) revertPlayerBattleCount(ctx context.Context, playerID string, gameDate civil.Date, gameID string) error {
	reverted, err := s.battleRepo.DecrementDailyBattleCount(ctx, playerID, gameDate)
	if err != nil {
		return fmt.Errorf("decrement daily battle: %w", err)
	}
	if !reverted {
		slog.WarnContext(ctx, "battle count revert target not found",
			"player_id", playerID, "game_date", gameDate.String(), "game_id", gameID)
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

	return s.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		prog, err := s.progressionRepo.GetProgressionForUpdate(txCtx, playerID)
		if err != nil {
			return fmt.Errorf("load progression: %w", err)
		}
		newExp := prog.Exp + expGain
		newLevel, err := domain.ComputeLevel(newExp, prog.Level, coeff)
		if err != nil {
			return fmt.Errorf("compute level: %w", err)
		}
		if _, err := s.progressionRepo.UpdateProgression(txCtx, playerID, newExp, newLevel); err != nil {
			return fmt.Errorf("persist progression: %w", err)
		}
		return nil
	})
}

// AwardGameExp はゲーム終了後に両プレイヤーに経験値を付与する唯一の入口。
func (s *PlayerInteractor) AwardGameExp(ctx context.Context, player1ID, player2ID string, winnerNum int64, reason, matchType string) error {
	expWin, err := s.gameConfigRepo.GetInt64(ctx, gameConfigKeyExpWin)
	if err != nil {
		return fmt.Errorf("read %s: %w", gameConfigKeyExpWin, err)
	}
	expLoss, err := s.gameConfigRepo.GetInt64(ctx, gameConfigKeyExpLoss)
	if err != nil {
		return fmt.Errorf("read %s: %w", gameConfigKeyExpLoss, err)
	}
	expDraw, err := s.gameConfigRepo.GetInt64(ctx, gameConfigKeyExpDraw)
	if err != nil {
		return fmt.Errorf("read %s: %w", gameConfigKeyExpDraw, err)
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
	return presenter.BuildPlayerResponse(view, coeff)
}

// computeCurrentGameDay はゲーム日境界オフセットを適用した現在のゲーム内日付を返す。
func computeCurrentGameDay() civil.Date {
	return gameDayFor(time.Now())
}

// gameDayFor はゲーム日境界オフセットを適用した、指定時刻に対応するゲーム内日付を返す。
func gameDayFor(t time.Time) civil.Date {
	return civil.DateOf(t.UTC().Add(gameDayOffset))
}
