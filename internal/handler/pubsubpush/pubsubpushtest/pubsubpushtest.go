// Package pubsubpushtest は Pub/Sub 購読のテストで共有する、push の本文の組み立て・送信と ERROR ログの確認のヘルパを提供する。
package pubsubpushtest

import (
	"encoding/base64"
	"encoding/json"
	"maps"
	"testing"

	"github.com/stretchr/testify/require"
)

// BuildEventEnvelope は、イベント本文 eventBody を載せたメッセージの ID が messageID である push 受信本文を返す。
func BuildEventEnvelope(t *testing.T, eventBody, messageID string) string {
	t.Helper()
	return BuildEventEnvelopeWithFields(t, eventBody, messageID, nil)
}

// BuildEventEnvelopeWithFields は、BuildEventEnvelope の push 受信本文に、fields の項目を最上位へ加えたものを返す。
func BuildEventEnvelopeWithFields(t *testing.T, eventBody, messageID string, fields map[string]any) string {
	t.Helper()
	envelope := map[string]any{
		"message": map[string]any{
			"data":      base64.StdEncoding.EncodeToString([]byte(eventBody)),
			"messageId": messageID,
		},
	}
	maps.Copy(envelope, fields)
	encoded, err := json.Marshal(envelope)
	require.NoError(t, err)
	return string(encoded)
}
