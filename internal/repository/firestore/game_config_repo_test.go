//go:build integration

package firestore_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	accountfirestore "github.com/kenyamaneko/overload-party-account/internal/repository/firestore"
)

var sharedClient *firestore.Client

func TestMain(m *testing.M) {
	emulatorHost := os.Getenv("FIRESTORE_EMULATOR_HOST")
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT_ID")
	if emulatorHost == "" || projectID == "" {
		fmt.Fprintln(os.Stderr, "game_config_repo_test: FIRESTORE_EMULATOR_HOST と GOOGLE_CLOUD_PROJECT_ID の両方が必要です")
		os.Exit(1)
	}

	client, err := firestore.NewClient(context.Background(), projectID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "game_config_repo_test: firestore client: %v\n", err)
		os.Exit(1)
	}
	sharedClient = client

	os.Exit(m.Run())
}

func TestGetInt64(t *testing.T) {
	repo := accountfirestore.NewGameConfigRepository(sharedClient)
	ctx := context.Background()

	t.Run("game_configの取得", func(t *testing.T) {
		t.Run("数値42を持つキーを読むと、42が返る", func(t *testing.T) {
			key := "TST-get-int64-ok"
			_, err := sharedClient.Collection("game_config").Doc(key).Set(ctx, map[string]any{"value": int64(42)})
			require.NoError(t, err)

			got, err := repo.GetInt64(ctx, key)
			require.NoError(t, err)
			assert.Equal(t, int64(42), got)
		})

		t.Run("キーのドキュメントが無いとき、port.ErrNotFoundになる", func(t *testing.T) {
			key := "TST-get-int64-missing"

			_, err := repo.GetInt64(ctx, key)
			require.ErrorIs(t, err, port.ErrNotFound)
		})

		t.Run("valueが数値でないとき、エラーになる", func(t *testing.T) {
			key := "TST-get-int64-invalid"
			_, err := sharedClient.Collection("game_config").Doc(key).Set(ctx, map[string]any{"value": "TST-not-a-number"})
			require.NoError(t, err)

			_, err = repo.GetInt64(ctx, key)
			require.Error(t, err)
		})
	})
}
