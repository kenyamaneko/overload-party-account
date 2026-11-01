package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/civil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

var (
	_ port.PlayerRepo            = (*PlayerRepository)(nil)
	_ port.PlayerPremiumRepo     = (*PlayerRepository)(nil)
	_ port.PlayerOnboardingRepo  = (*PlayerRepository)(nil)
	_ port.PlayerProgressionRepo = (*PlayerRepository)(nil)
	_ port.PlayerBattleRepo      = (*PlayerRepository)(nil)
)

const playerSelectColumns = `player_id, firebase_uid, name, is_premium, equipped_icon_no,
	        onboarding_status, premium_expires_at, created_at, updated_at`

const dailyBattleQuery = `SELECT player_id, game_date, daily_battle_count
	   FROM account.player_daily_battle
	  WHERE player_id = $1 AND game_date = $2`

// PlayerRepository はプレイヤーに関する永続化の PostgreSQL 実装である。
type PlayerRepository struct {
	pool *pgxpool.Pool
}

// NewPlayerRepository は PlayerRepository を生成する。
func NewPlayerRepository(pool *pgxpool.Pool) *PlayerRepository {
	return &PlayerRepository{pool: pool}
}

// Create はプレイヤーと、その累計経験値を同時に登録する。
// 同じ FirebaseUID または同じプレイヤーIDが登録済みのときは port.ErrAlreadyExists を返す。
func (r *PlayerRepository) Create(ctx context.Context, player *domain.Player, progression *domain.PlayerProgression) error {
	if findTx(ctx) != nil {
		return r.insertPlayerAndProgression(ctx, player, progression)
	}
	return NewTxManager(r.pool).RunInTx(ctx, func(txCtx context.Context) error {
		return r.insertPlayerAndProgression(txCtx, player, progression)
	})
}

func (r *PlayerRepository) insertPlayerAndProgression(ctx context.Context, player *domain.Player, progression *domain.PlayerProgression) error {
	db := resolveDBTX(ctx, r.pool)
	_, err := db.Exec(ctx,
		`INSERT INTO account.players (player_id, firebase_uid, name, is_premium, equipped_icon_no, onboarding_status, premium_expires_at, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		player.PlayerID,
		player.FirebaseUID,
		player.Name,
		player.IsPremium,
		player.EquippedIconNo,
		player.OnboardingStatus,
		player.PremiumExpiresAt,
		player.CreatedAt,
		player.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return fmt.Errorf("insert player: %w: %w", port.ErrAlreadyExists, err)
		}
		return fmt.Errorf("insert player: %w", err)
	}

	_, err = db.Exec(ctx,
		`INSERT INTO account.player_progression (player_id, exp, updated_at)
		 VALUES ($1,$2,$3)`,
		progression.PlayerID,
		progression.Exp,
		progression.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert player progression: %w", err)
	}

	return nil
}

// GetByID はプレイヤーを返す。登録されていなければ port.ErrNotFound を返す。
func (r *PlayerRepository) GetByID(ctx context.Context, playerID string) (*domain.Player, error) {
	row := resolveDBTX(ctx, r.pool).QueryRow(ctx,
		`SELECT `+playerSelectColumns+`
		   FROM account.players
		  WHERE player_id = $1`,
		playerID,
	)

	p, err := scanPlayer(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("player %s: %w", playerID, port.ErrNotFound)
		}
		return nil, fmt.Errorf("get player by id: %w", err)
	}
	return p, nil
}

// Exists はプレイヤーが登録されているかを返す。
func (r *PlayerRepository) Exists(ctx context.Context, playerID string) (bool, error) {
	var isFound bool
	if err := resolveDBTX(ctx, r.pool).QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM account.players WHERE player_id = $1)`,
		playerID,
	).Scan(&isFound); err != nil {
		return false, fmt.Errorf("check player exists: %w", err)
	}
	return isFound, nil
}

// GetDailyBattle はゲーム日のバトル回数を返す。記録がなければ nil を返す。
func (r *PlayerRepository) GetDailyBattle(ctx context.Context, playerID string, gameDate civil.Date) (*domain.PlayerDailyBattle, error) {
	return r.queryDailyBattle(ctx, playerID, gameDate, dailyBattleQuery)
}

// GetDailyBattleForUpdate はゲーム日のバトル回数を排他的に取得する。記録がなければ nil を返す。
// 別のトランザクションが同じ記録を更新している間は、その確定を待つ。
func (r *PlayerRepository) GetDailyBattleForUpdate(ctx context.Context, playerID string, gameDate civil.Date) (*domain.PlayerDailyBattle, error) {
	return r.queryDailyBattle(ctx, playerID, gameDate, dailyBattleQuery+` FOR UPDATE`)
}

