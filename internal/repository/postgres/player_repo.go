package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/civil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

var _ port.PlayerRepo = (*PlayerRepository)(nil)

// PlayerRepository は PostgreSQL を使用した PlayerRepo の実装である。
// account.players + 1:1 子の account.player_progression を「プレイヤー」アグリゲートとして
// JOIN で結合し、呼び出し元には物理分割を見せない。
// account.player_daily_battle はゲーム日単位の履歴台帳で、専用プリミティブで操作する。
type PlayerRepository struct {
	pool *pgxpool.Pool
}

// NewPlayerRepository は PlayerRepository を生成する。
func NewPlayerRepository(pool *pgxpool.Pool) *PlayerRepository {
	return &PlayerRepository{pool: pool}
}

// Create は players / player_progression をアトミックに挿入する。
// player_daily_battle はゲーム日ごとに発生する履歴行のため Create では作らない
// (初回バトルの IncrementDailyBattleCount UPSERT で当日の行が発生する)。
// context にトランザクションがあればそれに参加し、なければ独自トランザクションを使用する。
func (r *PlayerRepository) Create(ctx context.Context, player *apiaccount.Player, progression *apiaccount.PlayerProgression) error {
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

func (r *PlayerRepository) createInner(ctx context.Context, db dbtx, player *apiaccount.Player, progression *apiaccount.PlayerProgression) error {
	_, err := db.Exec(ctx,
		`INSERT INTO account.players (player_id, firebase_uid, name, is_premium, equipped_icon_no, premium_expires_at, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		player.PlayerID,
		player.FirebaseUID,
		player.Name,
		player.IsPremium,
		player.EquippedIconNo,
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

// FindByID はプレイヤー ID で検索する。該当なしは port.ErrNotFound でラップして返す。
func (r *PlayerRepository) FindByID(ctx context.Context, playerID string) (*apiaccount.Player, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT p.player_id, p.firebase_uid, p.name, pp.level, pp.exp,
		        p.is_premium, p.equipped_icon_no, pf.faction, p.onboarding_status,
		        p.premium_expires_at, p.created_at, p.updated_at
		   FROM account.players p
		   JOIN account.player_progression pp ON p.player_id = pp.player_id
		   LEFT JOIN account.player_factions pf ON pf.player_id = p.player_id AND pf.is_initial
		  WHERE p.player_id = $1`,
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

// FindByFirebaseUID は Firebase UID で検索する。該当なしは port.ErrNotFound でラップして返す。
// 業務分岐 (Register での既登録検出など) は呼び出し側で errors.Is(err, port.ErrNotFound) を判定する。
func (r *PlayerRepository) FindByFirebaseUID(ctx context.Context, firebaseUID string) (*apiaccount.Player, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT p.player_id, p.firebase_uid, p.name, pp.level, pp.exp,
		        p.is_premium, p.equipped_icon_no, pf.faction, p.onboarding_status,
		        p.premium_expires_at, p.created_at, p.updated_at
		   FROM account.players p
		   JOIN account.player_progression pp ON p.player_id = pp.player_id
		   LEFT JOIN account.player_factions pf ON pf.player_id = p.player_id AND pf.is_initial
		  WHERE p.firebase_uid = $1 LIMIT 1`,
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

// Exists は player_id に対応する行の存在のみを確認する純プリミティブ。
func (r *PlayerRepository) Exists(ctx context.Context, playerID string) (bool, error) {
	var exists bool
	if err := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM account.players WHERE player_id = $1)`,
		playerID,
	).Scan(&exists); err != nil {
		return false, fmt.Errorf("check player exists: %w", err)
	}
	return exists, nil
}

// GetDailyBattle は (player_id, gameDate) の行を返す。該当なしは (nil, nil)。
// 「指定ゲーム日にまだバトルしていない」を nil で表現する。
func (r *PlayerRepository) GetDailyBattle(ctx context.Context, playerID string, gameDate civil.Date) (*apiaccount.PlayerDailyBattle, error) {
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

// GetProgression は player_progression の現在値を返す。該当なしは port.ErrNotFound でラップ。
func (r *PlayerRepository) GetProgression(ctx context.Context, playerID string) (*apiaccount.PlayerProgression, error) {
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

// GetProgressionForUpdate は player_progression に SELECT ... FOR UPDATE で行ロックを取得する。
// FOR UPDATE の効果はトランザクション寿命に依存するため、呼び出し側が TxRunner.RunInTx 配下で
// 呼ぶ責務を持つ（pool 直接呼び出しでは autocommit で直ちにロックが解放される）。
func (r *PlayerRepository) GetProgressionForUpdate(ctx context.Context, playerID string) (*apiaccount.PlayerProgression, error) {
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

// GetOnboardingStatus は onboarding_status を返す純プリミティブ。
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

// UpdateName はプレイヤー名を更新する純プリミティブ。
// 行が無ければ port.ErrNotFound を返す。
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

// UpdatePremium はプレミアムステータスを更新する。
func (r *PlayerRepository) UpdatePremium(ctx context.Context, playerID string, isPremium bool, expiresAt *time.Time) error {
	db := connFrom(ctx, r.pool)
	_, err := db.Exec(ctx,
		`UPDATE account.players SET is_premium = $1, premium_expires_at = $2, updated_at = $3
		 WHERE player_id = $4`,
		isPremium, expiresAt, time.Now(), playerID,
	)
	if err != nil {
		return fmt.Errorf("update player premium: %w", err)
	}
	return nil
}

// IncrementDailyBattleCount は (player_id, gameDate) のカウントを 1 加算した結果を返す。
// INSERT ... ON CONFLICT DO UPDATE の単発 SQL なので加算自体は原子的。上限判定は呼び出し側で
// 加算後カウントとリミットを比較する責務 (リポジトリ層は不変条件を持たないプリミティブ)。
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

// UpdateProgression は exp / level をそのまま書き込む純プリミティブ。
// 加算やレベル計算は service 層の責務で、ここでは受け取った値を反映するだけ。
func (r *PlayerRepository) UpdateProgression(ctx context.Context, playerID string, exp, level int64) (*apiaccount.PlayerProgression, error) {
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

// UpdateOnboardingStatus は onboarding_status をそのまま書き込む純プリミティブ。
// state machine 順序の判定は service 層が事前に行う責務。
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

func scanPlayer(row pgx.Row) (*apiaccount.Player, error) {
	var p apiaccount.Player
	err := row.Scan(
		&p.PlayerID,
		&p.FirebaseUID,
		&p.Name,
		&p.Level,
		&p.Exp,
		&p.IsPremium,
		&p.EquippedIconNo,
		&p.InitialFaction,
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

func scanProgression(row pgx.Row) (*apiaccount.PlayerProgression, error) {
	var p apiaccount.PlayerProgression
	err := row.Scan(&p.PlayerID, &p.Level, &p.Exp, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// PostgreSQL の DATE カラムは time.Time としてスキャンし civil.Date に変換する。
func scanDailyBattle(row pgx.Row) (*apiaccount.PlayerDailyBattle, error) {
	var db apiaccount.PlayerDailyBattle
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
