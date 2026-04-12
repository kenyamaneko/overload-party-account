package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/civil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
	"github.com/kenyamaneko/overload-party-account/internal/port"
)

var _ port.PlayerRepo = (*PgPlayerRepository)(nil)

// PgPlayerRepository は PostgreSQL を使用した PlayerRepo の実装です。
type PgPlayerRepository struct {
	pool *pgxpool.Pool
}

// NewPgPlayerRepository は PgPlayerRepository を生成します。
func NewPgPlayerRepository(pool *pgxpool.Pool) *PgPlayerRepository {
	return &PgPlayerRepository{pool: pool}
}

// Create は players と player_daily_battle をアトミックに挿入します。
// context にトランザクションがあればそれに参加し、なければ独自トランザクションを使用します。
func (r *PgPlayerRepository) Create(ctx context.Context, player *apiaccount.Player, dailyBattle *apiaccount.PlayerDailyBattle) error {
	if txFromContext(ctx) != nil {
		return r.createInner(ctx, connFrom(ctx, r.pool), player, dailyBattle)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.createInner(ctx, tx, player, dailyBattle); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PgPlayerRepository) createInner(ctx context.Context, db dbtx, player *apiaccount.Player, dailyBattle *apiaccount.PlayerDailyBattle) error {
	_, err := db.Exec(ctx,
		`INSERT INTO players (player_id, firebase_uid, username, level, exp, is_premium, equipped_icon_no, selected_faction, premium_expires_at, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		player.PlayerID,
		player.FirebaseUID,
		player.Username,
		player.Level,
		player.Exp,
		player.IsPremium,
		player.EquippedIconNo,
		player.SelectedFaction,
		player.PremiumExpiresAt,
		player.CreatedAt,
		player.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert player: %w", err)
	}

	lastResetTime := civilDateToTime(dailyBattle.LastResetDate)
	_, err = db.Exec(ctx,
		`INSERT INTO player_daily_battle (player_id, daily_battle_count, last_reset_date)
		 VALUES ($1,$2,$3)`,
		dailyBattle.PlayerID,
		dailyBattle.DailyBattleCount,
		lastResetTime,
	)
	if err != nil {
		return fmt.Errorf("insert daily battle: %w", err)
	}

	return nil
}

// FindByID はプレイヤー ID で検索します。
func (r *PgPlayerRepository) FindByID(ctx context.Context, playerID string) (*apiaccount.Player, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT player_id, firebase_uid, username, level, exp, is_premium, equipped_icon_no, selected_faction, premium_expires_at, created_at, updated_at
		 FROM players WHERE player_id = $1`,
		playerID,
	)

	p, err := scanPlayer(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("player %s: %w", playerID, port.ErrNotFound)
		}
		return nil, fmt.Errorf("find player by id: %w", err)
	}
	return p, nil
}

// FindByFirebaseUID は Firebase UID で検索します。該当なしは (nil, nil) を返します。
func (r *PgPlayerRepository) FindByFirebaseUID(ctx context.Context, firebaseUID string) (*apiaccount.Player, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT player_id, firebase_uid, username, level, exp, is_premium, equipped_icon_no, selected_faction, premium_expires_at, created_at, updated_at
		 FROM players WHERE firebase_uid = $1 LIMIT 1`,
		firebaseUID,
	)

	p, err := scanPlayer(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find player by firebase_uid: %w", err)
	}
	return p, nil
}

// GetDailyBattle はプレイヤーの日次バトルデータを返します。該当なしは (nil, nil) を返します。
func (r *PgPlayerRepository) GetDailyBattle(ctx context.Context, playerID string) (*apiaccount.PlayerDailyBattle, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT player_id, daily_battle_count, last_reset_date
		 FROM player_daily_battle WHERE player_id = $1`,
		playerID,
	)

	db, err := scanDailyBattle(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get daily battle: %w", err)
	}
	return db, nil
}

// IncrementDailyBattle は日次バトル回数をアトミックにインクリメントします。
// last_reset_date が今日以前ならカウントをリセットします。新しいカウントを返します。
func (r *PgPlayerRepository) IncrementDailyBattle(ctx context.Context, playerID string, today civil.Date) (int64, error) {
	if txFromContext(ctx) != nil {
		return r.incrementDailyBattleInner(ctx, connFrom(ctx, r.pool), playerID, today)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	count, err := r.incrementDailyBattleInner(ctx, tx, playerID, today)
	if err != nil {
		return 0, err
	}
	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit tx: %w", err)
	}
	return count, nil
}

func (r *PgPlayerRepository) incrementDailyBattleInner(ctx context.Context, db dbtx, playerID string, today civil.Date) (int64, error) {
	row := db.QueryRow(ctx,
		`SELECT daily_battle_count, last_reset_date
		 FROM player_daily_battle WHERE player_id = $1 FOR UPDATE`,
		playerID,
	)

	var count int64
	var lastResetTime time.Time
	if err := row.Scan(&count, &lastResetTime); err != nil {
		return 0, fmt.Errorf("read daily battle: %w", err)
	}

	lastReset := timeToCivilDate(lastResetTime)
	if lastReset != today {
		count = 1
	} else {
		count++
	}

	todayTime := civilDateToTime(today)
	_, err := db.Exec(ctx,
		`UPDATE player_daily_battle SET daily_battle_count = $1, last_reset_date = $2 WHERE player_id = $3`,
		count, todayTime, playerID,
	)
	if err != nil {
		return 0, fmt.Errorf("update daily battle: %w", err)
	}

	return count, nil
}

