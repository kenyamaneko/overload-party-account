package pubsub

import (
	"context"
	"errors"
	"fmt"
	"testing"

	apishop "github.com/kenyamaneko/overload-party-shop/packages/api-shop"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleMessageFactionAcquired(t *testing.T) {
	t.Run("faction_acquiredイベントの処理", func(t *testing.T) {
		const existingEventID = "11111111-1111-1111-1111-111111111111"

		// 契約検証は apishop 側の型をそのまま使う。shop が schema を変えたら本テストが
		// compile / 実行で破綻し、乖離を CI で検知できるようにするため。
		tests := []struct {
			name            string
			payload         []byte
			seedProcessed   map[string]string
			addErr          error
			insertErr       error
			wantErr         bool
			wantErrContains string
			assertRepos     func(t *testing.T, factionRepo *fakeFactionRepo, eventRepo *fakeProcessedEventRepo)
		}{
			{
				name: "有効なfaction_acquiredを受けたとき、player_factionsにis_initial=FALSEで追加して成功になる",
				payload: mustMarshal(t, apishop.FactionAcquiredEvent{
					EventType: apishop.EventTypeFactionAcquired,
					EventID:   "aaaaaaaa-0001-0001-0001-000000000001",
					PlayerID:  "p-1",
					Faction:   "SHE",
				}),
				wantErr: false,
				assertRepos: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
					require.Len(t, factionRepo.added, 1)
					assert.Equal(t, "p-1", factionRepo.added[0].PlayerID)
					assert.Equal(t, "SHE", factionRepo.added[0].Faction)
					assert.False(t, factionRepo.added[0].IsInitial)
				},
			},
			{
				name: "同一event_idがprocessed_eventsに既にあるとき、副作用なしで成功になる",
				payload: mustMarshal(t, apishop.FactionAcquiredEvent{
					EventType: apishop.EventTypeFactionAcquired,
					EventID:   existingEventID,
					PlayerID:  "p-2",
					Faction:   "Tenki",
				}),
				seedProcessed: map[string]string{existingEventID: apishop.EventTypeFactionAcquired},
				wantErr:       false,
				assertRepos: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
					assert.Empty(t, factionRepo.added)
				},
			},
			{
				name:            "不正なJSONのとき、握りつぶさず失敗になる",
				payload:         []byte("{not-json"),
				wantErr:         true,
				wantErrContains: "faction-acquired: bad payload",
				assertRepos: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
					assert.Empty(t, factionRepo.added)
				},
			},
			{
				name: "未知のevent_typeのとき、責務外として副作用なく成功になる",
				payload: mustMarshal(t, apishop.FactionAcquiredEvent{
					EventType: "unknown",
					EventID:   "aaaaaaaa-0002-0002-0002-000000000002",
					PlayerID:  "p-3",
					Faction:   "Sugar",
				}),
				wantErr: false,
				assertRepos: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
					assert.Empty(t, factionRepo.added)
				},
			},
			{
				name: "processed_eventsへのINSERTが失敗するとき、失敗になる",
				payload: mustMarshal(t, apishop.FactionAcquiredEvent{
					EventType: apishop.EventTypeFactionAcquired,
					EventID:   "aaaaaaaa-0003-0003-0003-000000000003",
					PlayerID:  "p-4",
					Faction:   "Tuners",
				}),
				insertErr:       errors.New("db unavailable"),
				wantErr:         true,
				wantErrContains: "insert processed_events",
				assertRepos: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
					assert.Empty(t, factionRepo.added)
				},
			},
			{
				name: "player_factionsへの追加が失敗するとき、副作用を残さず失敗になる",
				payload: mustMarshal(t, apishop.FactionAcquiredEvent{
					EventType: apishop.EventTypeFactionAcquired,
					EventID:   "aaaaaaaa-0004-0004-0004-000000000004",
					PlayerID:  "p-5",
					Faction:   "SHE",
				}),
				addErr:          errors.New("fk violation"),
				wantErr:         true,
				wantErrContains: "add player_faction",
				assertRepos: func(t *testing.T, factionRepo *fakeFactionRepo, _ *fakeProcessedEventRepo) {
					assert.Empty(t, factionRepo.added, "AddPlayerFaction 失敗時は副作用が残らない")
				},
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

				sub := NewFactionAcquiredSubscriber(factionRepo, fakeTxRunner{}, eventRepo)

				err := sub.HandleMessage(context.Background(), tt.payload)

				assert.Equal(t, tt.wantErr, err != nil, "エラー有無 (err=%v)", err)
				assert.Contains(t, fmt.Sprintf("%v", err), tt.wantErrContains, "エラー内容が原因を区別できる")
				tt.assertRepos(t, factionRepo, eventRepo)
			})
		}
	})
}
