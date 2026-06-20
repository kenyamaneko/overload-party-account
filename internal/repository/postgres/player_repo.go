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

// 単一の PlayerRepository が責務別 interface 群すべてを暗黙的に満たすことを
// コンパイル時に保証する (Go の structural typing による多 interface 充足)。
var (
	_ port.PlayerRepo            = (*PlayerRepository)(nil)
	_ port.PlayerPremiumRepo     = (*PlayerRepository)(nil)
	_ port.PlayerOnboardingRepo  = (*PlayerRepository)(nil)
	_ port.PlayerProgressionRepo = (*PlayerRepository)(nil)
	_ port.PlayerBattleRepo      = (*PlayerRepository)(nil)
)

// PlayerRepository は port.PlayerRepo の PostgreSQL 実装。
type PlayerRepository struct {
	pool *pgxpool.Pool
}

// NewPlayerRepository は PlayerRepository を生成する。
func NewPlayerRepository(pool *pgxpool.Pool) *PlayerRepository {
	return &PlayerRepository{pool: pool}
}

// Create は players / player_progression をアトミックに挿入する。
func (r *PlayerRepository) Create(ctx context.Context, player *domain.Player, progression *domain.PlayerProgression) error {
	if txFromContext(ctx) != nil {
		return r.createInner(ctx, connFrom(ctx, r.pool), player, progression)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.createInner(ctx, tx, player, progression); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PlayerRepository) createInner(ctx context.Context, db dbtx, player *domain.Player, progression *domain.PlayerProgression) error {
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
		return fmt.Errorf("insert player: %w", err)
	}

	_, err = db.Exec(ctx,
		`INSERT INTO account.player_progression (player_id, level, exp)
		 VALUES ($1,$2,$3)`,
		progression.PlayerID,
		progression.Level,
		progression.Exp,
	)
	if err != nil {
		return fmt.Errorf("insert player progression: %w", err)
	}

	return nil
}

// FindByID は account.players の 1 行を返す。該当なしは port.ErrNotFound。
func (r *PlayerRepository) FindByID(ctx context.Context, playerID string) (*domain.Player, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT player_id, firebase_uid, name, is_premium, equipped_icon_no,
		        onboarding_status, premium_expires_at, created_at, updated_at
		   FROM account.players
		  WHERE player_id = $1`,
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

// FindByFirebaseUID は firebase_uid で account.players の 1 行を返す。該当なしは port.ErrNotFound。
func (r *PlayerRepository) FindByFirebaseUID(ctx context.Context, firebaseUID string) (*domain.Player, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT player_id, firebase_uid, name, is_premium, equipped_icon_no,
		        onboarding_status, premium_expires_at, created_at, updated_at
		   FROM account.players
		  WHERE firebase_uid = $1 LIMIT 1`,
		firebaseUID,
	)

	p, err := scanPlayer(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, port.ErrNotFound
		}
		return nil, fmt.Errorf("find player by firebase_uid: %w", err)
	}
	return p, nil
}

// Exists は player_id に対応する行の存在のみを確認する。
func (r *PlayerRepository) Exists(ctx context.Context, playerID string) (bool, error) {
	var isFound bool
	if err := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM account.players WHERE player_id = $1)`,
		playerID,
	).Scan(&isFound); err != nil {
		return false, fmt.Errorf("check player exists: %w", err)
	}
	return isFound, nil
}

// ExistsByFirebaseUID は firebase_uid に対応する行の存在のみを確認する。
func (r *PlayerRepository) ExistsByFirebaseUID(ctx context.Context, firebaseUID string) (bool, error) {
	var isFound bool
	if err := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM account.players WHERE firebase_uid = $1)`,
		firebaseUID,
	).Scan(&isFound); err != nil {
		return false, fmt.Errorf("check player exists by firebase_uid: %w", err)
	}
	return isFound, nil
}

// GetDailyBattle は (player_id, gameDate) の行を返す。該当なしは (nil, nil)。
func (r *PlayerRepository) GetDailyBattle(ctx context.Context, playerID string, gameDate civil.Date) (*domain.PlayerDailyBattle, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT player_id, game_date, daily_battle_count
		 FROM account.player_daily_battle WHERE player_id = $1 AND game_date = $2`,
		playerID, civilDateToTime(gameDate),
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

// GetProgression は player_progression の現在値を返す。該当なしは port.ErrNotFound。
func (r *PlayerRepository) GetProgression(ctx context.Context, playerID string) (*domain.PlayerProgression, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT player_id, level, exp, updated_at
		 FROM account.player_progression WHERE player_id = $1`,
		playerID,
	)

	prog, err := scanProgression(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("progression for player %s: %w", playerID, port.ErrNotFound)
		}
		return nil, fmt.Errorf("get player progression: %w", err)
	}
	return prog, nil
}

