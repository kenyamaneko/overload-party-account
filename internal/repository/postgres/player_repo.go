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
// account.players / account.player_daily_battle / account.player_progression の 3 テーブルを
// 「プレイヤー」という 1 つのアグリゲートとして扱い、呼び出し元には物理分割を見せない。
type PlayerRepository struct {
	pool *pgxpool.Pool
}

// NewPlayerRepository は PlayerRepository を生成する。
func NewPlayerRepository(pool *pgxpool.Pool) *PlayerRepository {
	return &PlayerRepository{pool: pool}
}

// selectPlayerColumnsJoin は Player アグリゲートを JOIN で組み立てる SELECT 句。
// scanPlayer の順序と揃えている。
const selectPlayerColumnsJoin = `p.player_id, p.firebase_uid, p.name, pp.level, pp.exp,
		p.is_premium, p.equipped_icon_no, p.selected_faction,
		p.premium_expires_at, p.created_at, p.updated_at`

// Create は players / player_daily_battle / player_progression をアトミックに挿入する。
// context にトランザクションがあればそれに参加し、なければ独自トランザクションを使用する。
func (r *PlayerRepository) Create(ctx context.Context, player *apiaccount.Player, dailyBattle *apiaccount.PlayerDailyBattle, progression *apiaccount.PlayerProgression) error {
	if txFromContext(ctx) != nil {
		return r.createInner(ctx, connFrom(ctx, r.pool), player, dailyBattle, progression)
	}
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := r.createInner(ctx, tx, player, dailyBattle, progression); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *PlayerRepository) createInner(ctx context.Context, db dbtx, player *apiaccount.Player, dailyBattle *apiaccount.PlayerDailyBattle, progression *apiaccount.PlayerProgression) error {
	_, err := db.Exec(ctx,
		`INSERT INTO account.players (player_id, firebase_uid, name, is_premium, equipped_icon_no, selected_faction, premium_expires_at, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		player.PlayerID,
		player.FirebaseUID,
		player.Name,
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

	_, err = db.Exec(ctx,
		`INSERT INTO account.player_daily_battle (player_id, daily_battle_count, last_reset_date)
		 VALUES ($1,$2,$3)`,
		dailyBattle.PlayerID,
		dailyBattle.DailyBattleCount,
		civilDateToTime(dailyBattle.LastResetDate),
	)
	if err != nil {
		return fmt.Errorf("insert daily battle: %w", err)
	}

	return nil
}

// FindByID はプレイヤー ID で検索する。該当なしは port.ErrNotFound でラップして返す。
func (r *PlayerRepository) FindByID(ctx context.Context, playerID string) (*apiaccount.Player, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT `+selectPlayerColumnsJoin+`
		 FROM account.players p
		 JOIN account.player_progression pp ON p.player_id = pp.player_id
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

// FindByFirebaseUID は Firebase UID で検索する。該当なしは (nil, nil) を返す。
func (r *PlayerRepository) FindByFirebaseUID(ctx context.Context, firebaseUID string) (*apiaccount.Player, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT `+selectPlayerColumnsJoin+`
		 FROM account.players p
		 JOIN account.player_progression pp ON p.player_id = pp.player_id
		 WHERE p.firebase_uid = $1 LIMIT 1`,
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

// GetDailyBattle はプレイヤーの日次バトルデータを返す。該当なしは (nil, nil) を返す。
func (r *PlayerRepository) GetDailyBattle(ctx context.Context, playerID string) (*apiaccount.PlayerDailyBattle, error) {
	row := connFrom(ctx, r.pool).QueryRow(ctx,
		`SELECT player_id, daily_battle_count, last_reset_date
		 FROM account.player_daily_battle WHERE player_id = $1`,
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

// UpdateDailyBattleCount は count と last_reset_date を書き込むプリミティブ。
// 日付リセット判定や上限チェックは service 層の責務で、ここでは受け取った値をそのまま反映する。
// 行が存在しなければ port.ErrNotFound を返す（player_daily_battle は players と 1:1 で
// Create 時に INSERT される前提）。
func (r *PlayerRepository) UpdateDailyBattleCount(ctx context.Context, playerID string, count int64, resetDate civil.Date) error {
	ct, err := connFrom(ctx, r.pool).Exec(ctx,
		`UPDATE account.player_daily_battle SET daily_battle_count = $1, last_reset_date = $2 WHERE player_id = $3`,
		count, civilDateToTime(resetDate), playerID,
	)
	if err != nil {
		return fmt.Errorf("update daily battle: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return fmt.Errorf("daily battle for player %s: %w", playerID, port.ErrNotFound)
	}
	return nil
}

// UpdateName はプレイヤー名を更新する純プリミティブ。
// 更新後のアグリゲート再取得は service 層が FindByID で行う責務とし、
// repo は単一テーブルへの書き込みだけを担う。行が無ければ port.ErrNotFound を返す。
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

// UpdateFaction は選択ファクションを更新する。
func (r *PlayerRepository) UpdateFaction(ctx context.Context, playerID, faction string) error {
	_, err := connFrom(ctx, r.pool).Exec(ctx,
		`UPDATE account.players SET selected_faction = $1, updated_at = $2
		 WHERE player_id = $3`,
		faction, time.Now(), playerID,
	)
	if err != nil {
		return fmt.Errorf("update player faction: %w", err)
	}
	return nil
}

// SetSelectedFactionIfNull は selected_faction が NULL の行に対してのみ UPDATE する。
// 戻り値は「行が更新されたか」のみで、未更新の理由 (既に選択済み / プレイヤー不在) の
// 識別は行わない。識別は呼び出し元 (service) が Exists と組み合わせて判断する。
func (r *PlayerRepository) SetSelectedFactionIfNull(ctx context.Context, playerID, faction string) (bool, error) {
	ct, err := connFrom(ctx, r.pool).Exec(ctx,
		`UPDATE account.players SET selected_faction = $1, updated_at = $2
		 WHERE player_id = $3 AND selected_faction IS NULL`,
		faction, time.Now(), playerID,
	)
	if err != nil {
		return false, fmt.Errorf("set selected faction if null: %w", err)
	}
	return ct.RowsAffected() == 1, nil
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
