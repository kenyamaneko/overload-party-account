package port

import (
	"context"
	"time"

	"cloud.google.com/go/civil"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

// PlayerRepo は account.players を中心とした Write 集約を抽象化するインターフェースです。
//
// 厳密に Write 用集約だけを扱い、複数テーブルの JOIN 結果が必要な参照系は
// PlayerViewRepo に分離します。Level/Exp/InitialFaction の取得には
// PlayerViewRepo か PlayerProgressionRepo / FactionRepo を使ってください。
type PlayerRepo interface {
	// Create は players / player_progression を同一トランザクションで初期化します。
	// player_daily_battle はゲーム日ごとに行が発生する履歴台帳のため Create では INSERT しません
	// (初回バトルの IncrementDailyBattleCount で UPSERT されます)。
	Create(ctx context.Context, player *domain.Player, progression *domain.PlayerProgression) error
	// FindByID は players 行のみを返します。Level/Exp は含みません。
	// 行が無ければ ErrNotFound。
	FindByID(ctx context.Context, playerID string) (*domain.Player, error)
	// FindByFirebaseUID は firebase_uid で検索する。該当なしは ErrNotFound。
	// 業務分岐 (Register の既登録検出など) は呼び出し側で errors.Is で判定する。
	FindByFirebaseUID(ctx context.Context, firebaseUID string) (*domain.Player, error)
	// Exists は player_id に対応する行の存在のみを確認する純プリミティブです。
	Exists(ctx context.Context, playerID string) (bool, error)
	// GetDailyBattle は (player_id, gameDate) の行を返します。該当なしは (nil, nil)。
	// 「指定ゲーム日にまだバトルしていない」を nil で表現するため、呼び出し側はカウント 0 として扱います。
	GetDailyBattle(ctx context.Context, playerID string, gameDate civil.Date) (*domain.PlayerDailyBattle, error)
	// GetProgression は account.player_progression の現在値を返します。行が無ければ ErrNotFound。
	GetProgression(ctx context.Context, playerID string) (*domain.PlayerProgression, error)
	// GetProgressionForUpdate は SELECT ... FOR UPDATE で行ロックを取得して progression を返します。
	// FOR UPDATE はトランザクション内でのみ意味を持つため、呼び出し側が TxRunner.RunInTx 配下で
	// 使う責務を負います。行が無ければ ErrNotFound。
	GetProgressionForUpdate(ctx context.Context, playerID string) (*domain.PlayerProgression, error)
	// GetOnboardingStatus は onboarding_status を返します。行が無ければ ErrNotFound。
	GetOnboardingStatus(ctx context.Context, playerID string) (string, error)
	// UpdateName は name のみを更新する純プリミティブです。
	// 行が存在しない場合は ErrNotFound を返します。
	UpdateName(ctx context.Context, playerID string, name string) error
	UpdatePremium(ctx context.Context, playerID string, isPremium bool, expiresAt *time.Time) error
	// IncrementDailyBattleCount は (player_id, gameDate) のカウントを 1 加算した結果を返します。
	// INSERT ... ON CONFLICT DO UPDATE の単発 SQL なので、加算自体は原子的です
	// (Get → Update を直列に呼ぶ実装と比べてカウントが飛んだり重複したりしません)。
	IncrementDailyBattleCount(ctx context.Context, playerID string, gameDate civil.Date) (int64, error)
	// UpdateProgression は exp と level をそのまま書き込む純プリミティブです。
	// 加算・レベル計算は usecase 層の責務で、repo は受け取った値を反映するだけです。
	// 行が無ければ ErrNotFound。
	UpdateProgression(ctx context.Context, playerID string, exp, level int64) (*domain.PlayerProgression, error)
	// UpdateOnboardingStatus は onboarding_status をそのまま書き込む純プリミティブです。
	// state machine 順序の判定は usecase 層 (domain.CanTransitionOnboardingStatus) の責務。
	// 行が無ければ ErrNotFound。
	UpdateOnboardingStatus(ctx context.Context, playerID, status string) error
}

// PlayerViewRepo はプレイヤー情報の Read Model (PlayerView) を組み立てる
// 参照専用リポジトリです。Write 用集約 (PlayerRepo) と物理的に同じ
// 永続化層に当たりますが、CQRS の Q 側として interface を分離します。
type PlayerViewRepo interface {
	// FindByID は player_id に対応する Read Model を返します。行が無ければ ErrNotFound。
	FindByID(ctx context.Context, playerID string) (*domain.PlayerView, error)
	// FindByFirebaseUID は firebase_uid に対応する Read Model を返します。行が無ければ ErrNotFound。
	FindByFirebaseUID(ctx context.Context, firebaseUID string) (*domain.PlayerView, error)
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
	// Insert は新規行を挿入します。全フィールド必須で、Register 時の初期化に使用します。
	Insert(ctx context.Context, s *domain.PlayerSettings) error
	// Get はプレイヤー設定を返します。該当なしは ErrNotFound。
	Get(ctx context.Context, playerID string) (*domain.PlayerSettings, error)
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
	// AddPlayerFaction は player_factions に is_initial=FALSE で 1 行追加します
	// (ショップ購入経路など、オンボーディング選択以外の取得用)。
	// (player_id, faction) 複合 PK の ON CONFLICT DO NOTHING で冪等。
	AddPlayerFaction(ctx context.Context, playerID, faction string) error
	// GetPlayerFactions は所持ファクション名の一覧を返します (is_initial 区別なし)。
	GetPlayerFactions(ctx context.Context, playerID string) ([]string, error)
	// GetInitialFaction はプレイヤーの initial faction (is_initial=TRUE) を返します。
	// 未選択なら (nil, nil)。
	GetInitialFaction(ctx context.Context, playerID string) (*string, error)
	// SetInitialFaction は (player_id, faction) を is_initial=TRUE で INSERT します。
	// PK 重複や partial unique index 違反は DB エラーとしてそのまま返します。
	SetInitialFaction(ctx context.Context, playerID, faction string) error
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
