package port

import (
	"context"
	"time"

	"cloud.google.com/go/civil"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// PlayerRepo はプレイヤーデータの永続化を抽象化するインターフェースです。
//
// account.players / account.player_daily_battle / account.player_progression の
// 3 テーブルを 1 つのアグリゲートとして扱います。呼び出し元は物理テーブル分割を知らずに
// 「プレイヤー」として操作できます。FindByID 等は JOIN で Level / Exp も詰めて返します。
type PlayerRepo interface {
	// Create は players / player_daily_battle / player_progression を同一トランザクションで初期化します。
	Create(ctx context.Context, player *apiaccount.Player, dailyBattle *apiaccount.PlayerDailyBattle, progression *apiaccount.PlayerProgression) error
	FindByID(ctx context.Context, playerID string) (*apiaccount.Player, error)
	FindByFirebaseUID(ctx context.Context, firebaseUID string) (*apiaccount.Player, error)
	GetDailyBattle(ctx context.Context, playerID string) (*apiaccount.PlayerDailyBattle, error)
	// UpdateDailyBattleCount は count と last_reset_date を書き込む純粋なプリミティブです。
	// 日付リセットや上限判定などのドメインルールは service 層で解決してから呼び出します。
	UpdateDailyBattleCount(ctx context.Context, playerID string, count int64, resetDate civil.Date) error
	// UpdateName は name のみを更新する純プリミティブです。
	// 行が存在しない場合は ErrNotFound を返します。
	UpdateName(ctx context.Context, playerID string, name string) error
	UpdatePremium(ctx context.Context, playerID string, isPremium bool, expiresAt *time.Time) error
	UpdateFaction(ctx context.Context, playerID, faction string) error
	// SetSelectedFactionIfNull は selected_faction が NULL の場合のみ faction を書き込みます。
	// 戻り値は「行が更新されたか」のみで、未更新の理由 (既に選択済み / プレイヤー不在) は
	// 識別しません。識別が必要な場合は呼び出し元が Exists と組み合わせて判断します。
	SetSelectedFactionIfNull(ctx context.Context, playerID, faction string) (set bool, err error)
	// Exists は player_id に対応する行の存在のみを確認する純プリミティブです。
	Exists(ctx context.Context, playerID string) (bool, error)
	// GetProgression は account.player_progression の現在値を返します。行が無ければ ErrNotFound。
	GetProgression(ctx context.Context, playerID string) (*apiaccount.PlayerProgression, error)
	// GetProgressionForUpdate は SELECT ... FOR UPDATE で行ロックを取得して progression を返します。
	// FOR UPDATE はトランザクション内でのみ意味を持つため、呼び出し側が TxRunner.RunInTx 配下で
	// 使う責務を負います。行が無ければ ErrNotFound。
	GetProgressionForUpdate(ctx context.Context, playerID string) (*apiaccount.PlayerProgression, error)
	// UpdateProgression は exp と level をそのまま書き込む純プリミティブです。
	// 加算・レベル計算は service 層の責務で、repo は受け取った値を反映するだけです。
	// 行が無ければ ErrNotFound。
	UpdateProgression(ctx context.Context, playerID string, exp, level int64) (*apiaccount.PlayerProgression, error)
	// ApplyOnboardingNameSet は name と onboarding_status を 1 SQL で更新する純プリミティブです。
	// onboarding_status は state machine 順序で前進する場合のみ書き換え、後進方向の遷移は
	// 黙ってスキップします (out-of-order 配信に対する防御)。name は無条件に書き換えます。
	// 行が無ければ ErrNotFound。
	ApplyOnboardingNameSet(ctx context.Context, playerID, name string) error
	// ApplyOnboardingFactionSet は selected_faction と onboarding_status を 1 SQL で更新する純プリミティブです。
	// onboarding_status は state machine 順序で前進する場合のみ書き換え、後進方向の遷移は
	// 黙ってスキップします。行が無ければ ErrNotFound。
	ApplyOnboardingFactionSet(ctx context.Context, playerID, faction string) error
	// ApplyOnboardingCompleted は onboarding_status を 'completed' に書き換える純プリミティブです。
	// completed は terminal 状態のため state machine 順序チェックは不要 (常に前進)。
	// 行が無ければ ErrNotFound。
	ApplyOnboardingCompleted(ctx context.Context, playerID string) error
}

// PlayerSettingsPatch は player_settings の部分更新リクエストを表します。
// nil フィールドは「変更なし（現状維持）」を意味し、repo 層の COALESCE で現在値が保持されます。
// JSON DTO と layer を切り離すため、port 層のプレーン構造体として定義します。
type PlayerSettingsPatch struct {
	Language    *string
	BgmVolume   *int64
	SeVolume    *int64
	PushEnabled *bool
}

// IsEmpty は 1 つも指定フィールドがない patch（全 nil）かを返します。
// handler 層で全 nil 送信を 400 として弾くために使います。
func (p *PlayerSettingsPatch) IsEmpty() bool {
	return p.Language == nil && p.BgmVolume == nil && p.SeVolume == nil && p.PushEnabled == nil
}

// PlayerSettingsRepo はプレイヤー設定の永続化を抽象化するインターフェースです。
type PlayerSettingsRepo interface {
	Get(ctx context.Context, playerID string) (*apiaccount.PlayerSettings, error)
	// Insert は新規行を挿入します。全フィールド必須で、Register 時の初期化に使用します。
	Insert(ctx context.Context, s *apiaccount.PlayerSettings) error
	// UpdatePartial は patch で指定された非 nil フィールドのみを更新します（COALESCE 方式）。
	// 行が存在しない場合は ErrNotFound を返します（通常 Register 時に Insert 済みの前提）。
	UpdatePartial(ctx context.Context, playerID string, patch *PlayerSettingsPatch) error
}

// GameConfigRepo はゲーム設定値の読み取りを抽象化するインターフェースです。
// キーが存在しない場合は ErrNotFound を返す（fail-fast）。
type GameConfigRepo interface {
	GetInt64(ctx context.Context, key string) (int64, error)
}

// FactionRepo はプレイヤーファクションの永続化を抽象化するインターフェースです。
type FactionRepo interface {
	AddPlayerFaction(ctx context.Context, playerID, faction, source string) error
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
