// Package firestoretest は Firestore エミュレータを使う、account サービスのテストヘルパを提供する。
package firestoretest

import (
	"os"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// EmulatorHostEnv は Firestore エミュレータの接続先を指定する環境変数の名前である。
	EmulatorHostEnv = "FIRESTORE_EMULATOR_HOST"

	gameConfigCollection = "game_config"
	gameConfigValueField = "value"
	projectIDPrefix      = "test-project-"
)

// NewProjectID は、他のテストと保存内容が混ざらない、テスト専用のプロジェクト ID を返す。
func NewProjectID() string {
	return projectIDPrefix + uuid.NewString()
}

// RequireEmulatorHost は Firestore エミュレータの接続先を返し、環境変数に無いときはテストを失敗させる。
func RequireEmulatorHost(t *testing.T) string {
	t.Helper()
	host := os.Getenv(EmulatorHostEnv)
	require.NotEmpty(t, host, EmulatorHostEnv+" must point to the Firestore emulator")
	return host
}

// NewClient は Firestore エミュレータに projectID で接続するクライアントを返し、テストの終了時に閉じる。
// エミュレータの接続先が環境変数に無いときはテストを失敗させる。
func NewClient(t *testing.T, projectID string) *firestore.Client {
	t.Helper()
	RequireEmulatorHost(t)
	client, err := firestore.NewClient(t.Context(), projectID)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, client.Close()) })
	return client
}

// SetGameConfigFields はゲーム設定のキー key のドキュメントを fields で書き込む。
func SetGameConfigFields(t *testing.T, client *firestore.Client, key string, fields map[string]any) {
	t.Helper()
	_, err := client.Collection(gameConfigCollection).Doc(key).Set(t.Context(), fields)
	require.NoError(t, err)
}

// SetGameConfigValue はゲーム設定のキー key の値を value で書き込む。
func SetGameConfigValue(t *testing.T, client *firestore.Client, key string, value any) {
	t.Helper()
	SetGameConfigFields(t, client, key, map[string]any{gameConfigValueField: value})
}

// SeedGameConfig はゲーム設定を values のとおりに書き込む。
func SeedGameConfig(t *testing.T, client *firestore.Client, values map[string]int64) {
	t.Helper()
	for key, value := range values {
		SetGameConfigValue(t, client, key, value)
	}
}
