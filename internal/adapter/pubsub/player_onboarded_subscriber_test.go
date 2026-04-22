package pubsub

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	apiscenario "github.com/kenyamaneko/overload-party-scenario/packages/api-scenario"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPlayerOnboardedSubscriber_ProcessEvent は「onboarding 完了イベント 1 本で
// 表示名更新 + 初期ファクション ロスター追加 + selected_faction 確定を同一
// トランザクションで行い、event_id で冪等化する」という仕様 (ADR-022) を固定する。
func TestPlayerOnboardedSubscriber_ProcessEvent(t *testing.T) {
	validEvent := apiscenario.PlayerOnboardedEvent{
		EventType:        apiscenario.EventTypePlayerOnboarded,
		EventID:          "11111111-1111-1111-1111-111111111111",
		Timestamp:        time.Now().UTC(),
		PlayerID:         "p-1",
		DisplayName:      "name-1",
		InitialFactionID: "SHE",
	}
	validPayload, err := json.Marshal(validEvent)
	require.NoError(t, err)

	tests := []struct {
		name            string
		payload         []byte
		seedProcessed   map[string]string
		seedSelected    bool
		updateUsernameE error
		addErr          error
		trySetErr       error
		insertErr       error
		wantAck         bool
		assertFn        func(t *testing.T, player *fakePlayerRepo, faction *fakeFactionRepo)
	}{
		{
			name:    "正常系: username/player_factions/selected_faction を全て反映して ACK",
			payload: validPayload,
			wantAck: true,
			assertFn: func(t *testing.T, player *fakePlayerRepo, faction *fakeFactionRepo) {
				assert.Equal(t, "name-1", player.usernames["p-1"])
				require.Len(t, faction.added, 1)
				assert.Equal(t, "initial_selection", faction.added[0].Source)
				assert.Equal(t, "SHE", faction.added[0].Faction)
				assert.Equal(t, "SHE", player.selectedFaction["p-1"])
			},
		},
		{
			name:          "冪等: 同一 event_id が processed_events にあれば副作用を起こさず ACK",
			payload:       validPayload,
			seedProcessed: map[string]string{validEvent.EventID: validEvent.EventType},
			wantAck:       true,
			assertFn: func(t *testing.T, player *fakePlayerRepo, faction *fakeFactionRepo) {
				assert.Empty(t, player.usernames)
				assert.Empty(t, faction.added)
				assert.Empty(t, player.selectedFaction)
			},
		},
		{
			name:         "既に selected_faction が埋まっているケース: username/player_factions は通し、selected_faction 書き込みはスキップされる (set=false)",
			payload:      validPayload,
			seedSelected: true,
			wantAck:      true,
			assertFn: func(t *testing.T, player *fakePlayerRepo, faction *fakeFactionRepo) {
				assert.Equal(t, "name-1", player.usernames["p-1"])
				require.Len(t, faction.added, 1)
				// selected_faction は書き換わらない。seedSelected=true により
				// TrySetInitialFaction が set=false を返す経路に入る。
				assert.NotContains(t, player.selectedFaction, "p-1")
			},
		},
		{
			name:    "不正 JSON: 握りつぶさず NACK",
			payload: []byte("broken"),
			wantAck: false,
			assertFn: func(t *testing.T, player *fakePlayerRepo, faction *fakeFactionRepo) {
				assert.Empty(t, player.usernames)
				assert.Empty(t, faction.added)
			},
		},
		{
			name: "未知 event_type: 責務外として ACK",
			payload: mustMarshal(t, apiscenario.PlayerOnboardedEvent{
				EventType: "unknown",
				EventID:   "22222222-2222-2222-2222-222222222222",
				PlayerID:  "p-2",
			}),
			wantAck: true,
			assertFn: func(t *testing.T, player *fakePlayerRepo, faction *fakeFactionRepo) {
				assert.Empty(t, player.usernames)
				assert.Empty(t, faction.added)
			},
		},
		{
			name: "initial_faction_id 欠落: ペイロード仕様違反で NACK",
			payload: mustMarshal(t, apiscenario.PlayerOnboardedEvent{
				EventType:   apiscenario.EventTypePlayerOnboarded,
				EventID:     "33333333-3333-3333-3333-333333333333",
				PlayerID:    "p-3",
				DisplayName: "x",
			}),
			wantAck: false,
			assertFn: func(t *testing.T, player *fakePlayerRepo, faction *fakeFactionRepo) {
				assert.Empty(t, player.usernames)
				assert.Empty(t, faction.added)
			},
		},
		{
			name:            "UpdateUsername 失敗: NACK でリトライ",
			payload:         validPayload,
			updateUsernameE: errors.New("db error"),
			wantAck:         false,
		},
		{
			name:    "AddPlayerFaction 失敗: NACK でリトライ",
			payload: validPayload,
			addErr:  errors.New("fk violation"),
			wantAck: false,
		},
		{
			name:      "TrySetInitialFaction 失敗: NACK でリトライ",
			payload:   validPayload,
			trySetErr: errors.New("db error"),
			wantAck:   false,
		},
		{
			name:      "processed_events INSERT 失敗: NACK でリトライ",
			payload:   validPayload,
			insertErr: errors.New("db unavailable"),
			wantAck:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			playerRepo := newFakePlayerRepo()
			playerRepo.updateUsernameErr = tt.updateUsernameE
			playerRepo.trySetFactionErr = tt.trySetErr
			if tt.seedSelected {
				playerRepo.selectedInitiallyFilled["p-1"] = true
			}
			factionRepo := &fakeFactionRepo{addErr: tt.addErr}
			eventRepo := newFakeProcessedEventRepo()
			eventRepo.insertErr = tt.insertErr
			for k, v := range tt.seedProcessed {
				eventRepo.seen[k] = v
			}

			s := &PlayerOnboardedSubscriber{
				playerRepo:  playerRepo,
				factionRepo: factionRepo,
				txRunner:    fakeTxRunner{},
				eventRepo:   eventRepo,
			}
			ack := s.processEvent(context.Background(), tt.payload)
			assert.Equal(t, tt.wantAck, ack)
			if tt.assertFn != nil {
				tt.assertFn(t, playerRepo, factionRepo)
			}
		})
	}
}
