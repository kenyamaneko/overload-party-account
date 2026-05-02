package port

import (
	"context"
	"time"

	"cloud.google.com/go/civil"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

// PlayerRepo は account.players を中心とした Write 集約を抽象化する。
// 複数テーブルの JOIN を伴う参照系は PlayerViewRepo に分離する (CQRS)。
type PlayerRepo interface {
	// Create は players / player_progression を同一トランザクションで初期化する。
	// player_daily_battle はゲーム日ごとに発生する履歴台帳のため Create では INSERT しない。
	Create(ctx context.Context, player *domain.Player, progression *domain.PlayerProgression) error
	// FindByID は players 行を返す。Level/Exp は含まない。行が無ければ ErrNotFound。
	FindByID(ctx context.Context, playerID string) (*domain.Player, error)
	// FindByFirebaseUID は firebase_uid で検索する。該当なしは ErrNotFound。
	FindByFirebaseUID(ctx context.Context, firebaseUID string) (*domain.Player, error)
	// Exists は player_id に対応する行の存在のみを確認する。
	Exists(ctx context.Context, playerID string) (bool, error)
	// GetDailyBattle は (player_id, gameDate) の行を返す。該当なしは (nil, nil)。
	GetDailyBattle(ctx context.Context, playerID string, gameDate civil.Date) (*domain.PlayerDailyBattle, error)
	// GetProgression は account.player_progression の現在値を返す。行が無ければ ErrNotFound。
	GetProgression(ctx context.Context, playerID string) (*domain.PlayerProgression, error)
	// GetProgressionForUpdate は SELECT ... FOR UPDATE で行ロックを取得して返す。
	// 呼び出し側が TxRunner.RunInTx 配下で使う責務を負う。行が無ければ ErrNotFound。
	GetProgressionForUpdate(ctx context.Context, playerID string) (*domain.PlayerProgression, error)
	// GetOnboardingStatus は onboarding_status を返す。行が無ければ ErrNotFound。
	GetOnboardingStatus(ctx context.Context, playerID string) (string, error)
	// UpdateName は name のみを更新する。行が無ければ ErrNotFound。
	UpdateName(ctx context.Context, playerID string, name string) error
	// UpdatePremium は is_premium と premium_expires_at を更新する。
	UpdatePremium(ctx context.Context, playerID string, isPremium bool, expiresAt *time.Time) error
	// IncrementDailyBattleCount は (player_id, gameDate) のカウントを 1 加算した結果を返す。
	IncrementDailyBattleCount(ctx context.Context, playerID string, gameDate civil.Date) (int64, error)
	// UpdateProgression は exp と level をそのまま書き込む。行が無ければ ErrNotFound。
	UpdateProgression(ctx context.Context, playerID string, exp, level int64) (*domain.PlayerProgression, error)
	// UpdateOnboardingStatus は onboarding_status をそのまま書き込む。行が無ければ ErrNotFound。
	UpdateOnboardingStatus(ctx context.Context, playerID, status string) error
}

// PlayerViewRepo はプレイヤー情報の Read Model を組み立てる参照専用リポジトリ。
type PlayerViewRepo interface {
	// FindByID は player_id に対応する Read Model を返す。行が無ければ ErrNotFound。
	FindByID(ctx context.Context, playerID string) (*domain.PlayerView, error)
	// FindByFirebaseUID は firebase_uid に対応する Read Model を返す。行が無ければ ErrNotFound。
	FindByFirebaseUID(ctx context.Context, firebaseUID string) (*domain.PlayerView, error)
}

// PlayerSettingsPatch は player_settings の部分更新リクエストを表す。
// nil フィールドは「変更なし」を意味し、repo 層の COALESCE で現在値が保持される。
type PlayerSettingsPatch struct {
	Language    *string
	BgmVolume   *int64
	SeVolume    *int64
	PushEnabled *bool
}

// IsEmpty は 1 つも指定フィールドがない patch (全 nil) かを返す。
func (p *PlayerSettingsPatch) IsEmpty() bool {
	return p.Language == nil && p.BgmVolume == nil && p.SeVolume == nil && p.PushEnabled == nil
}

// PlayerSettingsRepo はプレイヤー設定の永続化を抽象化する。
type PlayerSettingsRepo interface {
	// Insert は新規行を挿入する。Register 時の初期化に使用する。
	Insert(ctx context.Context, s *domain.PlayerSettings) error
	// Get はプレイヤー設定を返す。該当なしは ErrNotFound。
	Get(ctx context.Context, playerID string) (*domain.PlayerSettings, error)
	// UpdatePartial は patch で指定された非 nil フィールドのみを更新する (COALESCE 方式)。
	// 行が無ければ ErrNotFound。
	UpdatePartial(ctx context.Context, playerID string, patch *PlayerSettingsPatch) error
}

// GameConfigRepo はゲーム設定値の読み取りを抽象化する。キーが無ければ ErrNotFound (fail-fast)。
type GameConfigRepo interface {
	GetInt64(ctx context.Context, key string) (int64, error)
}

// FactionRepo はプレイヤーファクションの永続化を抽象化する。
type FactionRepo interface {
	// AddPlayerFaction は player_factions に is_initial=FALSE で 1 行追加する。
	// (player_id, faction) PK の ON CONFLICT DO NOTHING で冪等。
	AddPlayerFaction(ctx context.Context, playerID, faction string) error
	// GetPlayerFactions は所持ファクション名の一覧を返す (is_initial 区別なし)。
	GetPlayerFactions(ctx context.Context, playerID string) ([]string, error)
	// GetInitialFaction はプレイヤーの initial faction を返す。未選択なら (nil, nil)。
	GetInitialFaction(ctx context.Context, playerID string) (*string, error)
	// SetInitialFaction は (player_id, faction) を is_initial=TRUE で INSERT する。
	// PK 重複や partial unique index 違反は DB エラーとしてそのまま返す。
	SetInitialFaction(ctx context.Context, playerID, faction string) error
}

// ProcessedEventRepo は処理済み Pub/Sub イベントを追跡する。
// subscriber は Tx 冒頭で event_id を Insert し、新規挿入の真偽で重複配信を判別する。
type ProcessedEventRepo interface {
	// Insert は新規行が挿入された場合 true を返す。
	Insert(ctx context.Context, eventID, eventType string) (bool, error)
}

// TxRunner はトランザクション内で処理を実行する。
type TxRunner interface {
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}