func (r *PlayerRepository) queryDailyBattle(ctx context.Context, playerID string, gameDate civil.Date, query string) (*domain.PlayerDailyBattle, error) {
	row := resolveDBTX(ctx, r.pool).QueryRow(ctx, query, playerID, toTime(gameDate))

	dailyBattle, err := scanDailyBattle(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get daily battle: %w", err)
	}
	return dailyBattle, nil
}

// GetProgressionForUpdate は累計経験値を排他的に取得する。
// 別のトランザクションが同じプレイヤーを更新している間は、その確定を待つ。
func (r *PlayerRepository) GetProgressionForUpdate(ctx context.Context, playerID string) (*domain.PlayerProgression, error) {
	row := resolveDBTX(ctx, r.pool).QueryRow(ctx,
		`SELECT player_id, exp, updated_at
		 FROM account.player_progression WHERE player_id = $1 FOR UPDATE`,
		playerID,
	)
	progression, err := scanProgression(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("progression for player %s: %w", playerID, port.ErrNotFound)
		}
		return nil, fmt.Errorf("get progression for update: %w", err)
	}
	return progression, nil
}

// GetOnboardingStatus はオンボーディングの進行状態を返す。プレイヤーが登録されていなければ port.ErrNotFound を返す。
func (r *PlayerRepository) GetOnboardingStatus(ctx context.Context, playerID string) (string, error) {
	var status string
	err := resolveDBTX(ctx, r.pool).QueryRow(ctx,
		`SELECT onboarding_status FROM account.players WHERE player_id = $1`,
		playerID,
	).Scan(&status)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", fmt.Errorf("player %s: %w", playerID, port.ErrNotFound)
		}
		return "", fmt.Errorf("get onboarding status: %w", err)
	}
	return status, nil
}