// UpdateUsername はプレイヤー名を更新し、更新後のプレイヤーを返します。
func (r *PgPlayerRepository) UpdateUsername(ctx context.Context, playerID string, username string) (*apiaccount.Player, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`UPDATE players SET username = $1, updated_at = NOW()
		 WHERE player_id = $2
		 RETURNING player_id, firebase_uid, username, level, exp, is_premium, equipped_icon_no, selected_faction, premium_expires_at, created_at, updated_at`,
		username, playerID,
	)

	p, err := scanPlayer(row)
	if err != nil {
		return nil, fmt.Errorf("update username: %w", err)
	}
	return p, nil
}

// UpdatePremium はプレミアムステータスを更新します。
func (r *PgPlayerRepository) UpdatePremium(ctx context.Context, playerID string, isPremium bool, expiresAt *time.Time) error {
	db := connFrom(ctx, r.pool)
	_, err := db.Exec(ctx,
		`UPDATE players SET is_premium = $1, premium_expires_at = $2, updated_at = $3
		 WHERE player_id = $4`,
		isPremium, expiresAt, time.Now(), playerID,
	)
	if err != nil {
		return fmt.Errorf("update player premium: %w", err)
	}
	return nil
}

// UpdateFaction は選択ファクションを更新します。
func (r *PgPlayerRepository) UpdateFaction(ctx context.Context, playerID, faction string) error {
	_, err := connFrom(ctx, r.pool).Exec(ctx,
		`UPDATE players SET selected_faction = $1, updated_at = $2
		 WHERE player_id = $3`,
		faction, time.Now(), playerID,
	)
	if err != nil {
		return fmt.Errorf("update player faction: %w", err)
	}
	return nil
}

// AddExp は SELECT FOR UPDATE で経験値をアトミックに加算しレベルを再計算します。
// computeLevel はサービス層が提供するレベル計算関数です。
func (r *PgPlayerRepository) AddExp(ctx context.Context, playerID string, expGain int64, computeLevel func(newExp, currentLevel int64) int64) (*apiaccount.Player, error) {
	if txFromContext(ctx) != nil {
		return r.addExpInner(ctx, connFrom(ctx, r.pool), playerID, expGain, computeLevel)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("add exp begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	p, err := r.addExpInner(ctx, tx, playerID, expGain, computeLevel)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("add exp commit: %w", err)
	}
	return p, nil
}

func (r *PgPlayerRepository) addExpInner(ctx context.Context, db dbtx, playerID string, expGain int64, computeLevel func(newExp, currentLevel int64) int64) (*apiaccount.Player, error) {
	var curExp, curLevel int64
	err := db.QueryRow(ctx,
		`SELECT exp, level FROM players WHERE player_id = $1 FOR UPDATE`,
		playerID,
	).Scan(&curExp, &curLevel)
	if err != nil {
		return nil, fmt.Errorf("add exp select: %w", err)
	}

	newExp := curExp + expGain
	newLevel := computeLevel(newExp, curLevel)

	row := db.QueryRow(ctx,
		`UPDATE players SET exp = $2, level = $3, updated_at = NOW()
		 WHERE player_id = $1
		 RETURNING player_id, firebase_uid, username, level, exp, is_premium, equipped_icon_no, selected_faction, premium_expires_at, created_at, updated_at`,
		playerID, newExp, newLevel,
	)

	p, err := scanPlayer(row)
	if err != nil {
		return nil, fmt.Errorf("add exp update: %w", err)
	}
	return p, nil
}

func scanPlayer(row pgx.Row) (*apiaccount.Player, error) {
	var p apiaccount.Player
	err := row.Scan(
		&p.PlayerID,
		&p.FirebaseUID,
		&p.Username,
		&p.Level,
		&p.Exp,
		&p.IsPremium,
		&p.EquippedIconNo,
		&p.SelectedFaction,
		&p.PremiumExpiresAt,
		&p.CreatedAt,
		&p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// PostgreSQL の DATE カラムは time.Time としてスキャンし civil.Date に変換する。
func scanDailyBattle(row pgx.Row) (*apiaccount.PlayerDailyBattle, error) {
	var db apiaccount.PlayerDailyBattle
	var lastResetTime time.Time
	err := row.Scan(
		&db.PlayerID,
		&db.DailyBattleCount,
		&lastResetTime,
	)
	if err != nil {
		return nil, err
	}
	db.LastResetDate = timeToCivilDate(lastResetTime)
	return &db, nil
}

func civilDateToTime(d civil.Date) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

func timeToCivilDate(t time.Time) civil.Date {
	return civil.DateOf(t)
}
