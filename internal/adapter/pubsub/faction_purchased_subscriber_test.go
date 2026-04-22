package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/service"
)

// TestFactionPurchasedSubscriber_ProcessEvent は「shop 購入起因の faction ロスター
// 追加 (selected_faction は変更しない) を 1 イベント単位で冪等に処理する」という
// 仕様 (ADR-022) を固定する。Source 分岐ロジックが廃止されたことも併せて検証する
// (source フィールドを含まないペイロードで正常動作すること)。
func TestFactionPurchasedSubscriber_ProcessEvent(t *testing.T) {
	validEvent := apishop.FactionPurchasedEvent{
		EventType: apishop.EventTypeFactionPurchased,
		EventID:   "11111111-1111-1111-1111-111111111111",
		Timestamp: time.Now().UTC(),
		PlayerID:  "p-1",
		Faction:   "SHE",
	}
	validPayload, err := json.Marshal(validEvent)
	require.NoError(t, err)

	tests := []struct {
		name          string
		payload       []byte
		seedProcessed map[string]string
		addErr        error
		insertErr     error
		wantAck       bool
		assert        func(t *testing.T, factionRepo *fakeFactionRepo, eventRepo *fakeProcessedEventRepo)
	}{
		{
			name:    "正常系: player_factions に shop_purchase で INSERT して ACK",
			payload: validPayload,
			wantAck: true,
			assert: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
				require.Len(t, factionRepo.added, 1)
				assert.Equal(t, "p-1", factionRepo.added[0].PlayerID)
				assert.Equal(t, "SHE", factionRepo.added[0].Faction)
				assert.Equal(t, service.FactionSourceShopPurchase, factionRepo.added[0].Source)
			},
		},
		{
			name:          "冪等: 同一 event_id が既に processed_events にあれば副作用を起こさず ACK",
			payload:       validPayload,
			seedProcessed: map[string]string{validEvent.EventID: validEvent.EventType},
			wantAck:       true,
			assert: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
				assert.Empty(t, factionRepo.added)
			},
		},
		{
			name:    "不正 JSON: 握りつぶさず NACK",
			payload: []byte("{not-json"),
			wantAck: false,
			assert: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
				assert.Empty(t, factionRepo.added)
			},
		},
		{
			name: "未知の event_type: 責務外として ACK (副作用なし)",
			payload: mustMarshal(t, apishop.FactionPurchasedEvent{
				EventType: "unknown",
				EventID:   "22222222-2222-2222-2222-222222222222",
				PlayerID:  "p-2",
				Faction:   "Tenki",
			}),
			wantAck: true,
			assert: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
				assert.Empty(t, factionRepo.added)
			},
		},
		{
			name:      "processed_events INSERT 失敗: NACK でリトライ",
			payload:   validPayload,
			insertErr: errors.New("db unavailable"),
			wantAck:   false,
			assert: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
				assert.Empty(t, factionRepo.added)
			},
		},
		{
			name:    "player_factions INSERT 失敗: NACK でリトライ",
			payload: validPayload,
			addErr:  errors.New("fk violation"),
			wantAck: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			factionRepo := &fakeFactionRepo{addErr: tt.addErr}
			eventRepo := newFakeProcessedEventRepo()
			eventRepo.insertErr = tt.insertErr
			for k, v := range tt.seedProcessed {
				eventRepo.seen[k] = v
			}

			s := &FactionPurchasedSubscriber{
				factionRepo: factionRepo,
				txRunner:    fakeTxRunner{},
				eventRepo:   eventRepo,
			}
			ack := s.processEvent(context.Background(), tt.payload)
			assert.Equal(t, tt.wantAck, ack)
			if tt.assert != nil {
				tt.assert(t, factionRepo, eventRepo)
			}
		})
	}
}

func mustMarshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}