// GetProgressionForUpdate は SELECT ... FOR UPDATE で player_progression の行ロックを取得する。
// FOR UPDATE はトランザクション寿命に依存するため、呼び出し側が TxRunner.RunInTx 配下で使う責務を持つ。
func (r *PlayerRepository) GetProgressionForUpdate(ctx context.Context, playerID string) (*domain.PlayerProgression, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT player_id, level, exp, updated_at
		 FROM account.player_progression WHERE player_id = $1 FOR UPDATE`,
		playerID,
	)
	prog, err := scanProgression(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("progression for player %s: %w", playerID, port.ErrNotFound)
		}
		return nil, fmt.Errorf("get progression for update: %w", err)
	}
	return prog, nil
}

// GetOnboardingStatus は onboarding_status を返す。
func (r *PlayerRepository) GetOnboardingStatus(ctx context.Context, playerID string) (string, error) {
	var status string
	err := connFrom(ctx, r.pool).QueryRow(ctx,
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

// UpdateName はプレイヤー名を更新する。行が無ければ port.ErrNotFound。
func (r *PlayerRepository) UpdateName(ctx context.Context, playerID string, name string) error {
	ct, err := connFrom(ctx, r.pool).Exec(ctx,
		`UPDATE account.players SET name = $1, updated_at = NOW() WHERE player_id = $2`,
		name, playerID,
	)
	if err != nil {
		return fmt.Errorf("update name: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("player %s: %w", playerID, port.ErrNotFound)
	}
	return nil
}

// UpdatePremium はプレミアムステータスを更新する。行が無ければ port.ErrNotFound。
func (r *PlayerRepository) UpdatePremium(ctx context.Context, playerID string, isPremium bool, expiresAt *time.Time) error {
	ct, err := connFrom(ctx, r.pool).Exec(ctx,
		`UPDATE account.players SET is_premium = $1, premium_expires_at = $2, updated_at = $3
		 WHERE player_id = $4`,
		isPremium, expiresAt, time.Now(), playerID,
	)
	if err != nil {
		return fmt.Errorf("update player premium: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("player %s: %w", playerID, port.ErrNotFound)
	}
	return nil
}

// IncrementDailyBattleCount は (player_id, gameDate) のカウントを 1 加算した結果を返す。
// INSERT ... ON CONFLICT DO UPDATE の単発 SQL なので加算自体は原子的。
func (r *PlayerRepository) IncrementDailyBattleCount(ctx context.Context, playerID string, gameDate civil.Date) (int64, error) {
	var newCount int64
	err := connFrom(ctx, r.pool).QueryRow(ctx,
		`INSERT INTO account.player_daily_battle (player_id, game_date, daily_battle_count)
		 VALUES ($1, $2, 1)
		 ON CONFLICT (player_id, game_date)
		 DO UPDATE SET daily_battle_count = account.player_daily_battle.daily_battle_count + 1
		 RETURNING daily_battle_count`,
		playerID, civilDateToTime(gameDate),
	).Scan(&newCount)
	if err != nil {
		return 0, fmt.Errorf("increment daily battle: %w", err)
	}
	return newCount, nil
}

// UpdateProgression は exp / level をそのまま書き込む。行が無ければ port.ErrNotFound。
func (r *PlayerRepository) UpdateProgression(ctx context.Context, playerID string, exp, level int64) (*domain.PlayerProgression, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`UPDATE account.player_progression SET exp = $2, level = $3
		 WHERE player_id = $1
		 RETURNING player_id, level, exp, updated_at`,
		playerID, exp, level,
	)
	prog, err := scanProgression(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("progression for player %s: %w", playerID, port.ErrNotFound)
		}
		return nil, fmt.Errorf("update progression: %w", err)
	}
	return prog, nil
}

// UpdateOnboardingStatus は onboarding_status をそのまま書き込む。行が無ければ port.ErrNotFound。
func (r *PlayerRepository) UpdateOnboardingStatus(ctx context.Context, playerID, status string) error {
	ct, err := connFrom(ctx, r.pool).Exec(ctx,
		`UPDATE account.players SET onboarding_status = $1, updated_at = NOW() WHERE player_id = $2`,
		status, playerID,
	)
	if err != nil {
		return fmt.Errorf("update onboarding status: %w", err)
	}
	if ct.RowsAffected() == 0 {
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
	err := row.Scan(&p.PlayerID, &p.Level, &p.Exp, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func scanDailyBattle(row pgx.Row) (*domain.PlayerDailyBattle, error) {
	var db domain.PlayerDailyBattle
	var gameDateTime time.Time
	err := row.Scan(
		&db.PlayerID,
		&gameDateTime,
		&db.DailyBattleCount,
	)
	if err != nil {
		return nil, err
	}
	db.GameDate = timeToCivilDate(gameDateTime)
	return &db, nil
}

func civilDateToTime(d civil.Date) time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

func timeToCivilDate(t time.Time) civil.Date {
	return civil.DateOf(t)
}
