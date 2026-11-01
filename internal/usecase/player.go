package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"cloud.google.com/go/civil"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// gameDayOffset は、ゲーム日を日本時間の 5 時に切り替えるために UTC に加える時間である。
const gameDayOffset = 4 * time.Hour

// minRevertableDailyBattleCount は、バトル回数を戻せる最小の回数である。これ未満では 0 回を下回るため戻さない。
const minRevertableDailyBattleCount = 1

// PlayerInteractor はプレイヤー情報の参照と、表示名・バトル回数・経験値の更新を行う。
type PlayerInteractor struct {
	playerRepo              port.PlayerRepo
	progressionRepo         port.PlayerProgressionRepo
	battleRepo              port.PlayerBattleRepo
	battleCountReversalRepo port.BattleCountReversalRepo
	playerViewRepo          port.PlayerViewRepo
	gameConfigRepo          port.GameConfigRepo
	txRunner                port.TxRunner
	clock                   port.Clock
}

// NewPlayerInteractor は PlayerInteractor を生成する。
func NewPlayerInteractor(
	playerRepo port.PlayerRepo,
	progressionRepo port.PlayerProgressionRepo,
	battleRepo port.PlayerBattleRepo,
	battleCountReversalRepo port.BattleCountReversalRepo,
	playerViewRepo port.PlayerViewRepo,
	gameConfigRepo port.GameConfigRepo,
	txRunner port.TxRunner,
	clock port.Clock,
) *PlayerInteractor {
	return &PlayerInteractor{
		playerRepo:              playerRepo,
		progressionRepo:         progressionRepo,
		battleRepo:              battleRepo,
		battleCountReversalRepo: battleCountReversalRepo,
		playerViewRepo:          playerViewRepo,
		gameConfigRepo:          gameConfigRepo,
		txRunner:                txRunner,
		clock:                   clock,
	}
}

// GetPlayer はプレイヤー情報を返す。
func (pi *PlayerInteractor) GetPlayer(ctx context.Context, playerID string) (*domain.PlayerInfo, error) {
	view, err := pi.playerViewRepo.GetByID(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("get player view: %w", err)
	}
	return assemblePlayerInfo(ctx, pi.gameConfigRepo, view)
}

// UpdateName は表示名を更新し、更新後のプレイヤー情報を返す。
// 表示名が不正なときは domain.ErrInvalidName を返す。
func (pi *PlayerInteractor) UpdateName(ctx context.Context, playerID string, name string) (*domain.PlayerInfo, error) {
	if err := domain.ValidateName(name); err != nil {
		return nil, err
	}
	if err := pi.playerRepo.UpdateName(ctx, playerID, name); err != nil {
		return nil, fmt.Errorf("update name: %w", err)
	}
	return pi.GetPlayer(ctx, playerID)
}

// ValidateNameForOnboarding はオンボーディング中の表示名が正しいかを検証する。表示名は保存しない。
// 表示名が不正なときは domain.ErrInvalidName、プレイヤーが登録されていなければ port.ErrNotFound を包んだエラーを返す。
func (pi *PlayerInteractor) ValidateNameForOnboarding(ctx context.Context, playerID, name string) error {
	if err := ensurePlayerExists(ctx, pi.playerRepo, playerID); err != nil {
		return err
	}
	return domain.ValidateName(name)
}

// GetBattleLimit はプレイヤーの当日のバトル回数と、1日の上限を返す。
// プレミアム中のプレイヤーの上限は domain.UnlimitedDailyBattleLimit になる。
func (pi *PlayerInteractor) GetBattleLimit(ctx context.Context, playerID string) (*domain.BattleLimit, error) {
	player, err := pi.playerRepo.GetByID(ctx, playerID)
	if err != nil {
		return nil, fmt.Errorf("get player: %w", err)
	}

	count, err := pi.getBattleCount(ctx, playerID, toGameDay(pi.clock.Now()))
	if err != nil {
		return nil, err
	}

	if player.IsPremium {
		return &domain.BattleLimit{
			DailyBattleCount: count,
			DailyBattleLimit: domain.UnlimitedDailyBattleLimit,
			CanBattle:        true,
		}, nil
	}

	freeLimit, err := readGameConfig(ctx, pi.gameConfigRepo, gameConfigKeyFreeDailyBattleLimit)
	if err != nil {
		return nil, err
	}
	return &domain.BattleLimit{
		DailyBattleCount: count,
		DailyBattleLimit: freeLimit,
		CanBattle:        count < freeLimit,
	}, nil
}

