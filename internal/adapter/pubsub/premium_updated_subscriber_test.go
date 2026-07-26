package pubsub

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleMessagePremiumUpdated(t *testing.T) {
	t.Run("premium_updated イベントの処理", func(t *testing.T) {
		const existingEventID = "aaaaaaaa-1111-1111-1111-aaaaaaaaaaaa"
		expiry := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)

		// 契約検証は apishop 側の型をそのまま使う。shop が schema を変えたら本テストが
		// compile / 実行で破綻し、乖離を CI で検知できるようにするため。
		tests := []struct {
			name            string
			payload         []byte
			seedProcessed   map[string]string
			updateErr       error
			insertErr       error
			wantErr         bool
			wantErrContains string
			assertRepos     func(t *testing.T, premiumRepo *fakePremiumRepo)
		}{
			{
				name: "premium=true + expiry を受けたとき、players に反映して成功になる",
				payload: mustMarshal(t, apishop.PremiumUpdatedEvent{
					EventType:        apishop.EventTypePremiumUpdated,
					EventID:          "bbbbbbbb-0001-0001-0001-000000000001",
					PlayerID:         "p-1",
					IsPremium:        true,
					PremiumExpiresAt: &expiry,
				}),
				wantErr: false,
				assertRepos: func(t *testing.T, premiumRepo *fakePremiumRepo) {
					state, ok := premiumRepo.premium["p-1"]
					require.True(t, ok, "p-1 の premium 状態が反映されている")
					assert.True(t, state.IsPremium)
					require.NotNil(t, state.ExpiresAt)
					assert.True(t, state.ExpiresAt.Equal(expiry))
				},
			},
			{
				name: "premium=false を受けたとき、players に反映して成功になる",
				payload: mustMarshal(t, apishop.PremiumUpdatedEvent{
					EventType: apishop.EventTypePremiumUpdated,
					EventID:   "bbbbbbbb-0002-0002-0002-000000000002",
					PlayerID:  "p-2",
					IsPremium: false,
				}),
				wantErr: false,
				assertRepos: func(t *testing.T, premiumRepo *fakePremiumRepo) {
					state, ok := premiumRepo.premium["p-2"]
					require.True(t, ok)
					assert.False(t, state.IsPremium)
					assert.Nil(t, state.ExpiresAt)
				},
			},
			{
				name: "同一 event_id が processed_events に既にあるとき、副作用なしで成功になる",
				payload: mustMarshal(t, apishop.PremiumUpdatedEvent{
					EventType: apishop.EventTypePremiumUpdated,
					EventID:   existingEventID,
					PlayerID:  "p-3",
					IsPremium: true,
				}),
				seedProcessed: map[string]string{existingEventID: apishop.EventTypePremiumUpdated},
				wantErr:       false,
				assertRepos: func(t *testing.T, premiumRepo *fakePremiumRepo) {
					_, ok := premiumRepo.premium["p-3"]
					assert.False(t, ok, "冪等スキップ時は player 状態が更新されない")
				},
			},
			{
				name:            "不正な JSON のとき、握りつぶさず失敗になる",
				payload:         []byte("{malformed"),
				wantErr:         true,
				wantErrContains: "premium-updated: bad payload",
				assertRepos: func(t *testing.T, premiumRepo *fakePremiumRepo) {
					assert.Empty(t, premiumRepo.premium, "JSON parse 失敗時は副作用が残らない")
				},
			},
			{
				name: "未知の event_type のとき、責務外として副作用なく成功になる",
				payload: mustMarshal(t, apishop.PremiumUpdatedEvent{
					EventType: "unknown",
					EventID:   "bbbbbbbb-0003-0003-0003-000000000003",
					PlayerID:  "p-4",
				}),
				wantErr: false,
				assertRepos: func(t *testing.T, premiumRepo *fakePremiumRepo) {
					_, ok := premiumRepo.premium["p-4"]
					assert.False(t, ok)
				},
			},
			{
				name: "processed_events への INSERT が失敗するとき、UpdatePremium まで到達せず失敗になる",
				payload: mustMarshal(t, apishop.PremiumUpdatedEvent{
					EventType: apishop.EventTypePremiumUpdated,
					EventID:   "bbbbbbbb-0004-0004-0004-000000000004",
					PlayerID:  "p-5",
					IsPremium: true,
				}),
				insertErr:       errors.New("db unavailable"),
				wantErr:         true,
				wantErrContains: "insert processed_events",
				assertRepos: func(t *testing.T, premiumRepo *fakePremiumRepo) {
					assert.Empty(t, premiumRepo.premium, "processed_events INSERT 失敗時は UpdatePremium まで到達しない")
				},
			},
			{
				name: "UpdatePremium が失敗するとき、tx rollback で副作用を残さず失敗になる",
				payload: mustMarshal(t, apishop.PremiumUpdatedEvent{
					EventType: apishop.EventTypePremiumUpdated,
					EventID:   "bbbbbbbb-0005-0005-0005-000000000005",
					PlayerID:  "p-6",
					IsPremium: true,
				}),
				updateErr:       errors.New("row not found"),
				wantErr:         true,
				wantErrContains: "update premium",
				assertRepos: func(t *testing.T, premiumRepo *fakePremiumRepo) {
					assert.Empty(t, premiumRepo.premium, "UpdatePremium 失敗時は tx rollback により副作用が残らない")
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				premiumRepo := newFakePremiumRepo()
				premiumRepo.updatePremiumErr = tt.updateErr
				eventRepo := newFakeProcessedEventRepo()
				eventRepo.insertErr = tt.insertErr
				for k, v := range tt.seedProcessed {
					eventRepo.seen[k] = v
				}

				sub := NewPremiumUpdatedSubscriber(premiumRepo, fakeTxRunner{}, eventRepo)

				err := sub.HandleMessage(context.Background(), tt.payload)

				assert.Equal(t, tt.wantErr, err != nil, "エラー有無 (err=%v)", err)
				assert.Contains(t, fmt.Sprintf("%v", err), tt.wantErrContains, "エラー内容が原因を区別できる")
				tt.assertRepos(t, premiumRepo)
			})
		}
	})
}
