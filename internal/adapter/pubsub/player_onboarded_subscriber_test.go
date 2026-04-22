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

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// fakeOnboardingApplier は OnboardingApplier を満たす最小スタブ。
// subscriber は「service 層にイベント内容を正しく委譲し、戻り値に応じて
// ACK / NACK / 警告ログ分岐だけを行う」契約なので、service 内部の Tx / repo
// 挙動はここで抽象化する (subscriber テストで repo fake を直接触らない)。
type fakeOnboardingApplier struct {
	// returnProcessed / returnSelected は Apply の戻り値を制御する。
	returnProcessed bool
	returnSelected  bool
	// returnErr を設定すると Apply が常にこのエラーを返す。
	returnErr error

	called    bool
	gotEvent  string
	gotType   string
	gotPlayer string
	gotName   string
	gotFac    string
}

func (f *fakeOnboardingApplier) ApplyOnboardingResult(
	_ context.Context,
	eventID, eventType, playerID, displayName, initialFactionID string,
) (bool, bool, error) {
	f.called = true
	f.gotEvent = eventID
	f.gotType = eventType
	f.gotPlayer = playerID
	f.gotName = displayName
	f.gotFac = initialFactionID
	if f.returnErr != nil {
		return false, false, f.returnErr
	}
	return f.returnProcessed, f.returnSelected, nil
}

// TestPlayerOnboardedSubscriber_ProcessEvent は「Pub/Sub ペイロードを Unmarshal して
// OnboardingApplier に委譲する」という subscriber 単体の仕様を固定する。
// SelectableFactions / 冪等ガード / Tx 境界などの業務不変条件は service 層の責務なので
// ここではテストせず、返却値ごとの ACK / NACK / ログ分岐のみを確認する。
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
		returnProcessed bool
		returnSelected  bool
		returnErr       error
		wantAck         bool
		wantApplierCall bool
		// assertApplier は wantApplierCall=true のケースで委譲引数を検証する。
		assertApplier func(t *testing.T, a *fakeOnboardingApplier)
	}{
		{
			name:            "正常系: service が processed=true/selected=true を返したら ACK",
			payload:         validPayload,
			returnProcessed: true,
			returnSelected:  true,
			wantAck:         true,
			wantApplierCall: true,
			assertApplier: func(t *testing.T, a *fakeOnboardingApplier) {
				assert.Equal(t, validEvent.EventID, a.gotEvent)
				assert.Equal(t, validEvent.EventType, a.gotType)
				assert.Equal(t, "p-1", a.gotPlayer)
				assert.Equal(t, "name-1", a.gotName)
				assert.Equal(t, "SHE", a.gotFac)
			},
		},
		{
			name:            "冪等スキップ: service が processed=false を返したら副作用なし ACK",
			payload:         validPayload,
			returnProcessed: false,
			wantAck:         true,
			wantApplierCall: true,
		},
		{
			name:            "selected_faction 既存: processed=true/selected=false は警告ログのみで ACK",
			payload:         validPayload,
			returnProcessed: true,
			returnSelected:  false,
			wantAck:         true,
			wantApplierCall: true,
		},
		{
			name:            "不正 JSON: 握りつぶさず NACK (service 未呼び出し)",
			payload:         []byte("broken"),
			wantAck:         false,
			wantApplierCall: false,
		},
		{
			name: "未知 event_type: 責務外として ACK (service 未呼び出し)",
			payload: mustMarshal(t, apiscenario.PlayerOnboardedEvent{
				EventType: "unknown",
				EventID:   "22222222-2222-2222-2222-222222222222",
				PlayerID:  "p-2",
			}),
			wantAck:         true,
			wantApplierCall: false,
		},
		{
			name: "initial_faction_id 欠落: ペイロード仕様違反で NACK (service 未呼び出し)",
			payload: mustMarshal(t, apiscenario.PlayerOnboardedEvent{
				EventType:   apiscenario.EventTypePlayerOnboarded,
				EventID:     "33333333-3333-3333-3333-333333333333",
				PlayerID:    "p-3",
				DisplayName: "x",
			}),
			wantAck:         false,
			wantApplierCall: false,
		},
		{
			name:            "service 汎用エラー: NACK でリトライ",
			payload:         validPayload,
			returnErr:       errors.New("db error"),
			wantAck:         false,
			wantApplierCall: true,
		},
		{
			name:            "service が ErrNotFound: publisher バグとして明示 NACK (error ログ)",
			payload:         validPayload,
			returnErr:       port.ErrNotFound,
			wantAck:         false,
			wantApplierCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			applier := &fakeOnboardingApplier{
				returnProcessed: tt.returnProcessed,
				returnSelected:  tt.returnSelected,
				returnErr:       tt.returnErr,
			}
			s := &PlayerOnboardedSubscriber{applier: applier}

			// api-scenario 側に test fake が未整備のため、本 subscriber は
			// processEvent 直呼びで検証する。将来 apiscenariofake が整備
			// されたら apishopfakeStream と同様の Start() ループ越し検証に移行する。
			err := s.processEvent(context.Background(), tt.payload)
			assert.Equal(t, tt.wantAck, err == nil, "ack 判定 (nil=ack, err=%v)", err)
			assert.Equal(t, tt.wantApplierCall, applier.called)
			if tt.assertApplier != nil {
				tt.assertApplier(t, applier)
			}
		})
	}
}