// IncrementBattleCount は当日のバトル回数を 1 増やす。
// 無料プレイヤーの当日のバトル回数が上限に達しているときは ErrBattleLimitExceeded を返し、回数を増やさない。
func (pi *PlayerInteractor) IncrementBattleCount(ctx context.Context, playerID string) error {
	player, err := pi.playerRepo.GetByID(ctx, playerID)
	if err != nil {
		return fmt.Errorf("get player: %w", err)
	}
	var freeLimit int64
	if !player.IsPremium {
		freeLimit, err = readGameConfig(ctx, pi.gameConfigRepo, gameConfigKeyFreeDailyBattleLimit)
		if err != nil {
			return err
		}
	}

	gameDay := toGameDay(pi.clock.Now())
	return pi.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		count, err := pi.battleRepo.IncrementDailyBattleCount(txCtx, playerID, gameDay)
		if err != nil {
			return fmt.Errorf("increment daily battle: %w", err)
		}
		if !player.IsPremium && count > freeLimit {
			return ErrBattleLimitExceeded
		}
		return nil
	})
}

// getBattleCount はゲーム日のバトル回数を返す。記録がなければ 0 を返す。
func (pi *PlayerInteractor) getBattleCount(ctx context.Context, playerID string, gameDay civil.Date) (int64, error) {
	dailyBattle, err := pi.battleRepo.GetDailyBattle(ctx, playerID, gameDay)
	if err != nil {
		return 0, fmt.Errorf("get daily battle: %w", err)
	}
	if dailyBattle == nil {
		return 0, nil
	}
	return dailyBattle.DailyBattleCount, nil
}

// RevertBattleCount は、停止で無効になった対戦の消費バトル回数を両プレイヤーに返却する。
// consumedAt はその対戦でバトル回数を消費した時刻で、この時刻のゲーム日の回数から減らす。
// 同じ対戦の返却は1回だけ行い、2回目以降は何もせず成功する。
func (pi *PlayerInteractor) RevertBattleCount(ctx context.Context, gameID string, consumedAt time.Time, player1ID, player2ID string) error {
	return pi.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		isNew, err := pi.battleCountReversalRepo.MarkReverted(txCtx, gameID)
		if err != nil {
			return fmt.Errorf("mark battle count reverted: %w", err)
		}
		if !isNew {
			return nil
		}

		gameDate := toGameDay(consumedAt)
		if err := pi.revertPlayerBattleCount(txCtx, player1ID, gameDate, gameID); err != nil {
			return err
		}
		return pi.revertPlayerBattleCount(txCtx, player2ID, gameDate, gameID)
	})
}

// revertPlayerBattleCount は1人のプレイヤーのバトル回数を1回戻す。回数が 0 回のときは戻さない。
// そのゲーム日の記録がないときは警告ログを残し、成功として扱う。
func (pi *PlayerInteractor) revertPlayerBattleCount(ctx context.Context, playerID string, gameDate civil.Date, gameID string) error {
	dailyBattle, err := pi.battleRepo.GetDailyBattleForUpdate(ctx, playerID, gameDate)
	if err != nil {
		return fmt.Errorf("get daily battle: %w", err)
	}
	if dailyBattle == nil {
		slog.WarnContext(ctx, "battle count revert target not found",
			"player_id", playerID, "game_date", gameDate.String(), "game_id", gameID)
		return nil
	}
	if dailyBattle.DailyBattleCount < minRevertableDailyBattleCount {
		return nil
	}
	if err := pi.battleRepo.DecrementDailyBattleCount(ctx, playerID, gameDate); err != nil {
		return fmt.Errorf("decrement daily battle: %w", err)
	}
	return nil
}

