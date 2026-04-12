package port

import (
	"context"
	"time"

	"cloud.google.com/go/civil"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// PlayerRepo はプレイヤーデータの永続化を抽象化するインターフェースです。
type PlayerRepo interface {
	Create(ctx context.Context, player *apiaccount.Player, dailyBattle *apiaccount.PlayerDailyBattle) error
	FindByID(ctx context.Context, playerID string) (*apiaccount.Player, error)
	FindByFirebaseUID(ctx context.Context, firebaseUID string) (*apiaccount.Player, error)
	GetDailyBattle(ctx context.Context, playerID string) (*apiaccount.PlayerDailyBattle, error)
	IncrementDailyBattle(ctx context.Context, playerID string, today civil.Date) (int64, error)
	UpdateUsername(ctx context.Context, playerID string, username string) (*apiaccount.Player, error)
	UpdatePremium(ctx context.Context, playerID string, isPremium bool, expiresAt *time.Time) error
	UpdateFaction(ctx context.Context, playerID, faction string) error
	AddExp(ctx context.Context, playerID string, expGain int64, computeLevel func(newExp, currentLevel int64) int64) (*apiaccount.Player, error)
}

// UserSettingsRepo はユーザー設定の永続化を抽象化するインターフェースです。
type UserSettingsRepo interface {
	Get(ctx context.Context, playerID string) (*apiaccount.UserSettings, error)
	Upsert(ctx context.Context, s *apiaccount.UserSettings) error
}

// GameConfigRepo はゲーム設定値の読み取りを抽象化するインターフェースです。
type GameConfigRepo interface {
	GetInt64(ctx context.Context, key string, fallback int64) (int64, error)
}

// FactionRepo はプレイヤーファクションの永続化を抽象化するインターフェースです。
type FactionRepo interface {
	AddPlayerFaction(ctx context.Context, playerID, faction, source string) error
	// InsertInitial は複合 PK の ON CONFLICT DO NOTHING で player_factions 行を挿入します。
	// 新規挿入なら created=true を返し、初期選択フローの冪等性シグナルとして使用します。
	InsertInitial(ctx context.Context, playerID, faction, source string) (created bool, err error)
	GetPlayerFactions(ctx context.Context, playerID string) ([]string, error)
}

// ProcessedEventRepo は処理済み Pub/Sub イベントを追跡するインターフェースです。
// subscriber はトランザクション冒頭で event_id を INSERT し、重複キーなら処理済みと判断します。
type ProcessedEventRepo interface {
	// Insert は新規行が挿入された場合 true を返します（冪等性ガード）。
	Insert(ctx context.Context, eventID, eventType string) (bool, error)
}

// TxRunner はトランザクション内で処理を実行するインターフェースです。
type TxRunner interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}
