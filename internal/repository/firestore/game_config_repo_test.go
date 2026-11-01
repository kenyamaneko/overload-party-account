//go:build integration

package firestore_test

import (
	"context"
	"testing"

	"cloud.google.com/go/firestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	firestorerepo "github.com/kenyamaneko/overload-party-account/internal/repository/firestore"
	"github.com/kenyamaneko/overload-party-account/internal/repository/firestore/firestoretest"
)

type gameConfigFixture struct {
	client *firestore.Client
	repo   *firestorerepo.GameConfigRepository
}

func newGameConfigFixture(t *testing.T) *gameConfigFixture {
	t.Helper()
	client := firestoretest.NewClient(t, firestoretest.NewProjectID())
	return &gameConfigFixture{client: client, repo: firestorerepo.New(client)}
}

func TestGameConfigIntegerLookup(t *testing.T) {
	t.Run("[ゲーム設定]整数値の取得", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("設定値が12のとき、12が返る", func(t *testing.T) {
				f := newGameConfigFixture(t)
				firestoretest.SetGameConfigValue(t, f.client, "test_key_a", int64(12))

				got, err := f.repo.GetInt64(t.Context(), "test_key_a")

				require.NoError(t, err)
				assert.Equal(t, int64(12), got)
			})
			t.Run("キー「test_key_a」が12、キー「test_key_b」が34の2件が登録されているとき、キー「test_key_b」を指定すると34が返る", func(t *testing.T) {
				f := newGameConfigFixture(t)
				firestoretest.SetGameConfigValue(t, f.client, "test_key_a", int64(12))
				firestoretest.SetGameConfigValue(t, f.client, "test_key_b", int64(34))

				got, err := f.repo.GetInt64(t.Context(), "test_key_b")

				require.NoError(t, err)
				assert.Equal(t, int64(34), got)
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("キーを指定して取得すると", func(t *testing.T) {
				tests := []struct {
					name    string
					prepare func(t *testing.T, f *gameConfigFixture)
				}{
					{
						name:    "設定が1件も登録されていないとき、見つからないエラーになる",
						prepare: func(t *testing.T, f *gameConfigFixture) {},
					},
					{
						name: "別のキーの設定だけが登録されているとき、見つからないエラーになる",
						prepare: func(t *testing.T, f *gameConfigFixture) {
							firestoretest.SetGameConfigValue(t, f.client, "test_key_b", int64(34))
						},
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						f := newGameConfigFixture(t)
						tt.prepare(t, f)

						_, err := f.repo.GetInt64(t.Context(), "test_key_a")

						assert.ErrorIs(t, err, port.ErrNotFound)
					})
				}
			})
			t.Run("設定の値が整数として読めないとき", func(t *testing.T) {
				tests := []struct {
					name   string
					fields map[string]any
				}{
					{name: "値の項目が無いとき、値が整数として読めないことを示すエラーになる", fields: map[string]any{"other": int64(12)}},
					{name: "値がnullのとき、値が整数として読めないことを示すエラーになる", fields: map[string]any{"value": nil}},
					{name: "値が文字列「12」のとき、値が整数として読めないことを示すエラーになる", fields: map[string]any{"value": "12"}},
					{name: "値が小数1.5のとき、値が整数として読めないことを示すエラーになる", fields: map[string]any{"value": 1.5}},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						f := newGameConfigFixture(t)
						firestoretest.SetGameConfigFields(t, f.client, "test_key_a", tt.fields)

						_, err := f.repo.GetInt64(t.Context(), "test_key_a")

						assert.ErrorIs(t, err, port.ErrValueNotInteger)
					})
				}
			})
			t.Run("呼び出しが取り消し済みのとき、取り消しを示すエラーになる", func(t *testing.T) {
				f := newGameConfigFixture(t)
				firestoretest.SetGameConfigValue(t, f.client, "test_key_a", int64(12))
				ctx, cancel := context.WithCancel(t.Context())
				cancel()

				_, err := f.repo.GetInt64(ctx, "test_key_a")

				assert.ErrorIs(t, err, context.Canceled)
			})
			t.Run("取得がエラーになった後に、取り消されていない呼び出しで、同じキー「test_key_a」を取得すると", func(t *testing.T) {
				tests := []struct {
					name    string
					prepare func(t *testing.T, f *gameConfigFixture)
				}{
					{
						name: "設定の値が文字列「12」で整数として読めなかった後に、同じキーの設定が整数12に書き換わったとき、12が返る",
						prepare: func(t *testing.T, f *gameConfigFixture) {
							firestoretest.SetGameConfigValue(t, f.client, "test_key_a", "12")
							_, err := f.repo.GetInt64(t.Context(), "test_key_a")
							require.ErrorIs(t, err, port.ErrValueNotInteger)
							firestoretest.SetGameConfigValue(t, f.client, "test_key_a", int64(12))
						},
					},
					{
						name: "設定値が12のキーの取得が、呼び出しが取り消し済みでエラーになった後のとき、12が返る",
						prepare: func(t *testing.T, f *gameConfigFixture) {
							firestoretest.SetGameConfigValue(t, f.client, "test_key_a", int64(12))
							ctx, cancel := context.WithCancel(t.Context())
							cancel()
							_, err := f.repo.GetInt64(ctx, "test_key_a")
							require.ErrorIs(t, err, context.Canceled)
						},
					},
					{
						name: "設定が1件も登録されておらず見つからないエラーになった後に、同じキーの設定が整数12で登録されたとき、12が返る",
						prepare: func(t *testing.T, f *gameConfigFixture) {
							_, err := f.repo.GetInt64(t.Context(), "test_key_a")
							require.ErrorIs(t, err, port.ErrNotFound)
							firestoretest.SetGameConfigValue(t, f.client, "test_key_a", int64(12))
						},
					},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						f := newGameConfigFixture(t)
						tt.prepare(t, f)

						got, err := f.repo.GetInt64(t.Context(), "test_key_a")

						require.NoError(t, err)
						assert.Equal(t, int64(12), got)
					})
				}
			})
		})
	})
}
