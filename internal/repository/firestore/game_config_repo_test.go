//go:build integration

package firestore_test

import (
	"context"
	"os"
	"testing"

	cloudfirestore "cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/repository/firestore"
)

// newTestClient は FIRESTORE_EMULATOR_HOST に接続する Firestore クライアントを返す。
// testing.md の方針によりエミュレータ未起動時はスキップせず失敗させる。
func newTestClient(t *testing.T) *cloudfirestore.Client {
	t.Helper()
	if os.Getenv("FIRESTORE_EMULATOR_HOST") == "" {
		t.Fatal("FIRESTORE_EMULATOR_HOST が設定されていない。Firestore エミュレータを起動してから実行すること")
	}
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT_ID")
	if projectID == "" {
		projectID = "overload-party-test"
	}
	client, err := cloudfirestore.NewClient(context.Background(), projectID)
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestGameConfigRepository_GetInt64(t *testing.T) {
	t.Run("GameConfigRepository", func(t *testing.T) {
		t.Run("GetInt64", func(t *testing.T) {
			t.Run("指定したキーのドキュメントが存在するとき、その値を返す", func(t *testing.T) {
				client := newTestClient(t)
				key := "test-key-" + uuid.NewString()
				_, err := client.Collection("game_config").Doc(key).Set(context.Background(), map[string]any{"value": int64(42)})
				require.NoError(t, err)
				repo := firestore.NewGameConfigRepository(client)

				value, err := repo.GetInt64(context.Background(), key)

				require.NoError(t, err)
				assert.Equal(t, int64(42), value)
			})

			t.Run("指定したキーのドキュメントが存在しないとき、見つからないことを示すエラーを返す", func(t *testing.T) {
				client := newTestClient(t)
				repo := firestore.NewGameConfigRepository(client)

				_, err := repo.GetInt64(context.Background(), "missing-key-"+uuid.NewString())

				assert.ErrorIs(t, err, port.ErrNotFound)
			})
		})
	})
}
