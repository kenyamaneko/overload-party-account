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

// seedPlayer は account.players + account.player_daily_battle の最小シードを投入する。
// postgres 層テストの helpers_test.go と同じパターン。
func seedPlayer(t *testing.T, playerID, firebaseUID, username string, isPremium bool) *apiaccount.Player {
	t.Helper()
	now := time.Now().UTC()
	p := &apiaccount.Player{
		PlayerID:    playerID,
		FirebaseUID: firebaseUID,
		Username:    username,
		Level:       1,
		Exp:         0,
		IsPremium:   isPremium,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err := sharedPg.Pool.Exec(context.Background(),
		`INSERT INTO account.players (player_id, firebase_uid, username, level, exp, is_premium, equipped_icon_no, selected_faction, premium_expires_at, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		p.PlayerID, p.FirebaseUID, p.Username, p.Level, p.Exp, p.IsPremium,
		p.EquippedIconNo, p.SelectedFaction, p.PremiumExpiresAt, p.CreatedAt, p.UpdatedAt)
	require.NoError(t, err)

	today := civil.DateOf(now)
	_, err = sharedPg.Pool.Exec(context.Background(),
		`INSERT INTO account.player_daily_battle (player_id, daily_battle_count, last_reset_date)
		 VALUES ($1, $2, $3)`,
		p.PlayerID, 0, time.Date(today.Year, today.Month, today.Day, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	return p
}

// seedPlayerWithState は指定の level/exp/daily_battle 状態でシードする。
// GetBattleLimit / AwardExp のテスト用。
func seedPlayerWithState(t *testing.T, playerID, firebaseUID, username string, isPremium bool, level, exp int64, count int64, lastReset civil.Date) *apiaccount.Player {
	t.Helper()
	now := time.Now().UTC()
	p := &apiaccount.Player{
		PlayerID:    playerID,
		FirebaseUID: firebaseUID,
		Username:    username,
		Level:       level,
		Exp:         exp,
		IsPremium:   isPremium,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	_, err := sharedPg.Pool.Exec(context.Background(),
		`INSERT INTO account.players (player_id, firebase_uid, username, level, exp, is_premium, equipped_icon_no, selected_faction, premium_expires_at, created_at, updated_at)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)`,
		p.PlayerID, p.FirebaseUID, p.Username, p.Level, p.Exp, p.IsPremium,
		p.EquippedIconNo, p.SelectedFaction, p.PremiumExpiresAt, p.CreatedAt, p.UpdatedAt)
	require.NoError(t, err)

	_, err = sharedPg.Pool.Exec(context.Background(),
		`INSERT INTO account.player_daily_battle (player_id, daily_battle_count, last_reset_date)
		 VALUES ($1, $2, $3)`,
		p.PlayerID, count,
		time.Date(lastReset.Year, lastReset.Month, lastReset.Day, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	return p
}

// seedUserSettings は account.user_settings に 1 行追加する。
func seedUserSettings(t *testing.T, playerID, language string, bgm, se int64, push bool) {
	t.Helper()
	_, err := sharedPg.Pool.Exec(context.Background(),
		`INSERT INTO account.user_settings (player_id, language, bgm_volume, se_volume, push_enabled)
		 VALUES ($1, $2, $3, $4, $5)`,
		playerID, language, bgm, se, push)
	require.NoError(t, err)
}

// newRealRepos は実 PostgreSQL repository と TxManager のセットを返す。
// service テストで内部 mock を一切使わず、shop 流の「実 DB + 外部依存のみ fake」
// パターンで service を組むためのヘルパ。
func newRealRepos() (*postgres.PlayerRepository, *postgres.FactionRepository, *postgres.UserSettingsRepository, *postgres.TxManager) {
	return postgres.NewPlayerRepository(sharedPg.Pool),
		postgres.NewFactionRepository(sharedPg.Pool),
		postgres.NewUserSettingsRepository(sharedPg.Pool),
		postgres.NewTxManager(sharedPg.Pool)
}

// 共通テストプレイヤー ID — postgres 層テストと重複するが、
// 別パッケージなのでここで再定義する（shop と同じ運用）。
const (
	testPlayerID1 = "11111111-1111-1111-1111-111111111111"
	testPlayerID2 = "22222222-2222-2222-2222-222222222222"
)
