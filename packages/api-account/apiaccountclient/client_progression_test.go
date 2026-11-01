package apiaccountclient_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
	"github.com/kenyamaneko/overload-party-account/packages/api-account/apiaccountclient"
	"github.com/kenyamaneko/overload-party-account/packages/api-account/apiaccountclient/apiaccountclienttest"
)

func TestClientListFactions(t *testing.T) {
	t.Run("[アカウントサービス連携]陣営一覧の取得", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("アカウントサービスが陣営0件の一覧を返したとき、空の一覧が返る", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.ListFactionsFn = func() (int, any) { return 200, apiaccount.FactionListing{Factions: []string{}} }

				got, err := client.ListFactions(context.Background())

				require.NoError(t, err)
				require.NotNil(t, got)
				assert.Empty(t, got.Factions)
			})

			t.Run("アカウントサービスが陣営faction-aの1件の一覧を返したとき、faction-aだけの一覧が返る", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.ListFactionsFn = func() (int, any) {
					return 200, apiaccount.FactionListing{Factions: []string{"faction-a"}}
				}

				got, err := client.ListFactions(context.Background())

				require.NoError(t, err)
				assert.Equal(t, &apiaccount.FactionListing{Factions: []string{"faction-a"}}, got)
			})

			t.Run("アカウントサービスが陣営faction-aとfaction-bの2件の一覧を返したとき、faction-aとfaction-bをその順に含む一覧が返る", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.ListFactionsFn = func() (int, any) {
					return 200, apiaccount.FactionListing{Factions: []string{"faction-a", "faction-b"}}
				}

				got, err := client.ListFactions(context.Background())

				require.NoError(t, err)
				assert.Equal(t, &apiaccount.FactionListing{Factions: []string{"faction-a", "faction-b"}}, got)
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("アカウントサービスが404を返したとき、見つからないエラーになる", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.ListFactionsFn = func() (int, any) { return 404, nil }

				got, err := client.ListFactions(context.Background())

				assert.ErrorIs(t, err, apiaccountclient.ErrNotFound)
				assert.Nil(t, got)
			})

			t.Run("呼び出しが取り消し済みのとき、取り消しを示すエラーになる", func(t *testing.T) {
				client, _ := apiaccountclienttest.NewFakeClient(t)

				got, err := client.ListFactions(apiaccountclienttest.NewCanceledContext())

				assert.ErrorIs(t, err, context.Canceled)
				assert.Nil(t, got)
			})
		})
	})
}
