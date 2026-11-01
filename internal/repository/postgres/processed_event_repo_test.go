//go:build integration

package postgres_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres/postgrestest"
)

const eventID2 = "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"

func TestProcessedEventRecording(t *testing.T) {
	t.Run("[処理済みイベント]イベントIDの記録", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("記録すると", func(t *testing.T) {
				tests := []struct {
					name    string
					prepare func(t *testing.T, r *postgrestest.Repositories)
					eventID string
				}{
					{
						name:    "記録が無いイベントIDのとき、新規に記録されたという結果が返る",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {},
						eventID: postgrestest.EventID1,
					},
					{
						name: "記録するイベントIDとは別のイベントIDが記録済みのとき、新規に記録されたという結果が返る",
						prepare: func(t *testing.T, r *postgrestest.Repositories) {
							_, err := r.Events.InsertProcessedEvent(t.Context(), postgrestest.EventID1, "test_event_a")
							require.NoError(t, err)
						},
						eventID: eventID2,
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						r := postgrestest.NewRepositories(t, sharedPg)
						tt.prepare(t, r)

						got, err := r.Events.InsertProcessedEvent(t.Context(), tt.eventID, "test_event_a")

						require.NoError(t, err)
						assert.True(t, got)
					})
				}
			})
			t.Run("イベントIDが種別「test_event_a」で記録済みのとき、同じイベントIDを記録すると", func(t *testing.T) {
				recordAgain := func(t *testing.T, eventType string) (*postgrestest.Repositories, bool) {
					t.Helper()
					r := postgrestest.NewRepositories(t, sharedPg)
					_, err := r.Events.InsertProcessedEvent(t.Context(), postgrestest.EventID1, "test_event_a")
					require.NoError(t, err)

					got, err := r.Events.InsertProcessedEvent(t.Context(), postgrestest.EventID1, eventType)

					require.NoError(t, err)
					return r, got
				}
				resultTests := []struct {
					name      string
					eventType string
				}{
					{name: "記録する種別が「test_event_a」のとき、既に記録済みという結果が返る", eventType: "test_event_a"},
					{name: "記録する種別が「test_event_b」のとき、既に記録済みという結果が返る", eventType: "test_event_b"},
				}
				for _, tt := range resultTests {
					t.Run(tt.name, func(t *testing.T) {
						_, got := recordAgain(t, tt.eventType)

						assert.False(t, got)
					})
				}
				savedCountTests := []struct {
					name      string
					eventType string
				}{
					{name: "記録する種別が「test_event_a」のとき、データベースに保存された処理済みイベントは1件のままになる", eventType: "test_event_a"},
					{name: "記録する種別が「test_event_b」のとき、データベースに保存された処理済みイベントは1件のままになる", eventType: "test_event_b"},
				}
				for _, tt := range savedCountTests {
					t.Run(tt.name, func(t *testing.T) {
						r, _ := recordAgain(t, tt.eventType)

						assert.Equal(t, 1, r.CountProcessedEvents(t))
					})
				}
			})
			t.Run("同じイベントIDを2回同時に記録すると", func(t *testing.T) {
				type recordResult struct {
					recorded bool
					err      error
				}
				recordConcurrently := func(t *testing.T) (*postgrestest.Repositories, []recordResult) {
					t.Helper()
					r := postgrestest.NewRepositories(t, sharedPg)

					results := postgrestest.RunConcurrently(2, func(int) recordResult {
						recorded, err := r.Events.InsertProcessedEvent(t.Context(), postgrestest.EventID1, "test_event_a")
						return recordResult{recorded: recorded, err: err}
					})
					return r, results
				}
				t.Run("新規に記録されたという結果が返るのは、片方の呼び出しだけになる", func(t *testing.T) {
					_, results := recordConcurrently(t)

					recorded := make([]bool, 0, len(results))
					for _, result := range results {
						require.NoError(t, result.err)
						recorded = append(recorded, result.recorded)
					}
					assert.ElementsMatch(t, []bool{true, false}, recorded)
				})
				t.Run("データベースに保存された処理済みイベントは1件になる", func(t *testing.T) {
					r, results := recordConcurrently(t)

					for _, result := range results {
						require.NoError(t, result.err)
					}
					assert.Equal(t, 1, r.CountProcessedEvents(t))
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("呼び出しが取り消し済みのとき、記録すると、取り消しを示すエラーになる", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				_, err := r.Events.InsertProcessedEvent(ctx, postgrestest.EventID1, "test_event_a")

				assert.ErrorIs(t, err, context.Canceled)
			})
			t.Run("呼び出しが取り消し済みでイベントIDの記録がエラーになった後に、取り消されていない呼び出しで同じイベントIDを記録すると、新規に記録されたという結果が返る", func(t *testing.T) {
				r := postgrestest.NewRepositories(t, sharedPg)
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				_, err := r.Events.InsertProcessedEvent(ctx, postgrestest.EventID1, "test_event_a")
				require.Error(t, err)

				got, err := r.Events.InsertProcessedEvent(t.Context(), postgrestest.EventID1, "test_event_a")

				require.NoError(t, err)
				assert.True(t, got)
			})
		})
	})
}