// AwardExp はプレイヤーの累計経験値に経験値を加算する。加算量が 0 のときは何も変えない。
// 加算量が負のときは ErrInvalidExpGain を返す。
func (pi *PlayerInteractor) AwardExp(ctx context.Context, playerID string, expGain int64) error {
	if expGain < 0 {
		return fmt.Errorf("%w: %d", ErrInvalidExpGain, expGain)
	}
	return pi.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		return pi.addExp(txCtx, playerID, expGain)
	})
}

// AwardGameExp は対戦の結果に応じて、両プレイヤーに経験値を付与する。NPC戦では2人目のプレイヤーには付与しない。
// 2人分の付与は1つのトランザクションで行い、どちらかが失敗したときはどちらにも付与しない。
// 勝者番号が不正なときは ErrInvalidWinnerNumber、対戦種別が不正なときは ErrInvalidMatchType、対人戦で2人目のプレイヤーIDが空のときは ErrPlayer2IDRequired を返す。
func (pi *PlayerInteractor) AwardGameExp(
	ctx context.Context,
	player1ID, player2ID string,
	winnerNumber domain.WinnerNumber,
	reason domain.WinReason,
	matchType domain.MatchType,
) error {
	player1Outcome, player2Outcome, err := decideGameOutcomes(winnerNumber, reason, matchType)
	if err != nil {
		return err
	}
	if matchType == domain.MatchTypePvp && player2ID == "" {
		return ErrPlayer2IDRequired
	}
	gains, err := pi.readGameExpGains(ctx)
	if err != nil {
		return err
	}
	player1Gain, err := gains.gainOf(player1Outcome)
	if err != nil {
		return err
	}
	player2Gain, err := gains.gainOf(player2Outcome)
	if err != nil {
		return err
	}

	return pi.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		if err := pi.addExp(txCtx, player1ID, player1Gain); err != nil {
			return err
		}
		if matchType == domain.MatchTypeNpc {
			return nil
		}
		return pi.addExp(txCtx, player2ID, player2Gain)
	})
}

// addExp は、呼び出し元のトランザクションの中で、プレイヤーの累計経験値に経験値を加算する。
// 加算量が 0 のときは何も変えない。
func (pi *PlayerInteractor) addExp(ctx context.Context, playerID string, expGain int64) error {
	if expGain == 0 {
		return nil
	}
	progression, err := pi.progressionRepo.GetProgressionForUpdate(ctx, playerID)
	if err != nil {
		return fmt.Errorf("load progression: %w", err)
	}
	newExp := progression.Exp + expGain
	if err := pi.progressionRepo.UpdateProgression(ctx, playerID, newExp); err != nil {
		return fmt.Errorf("persist progression: %w", err)
	}
	return nil
}

// gameExpGains は対戦の結果ごとの経験値の付与量である。
type gameExpGains struct {
	win  int64
	loss int64
	draw int64
}

// readGameExpGains はゲーム設定から、対戦の結果ごとの経験値の付与量を読む。
func (pi *PlayerInteractor) readGameExpGains(ctx context.Context) (gameExpGains, error) {
	win, err := readGameConfig(ctx, pi.gameConfigRepo, gameConfigKeyExpWin)
	if err != nil {
		return gameExpGains{}, err
	}
	loss, err := readGameConfig(ctx, pi.gameConfigRepo, gameConfigKeyExpLoss)
	if err != nil {
		return gameExpGains{}, err
	}
	draw, err := readGameConfig(ctx, pi.gameConfigRepo, gameConfigKeyExpDraw)
	if err != nil {
		return gameExpGains{}, err
	}
	return gameExpGains{win: win, loss: loss, draw: draw}, nil
}

// gainOf は対戦の結果に対応する経験値の付与量を返す。
func (g gameExpGains) gainOf(outcome gameOutcome) (int64, error) {
	switch outcome {
	case gameOutcomeWin:
		return g.win, nil
	case gameOutcomeLoss:
		return g.loss, nil
	case gameOutcomeDraw:
		return g.draw, nil
	default:
		return 0, fmt.Errorf("unexpected game outcome %q", outcome)
	}
}

// toGameDay は時刻に対応するゲーム日を返す。
func toGameDay(t time.Time) civil.Date {
	return civil.DateOf(t.UTC().Add(gameDayOffset))
}
