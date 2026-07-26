//go:build integration

package rest

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres/postgrestest"
)

var sharedPg *postgrestest.Postgres

func TestMain(m *testing.M) {
	os.Exit(postgrestest.RunMain(m, &sharedPg,
		postgrestest.WithSchemaFile("db/schema.sql"),
		postgrestest.WithSchema("account"),
	))
}

// fakeGameConfigRepo は Firestore を起こさないための port.GameConfigRepo 最小 fake。
// errKeys に登録したキーは values より優先して任意のエラーを返す (500 写像など
// port.ErrNotFound 以外の伝播経路を注入するための境界フェイク)。
type fakeGameConfigRepo struct {
	values  map[string]int64
	errKeys map[string]error
}

var _ port.GameConfigRepo = (*fakeGameConfigRepo)(nil)

// GetInt64 はキー不在を握りつぶさず production の Firestore 実装と同じく port.ErrNotFound を返す。
func (f *fakeGameConfigRepo) GetInt64(_ context.Context, key string) (int64, error) {
	if err, ok := f.errKeys[key]; ok {
		return 0, err
	}
	if v, ok := f.values[key]; ok {
		return v, nil
	}
	return 0, fmt.Errorf("game config %q: %w", key, port.ErrNotFound)
}

// seedPlayer は 1 プレイヤーをシードする。
func seedPlayer(t *testing.T, playerID, firebaseUID string) {
	t.Helper()
	ctx := context.Background()
	_, err := sharedPg.Pool.Exec(ctx,
		`INSERT INTO account.players (player_id, firebase_uid, name, is_premium) VALUES ($1,$2,$3,$4)`,
		playerID, firebaseUID, "Seeded", false)
	require.NoError(t, err)
	_, err = sharedPg.Pool.Exec(ctx,
		`INSERT INTO account.player_progression (player_id, level, exp) VALUES ($1,1,0)`,
		playerID)
	require.NoError(t, err)
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
