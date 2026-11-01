package port

import (
	"context"
	"time"

	"cloud.google.com/go/civil"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

// PlayerRepo はプレイヤーの基本情報の登録・取得・存在確認と、表示名の更新を担う。
type PlayerRepo interface {
	// Create はプレイヤーと、その累計経験値を同時に登録する。
	// 同じ FirebaseUID または同じプレイヤーIDが登録済みのときは ErrAlreadyExists を返す。
	Create(ctx context.Context, player *domain.Player, progression *domain.PlayerProgression) error
	// GetByID はプレイヤーを返す。登録されていなければ ErrNotFound を返す。
	GetByID(ctx context.Context, playerID string) (*domain.Player, error)
	// Exists はプレイヤーが登録されているかを返す。
	Exists(ctx context.Context, playerID string) (bool, error)
	// UpdateName は表示名を更新する。プレイヤーが登録されていなければ ErrNotFound を返す。
	UpdateName(ctx context.Context, playerID string, name string) error
}

// PlayerPremiumRepo はプレイヤーのプレミアム状態の更新と、最後に反映したプレミアム状態更新イベントの発生時刻の排他取得を担う。
type PlayerPremiumRepo interface {
	// UpdatePremium はプレミアム状態と有効期限を更新する。最後に反映したイベントの発生時刻は変えない。
	// プレイヤーが登録されていなければ ErrNotFound を返す。
	UpdatePremium(ctx context.Context, playerID string, isPremium bool, expiresAt *time.Time) error
	// UpdatePremiumFromEvent はプレミアム状態と有効期限を更新し、イベントの発生時刻を最後に反映したイベントの発生時刻として保存する。
	// プレイヤーが登録されていなければ ErrNotFound を返す。
	UpdatePremiumFromEvent(ctx context.Context, playerID string, isPremium bool, expiresAt *time.Time, occurredAt time.Time) error
	// GetLastPremiumEventOccurredAtForUpdate は最後に反映したプレミアム状態更新イベントの発生時刻を排他的に取得する。
	// 別のトランザクションが同じプレイヤーを更新している間は、その確定を待つ。トランザクション内で呼ぶ。
	// イベントをまだ反映していないときは nil を返す。プレイヤーが登録されていなければ ErrNotFound を返す。
	GetLastPremiumEventOccurredAtForUpdate(ctx context.Context, playerID string) (*time.Time, error)
}

// PlayerOnboardingRepo はプレイヤーのオンボーディング進行状態の取得と更新を担う。
type PlayerOnboardingRepo interface {
	// GetOnboardingStatus は進行状態を返す。プレイヤーが登録されていなければ ErrNotFound を返す。
	GetOnboardingStatus(ctx context.Context, playerID string) (string, error)
	// UpdateOnboardingStatus は進行状態を指定した値に更新する。プレイヤーが登録されていなければ ErrNotFound を返す。
	UpdateOnboardingStatus(ctx context.Context, playerID, status string) error
}

// PlayerProgressionRepo はプレイヤーの累計経験値の取得と更新を担う。
type PlayerProgressionRepo interface {
	// GetProgressionForUpdate は累計経験値を排他的に取得する。
	// 別のトランザクションが同じプレイヤーを更新している間は、その確定を待つ。トランザクション内で呼ぶ。
	// プレイヤーが登録されていなければ ErrNotFound を返す。
	GetProgressionForUpdate(ctx context.Context, playerID string) (*domain.PlayerProgression, error)
	// UpdateProgression は累計経験値を指定した値に更新する。
	// プレイヤーが登録されていなければ ErrNotFound を返す。
	UpdateProgression(ctx context.Context, playerID string, exp int64) error
}

// PlayerBattleRepo はプレイヤーのゲーム日ごとのバトル回数を担う。
type PlayerBattleRepo interface {
	// GetDailyBattle はゲーム日のバトル回数を返す。記録がなければ nil を返す。
	GetDailyBattle(ctx context.Context, playerID string, gameDate civil.Date) (*domain.PlayerDailyBattle, error)
	// IncrementDailyBattleCount はゲーム日のバトル回数を 1 増やし、増やした後の回数を返す。
	IncrementDailyBattleCount(ctx context.Context, playerID string, gameDate civil.Date) (int64, error)
	// GetDailyBattleForUpdate はゲーム日のバトル回数を排他的に取得する。記録がなければ nil を返す。
	// 別のトランザクションが同じ記録を更新している間は、その確定を待つ。トランザクション内で呼ぶ。
	GetDailyBattleForUpdate(ctx context.Context, playerID string, gameDate civil.Date) (*domain.PlayerDailyBattle, error)
	// DecrementDailyBattleCount はゲーム日のバトル回数を 1 減らす。記録がなければ ErrNotFound を返す。
	DecrementDailyBattleCount(ctx context.Context, playerID string, gameDate civil.Date) error
}

// BattleCountReversalRepo は対戦ごとのバトル回数の返却を記録する。
type BattleCountReversalRepo interface {
	// MarkReverted は対戦の返却を記録する。新しく記録できたときは true、記録済みのときは false を返す。
	MarkReverted(ctx context.Context, gameID string) (bool, error)
}

// PlayerViewRepo はプレイヤー情報の組み立てに使う参照用の情報を返す。
type PlayerViewRepo interface {
	// GetByID はプレイヤーの参照用の情報を返す。登録されていなければ ErrNotFound を返す。
	GetByID(ctx context.Context, playerID string) (*domain.PlayerView, error)
	// GetByFirebaseUID は FirebaseUID に対応するプレイヤーの参照用の情報を返す。登録されていなければ ErrNotFound を返す。
	GetByFirebaseUID(ctx context.Context, firebaseUID string) (*domain.PlayerView, error)
}

// PlayerSettingsPatch はプレイヤー設定の更新内容である。nil の項目は変更しない。
type PlayerSettingsPatch struct {
	Language      *string
	BgmVolume     *int64
	SeVolume      *int64
	IsPushEnabled *bool
}

// IsEmpty は更新する項目が1つも指定されていないかを返す。
func (p *PlayerSettingsPatch) IsEmpty() bool {
	return p.Language == nil && p.BgmVolume == nil && p.SeVolume == nil && p.IsPushEnabled == nil
}

// PlayerSettingsRepo はプレイヤー設定の登録・取得・更新を担う。
type PlayerSettingsRepo interface {
	// InsertPlayerSettings はプレイヤー設定を登録する。
	InsertPlayerSettings(ctx context.Context, settings *domain.PlayerSettings) error
	// GetPlayerSettings はプレイヤー設定を返す。登録されていなければ ErrNotFound を返す。
	GetPlayerSettings(ctx context.Context, playerID string) (*domain.PlayerSettings, error)
	// UpdatePlayerSettings は更新内容で指定された項目だけを更新する。登録されていなければ ErrNotFound を返す。
	UpdatePlayerSettings(ctx context.Context, playerID string, patch *PlayerSettingsPatch) error
}

// GameConfigRepo はゲーム設定の読み取りを担う。
type GameConfigRepo interface {
	// GetInt64 はキーに対応するゲーム設定の整数値を返す。
	// 設定がなければ ErrNotFound、値が整数として読めなければ ErrValueNotInteger を返す。
	GetInt64(ctx context.Context, key string) (int64, error)
}

// FactionRepo はプレイヤーが所持する陣営を担う。
type FactionRepo interface {
	// AddPlayerFaction は陣営を所持陣営に加える。初期陣営にはしない。所持済みのときは何も変えない。
	// プレイヤーが登録されていなければ ErrNotFound を返す。
	AddPlayerFaction(ctx context.Context, playerID, faction string) error
	// GetPlayerFactions は所持陣営の一覧を返す。順序は決まっていない。所持陣営がなければ空の一覧を返す。
	GetPlayerFactions(ctx context.Context, playerID string) ([]string, error)
	// GetInitialFaction は初期陣営を返す。選択されていなければ nil を返す。
	GetInitialFaction(ctx context.Context, playerID string) (*string, error)
	// SetInitialFaction は陣営を初期陣営にし、所持陣営にも加える。
	// 初期陣営がすでにあるときは、同じ陣営でも ErrAlreadyExists を返す。
	SetInitialFaction(ctx context.Context, playerID, faction string) error
}

// ProcessedEventRepo は処理済みの Pub/Sub イベントを記録する。
type ProcessedEventRepo interface {
	// InsertProcessedEvent はイベントIDを処理済みとして記録する。新しく記録できたときは true、記録済みのときは false を返す。
	InsertProcessedEvent(ctx context.Context, eventID, eventType string) (bool, error)
}

// TxRunner は処理を1つのトランザクションで実行する。
type TxRunner interface {
	// RunInTx は fn を1つのトランザクションで実行する。fn がエラーを返したときは fn の書き込みをすべて取り消し、そのエラーを返す。
	// 呼び出しが取り消されていないのにトランザクションを開始できないときは、ErrDatabaseUnavailable を包んだエラーを返す。
	RunInTx(ctx context.Context, fn func(ctx context.Context) error) error
}
