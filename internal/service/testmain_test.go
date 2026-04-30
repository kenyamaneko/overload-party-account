//go:build integration

package service

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/civil"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres/postgrestest"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// sharedPg はパッケージ全体で共有する Testcontainers PostgreSQL ハンドル。
// コンテナ起動コストを償却するため、各テストは sharedPg.Truncate(t) で
// 状態をリセットして同じインスタンスを再利用する。
var sharedPg *postgrestest.Postgres

func TestMain(m *testing.M) {
	os.Exit(postgrestest.RunMain(m, &sharedPg,
		postgrestest.WithSchemaFile("db/schema.sql"),
		postgrestest.WithSchema("account"),
	))
}

// fakeGameConfigRepo は port.GameConfigRepo の最小 fake 実装。
// service テストで Firestore を起こさないために使用する。
type fakeGameConfigRepo struct {
	values map[string]int64
}

var _ port.GameConfigRepo = (*fakeGameConfigRepo)(nil)

func newFakeGameConfigRepo(values map[string]int64) *fakeGameConfigRepo {
	v := make(map[string]int64, len(values))
	for k, val := range values {
		v[k] = val
	}
	return &fakeGameConfigRepo{values: v}
}

// GetInt64 はキーが存在しない場合 port.ErrNotFound でラップしたエラーを返す。
// 握りつぶさず fail-fast するため production の Firestore 実装と同じ契約。
func (f *fakeGameConfigRepo) GetInt64(_ context.Context, key string) (int64, error) {
	if v, ok := f.values[key]; ok {
		return v, nil
	}
	return 0, fmt.Errorf("game config %q: %w", key, port.ErrNotFound)
}

// seedPlayer は account.players + player_progression の最小シードを投入する。
// postgres 層テストの helpers_test.go と同じパターン。
// player_daily_battle はゲーム日単位の履歴台帳になったため seedPlayer では作らない
// (バトル発生時の IncrementDailyBattleCount で UPSERT される)。
// 引数 name に "" を渡すと account.players.name を NULL として挿入する
// (オンボーディング前の name 未確定状態を再現するため)。
func seedPlayer(t *testing.T, playerID, firebaseUID, name string, isPremium bool) *apiaccount.Player {
	t.Helper()
	now := time.Now().UTC()
	var namePtr *string
	if name != "" {
		namePtr = &name
	}
	p := &apiaccount.Player{
		PlayerID:    playerID,
		FirebaseUID: firebaseUID,
		Name:        namePtr,
		Level:       1,
		Exp:         0,
		IsPremium:   isPremium,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	ctx := context.Background()
	_, err := sharedPg.Pool.Exec(ctx,
		`INSERT INTO account.players (player_id, firebase_uid, name, is_premium, equipped_icon_no, premium_expires_at, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		p.PlayerID, p.FirebaseUID, p.Name, p.IsPremium,
		p.EquippedIconNo, p.PremiumExpiresAt, p.CreatedAt, p.UpdatedAt)
	require.NoError(t, err)

	_, err = sharedPg.Pool.Exec(ctx,
		`INSERT INTO account.player_progression (player_id, level, exp) VALUES ($1,$2,$3)`,
		p.PlayerID, p.Level, p.Exp)
	require.NoError(t, err)

	return p
}

// seedPlayerWithState は指定の level/exp 状態でシードし、必要なら指定ゲーム日に
// player_daily_battle を直接 INSERT する。
// GetBattleLimit / AwardExp のテスト用。level/exp は player_progression テーブルに入る。
// dailyCount < 0 のときは player_daily_battle に行を作らない (新ゲーム日でまだバトルしていない状態を再現する)。
// 引数 name に "" を渡すと account.players.name を NULL として挿入する。
func seedPlayerWithState(t *testing.T, playerID, firebaseUID, name string, isPremium bool, level, exp int64, dailyCount int64, dailyDate civil.Date) *apiaccount.Player {
	t.Helper()
	now := time.Now().UTC()
	var namePtr *string
	if name != "" {
		namePtr = &name
	}
	p := &apiaccount.Player{
		PlayerID:    playerID,
		FirebaseUID: firebaseUID,
		Name:        namePtr,
		Level:       level,
		Exp:         exp,
		IsPremium:   isPremium,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	ctx := context.Background()
	_, err := sharedPg.Pool.Exec(ctx,
		`INSERT INTO account.players (player_id, firebase_uid, name, is_premium, equipped_icon_no, premium_expires_at, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		p.PlayerID, p.FirebaseUID, p.Name, p.IsPremium,
		p.EquippedIconNo, p.PremiumExpiresAt, p.CreatedAt, p.UpdatedAt)
	require.NoError(t, err)

	_, err = sharedPg.Pool.Exec(ctx,
		`INSERT INTO account.player_progression (player_id, level, exp) VALUES ($1,$2,$3)`,
		p.PlayerID, p.Level, p.Exp)
	require.NoError(t, err)

	if dailyCount >= 0 {
		_, err = sharedPg.Pool.Exec(ctx,
			`INSERT INTO account.player_daily_battle (player_id, game_date, daily_battle_count)
			 VALUES ($1, $2, $3)`,
			p.PlayerID,
			time.Date(dailyDate.Year, dailyDate.Month, dailyDate.Day, 0, 0, 0, 0, time.UTC),
			dailyCount,
		)
		require.NoError(t, err)
	}
	return p
}

// seedPlayerSettings は account.player_settings に 1 行追加する。
func seedPlayerSettings(t *testing.T, playerID, language string, bgm, se int64, push bool) {
	t.Helper()
	_, err := sharedPg.Pool.Exec(context.Background(),
		`INSERT INTO account.player_settings (player_id, language, bgm_volume, se_volume, push_enabled)
		 VALUES ($1, $2, $3, $4, $5)`,
		playerID, language, bgm, se, push)
	require.NoError(t, err)
}

// newRealRepos は実 PostgreSQL repository と TxManager のセットを返す。
// service テストで内部 mock を一切使わず、shop 流の「実 DB + 外部依存のみ fake」
// パターンで service を組むためのヘルパ。
func newRealRepos() (*postgres.PlayerRepository, *postgres.FactionRepository, *postgres.PlayerSettingsRepository, *postgres.TxManager) {
	return postgres.NewPlayerRepository(sharedPg.Pool),
		postgres.NewFactionRepository(sharedPg.Pool),
		postgres.NewPlayerSettingsRepository(sharedPg.Pool),
		postgres.NewTxManager(sharedPg.Pool)
}

// newProcessedEventRepo は実 PostgreSQL の processed_events リポジトリを返す。
// OnboardingService の冪等ガード Tx 境界テスト等で使用する。
func newProcessedEventRepo() *postgres.ProcessedEventRepository {
	return postgres.NewProcessedEventRepository(sharedPg.Pool)
}

// 共通テストプレイヤー ID — postgres 層テストと重複するが、
// 別パッケージなのでここで再定義する（shop と同じ運用）。
const (
	testPlayerID1 = "11111111-1111-1111-1111-111111111111"
	testPlayerID2 = "22222222-2222-2222-2222-222222222222"
)