// UpdateName は表示名を更新する。プレイヤーが登録されていなければ port.ErrNotFound を返す。
func (r *PlayerRepository) UpdateName(ctx context.Context, playerID string, name string) error {
	commandTag, err := resolveDBTX(ctx, r.pool).Exec(ctx,
		`UPDATE account.players SET name = $1 WHERE player_id = $2`,
		name, playerID,
	)
	if err != nil {
		return fmt.Errorf("update name: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("player %s: %w", playerID, port.ErrNotFound)
	}
	return nil
}

// UpdatePremium はプレミアム状態と有効期限を更新する。最後に反映したイベントの発生時刻は変えない。
// プレイヤーが登録されていなければ port.ErrNotFound を返す。
func (r *PlayerRepository) UpdatePremium(ctx context.Context, playerID string, isPremium bool, expiresAt *time.Time) error {
	commandTag, err := resolveDBTX(ctx, r.pool).Exec(ctx,
		`UPDATE account.players SET is_premium = $1, premium_expires_at = $2
		 WHERE player_id = $3`,
		isPremium, expiresAt, playerID,
	)
	if err != nil {
		return fmt.Errorf("update player premium: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("player %s: %w", playerID, port.ErrNotFound)
	}
	return nil
}

// UpdatePremiumFromEvent はプレミアム状態と有効期限を更新し、イベントの発生時刻を最後に反映したイベントの発生時刻として保存する。
// プレイヤーが登録されていなければ port.ErrNotFound を返す。
func (r *PlayerRepository) UpdatePremiumFromEvent(ctx context.Context, playerID string, isPremium bool, expiresAt *time.Time, occurredAt time.Time) error {
	commandTag, err := resolveDBTX(ctx, r.pool).Exec(ctx,
		`UPDATE account.players
		    SET is_premium = $1, premium_expires_at = $2, last_premium_event_occurred_at = $3
		  WHERE player_id = $4`,
		isPremium, expiresAt, occurredAt, playerID,
	)
	if err != nil {
		return fmt.Errorf("update player premium from event: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("player %s: %w", playerID, port.ErrNotFound)
	}
	return nil
}

// GetLastPremiumEventOccurredAtForUpdate は最後に反映したプレミアム状態更新イベントの発生時刻を排他的に取得する。
// 別のトランザクションが同じプレイヤーを更新している間は、その確定を待つ。
// イベントをまだ反映していないときは nil を返す。プレイヤーが登録されていなければ port.ErrNotFound を返す。
func (r *PlayerRepository) GetLastPremiumEventOccurredAtForUpdate(ctx context.Context, playerID string) (*time.Time, error) {
	var occurredAt *time.Time
	if err := resolveDBTX(ctx, r.pool).QueryRow(ctx,
		`SELECT last_premium_event_occurred_at
		   FROM account.players
		  WHERE player_id = $1
		    FOR UPDATE`,
		playerID,
	).Scan(&occurredAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("player %s: %w", playerID, port.ErrNotFound)
		}
		return nil, fmt.Errorf("get last premium event occurred at: %w", err)
	}
	return occurredAt, nil
}

// IncrementDailyBattleCount はゲーム日のバトル回数を 1 増やし、増やした後の回数を返す。
func (r *PlayerRepository) IncrementDailyBattleCount(ctx context.Context, playerID string, gameDate civil.Date) (int64, error) {
	var newCount int64
	err := resolveDBTX(ctx, r.pool).QueryRow(ctx,
		`INSERT INTO account.player_daily_battle (player_id, game_date, daily_battle_count)
		 VALUES ($1, $2, 1)
		 ON CONFLICT (player_id, game_date)
		 DO UPDATE SET daily_battle_count = account.player_daily_battle.daily_battle_count + 1
		 RETURNING daily_battle_count`,
		playerID, toTime(gameDate),
	).Scan(&newCount)
	if err != nil {
		return 0, fmt.Errorf("increment daily battle: %w", err)
	}
	return newCount, nil
}

// DecrementDailyBattleCount はゲーム日のバトル回数を 1 減らす。記録がなければ port.ErrNotFound を返す。
func (r *PlayerRepository) DecrementDailyBattleCount(ctx context.Context, playerID string, gameDate civil.Date) error {
	commandTag, err := resolveDBTX(ctx, r.pool).Exec(ctx,
		`UPDATE account.player_daily_battle
		 SET daily_battle_count = daily_battle_count - 1
		 WHERE player_id = $1 AND game_date = $2`,
		playerID, toTime(gameDate),
	)
	if err != nil {
		return fmt.Errorf("decrement daily battle: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("daily battle of player %s on %s: %w", playerID, gameDate, port.ErrNotFound)
	}
	return nil
}

// UpdateProgression は累計経験値を指定した値に更新する。
// プレイヤーが登録されていなければ port.ErrNotFound を返す。
func (r *PlayerRepository) UpdateProgression(ctx context.Context, playerID string, exp int64) error {
	commandTag, err := resolveDBTX(ctx, r.pool).Exec(ctx,
		`UPDATE account.player_progression SET exp = $2
		 WHERE player_id = $1`,
		playerID, exp,
	)
	if err != nil {
		return fmt.Errorf("update progression: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("progression for player %s: %w", playerID, port.ErrNotFound)
	}
	return nil
}

// UpdateOnboardingStatus はオンボーディングの進行状態を指定した値に更新する。
// プレイヤーが登録されていなければ port.ErrNotFound を返す。
func (r *PlayerRepository) UpdateOnboardingStatus(ctx context.Context, playerID, status string) error {
	commandTag, err := resolveDBTX(ctx, r.pool).Exec(ctx,
		`UPDATE account.players SET onboarding_status = $1 WHERE player_id = $2`,
		status, playerID,
	)
	if err != nil {
		return fmt.Errorf("update onboarding status: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return fmt.Errorf("player %s: %w", playerID, port.ErrNotFound)
	}
	return nil
}

func scanPlayer(row pgx.Row) (*domain.Player, error) {
	var p domain.Player
	err := row.Scan(
		&p.PlayerID,
		&p.FirebaseUID,
		&p.Name,
		&p.IsPremium,
		&p.EquippedIconNo,
		&p.OnboardingStatus,
		&p.PremiumExpiresAt,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func scanProgression(row pgx.Row) (*domain.PlayerProgression, error) {
	var p domain.PlayerProgression
	err := row.Scan(&p.PlayerID, &p.Exp, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func scanDailyBattle(row pgx.Row) (*domain.PlayerDailyBattle, error) {
	var dailyBattle domain.PlayerDailyBattle
	var gameDate time.Time
	err := row.Scan(
		&dailyBattle.PlayerID,
		&gameDate,
		&dailyBattle.DailyBattleCount,
	)
	if err != nil {
		return nil, err
	}
	dailyBattle.GameDate = toCivilDate(gameDate)
	return &dailyBattle, nil
}

func toTime(d civil.Date) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

func toCivilDate(t time.Time) civil.Date {
	return civil.DateOf(t)
}
