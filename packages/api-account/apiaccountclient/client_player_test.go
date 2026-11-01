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

func TestClientRegisterPlayer(t *testing.T) {
	t.Run("[アカウントサービス連携]プレイヤー登録", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("アカウントサービスが201とplayer_idがplayer-1のプレイヤー情報を返したとき、返ったプレイヤー情報のplayer_idはplayer-1になる", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.RegisterFn = func(apiaccount.RegisterRequest) (int, any) { return 201, apiaccountclienttest.NewDummyPlayer() }

				got, err := client.RegisterPlayer(context.Background(), apiaccount.RegisterRequest{FirebaseUID: "firebase-uid-1"})

				require.NoError(t, err)
				assert.Equal(t, "player-1", got.PlayerID)
			})

			t.Run("FirebaseUIDがfirebase-uid-1のとき、アカウントサービスにFirebaseUIDがfirebase-uid-1の登録リクエストが届く", func(t *testing.T) {
				var received apiaccountclienttest.Captured[apiaccount.RegisterRequest]
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.RegisterFn = func(req apiaccount.RegisterRequest) (int, any) {
					received.Set(req)
					return 201, apiaccountclienttest.NewDummyPlayer()
				}

				_, err := client.RegisterPlayer(context.Background(), apiaccount.RegisterRequest{FirebaseUID: "firebase-uid-1"})

				require.NoError(t, err)
				assert.Equal(t, apiaccount.RegisterRequest{FirebaseUID: "firebase-uid-1"}, received.Get())
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("アカウントサービスが409を返したとき、競合のエラーになる", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.RegisterFn = func(apiaccount.RegisterRequest) (int, any) { return 409, nil }

				got, err := client.RegisterPlayer(context.Background(), apiaccount.RegisterRequest{FirebaseUID: "firebase-uid-1"})

				assert.ErrorIs(t, err, apiaccountclient.ErrConflict)
				assert.Nil(t, got)
			})

			t.Run("呼び出しが取り消し済みのとき、取り消しを示すエラーになる", func(t *testing.T) {
				client, _ := apiaccountclienttest.NewFakeClient(t)

				got, err := client.RegisterPlayer(apiaccountclienttest.NewCanceledContext(), apiaccount.RegisterRequest{FirebaseUID: "firebase-uid-1"})

				assert.ErrorIs(t, err, context.Canceled)
				assert.Nil(t, got)
			})
		})
	})
}

func TestClientLoginPlayer(t *testing.T) {
	t.Run("[アカウントサービス連携]ログイン", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("アカウントサービスが200とplayer_idがplayer-1のプレイヤー情報を返したとき、返ったプレイヤー情報のplayer_idはplayer-1になる", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.LoginFn = func(apiaccount.LoginRequest) (int, any) { return 200, apiaccountclienttest.NewDummyPlayer() }

				got, err := client.LoginPlayer(context.Background(), apiaccount.LoginRequest{FirebaseUID: "firebase-uid-1"})

				require.NoError(t, err)
				assert.Equal(t, "player-1", got.PlayerID)
			})

			t.Run("FirebaseUIDがfirebase-uid-1のとき、アカウントサービスにFirebaseUIDがfirebase-uid-1のログインリクエストが届く", func(t *testing.T) {
				var received apiaccountclienttest.Captured[apiaccount.LoginRequest]
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.LoginFn = func(req apiaccount.LoginRequest) (int, any) {
					received.Set(req)
					return 200, apiaccountclienttest.NewDummyPlayer()
				}

				_, err := client.LoginPlayer(context.Background(), apiaccount.LoginRequest{FirebaseUID: "firebase-uid-1"})

				require.NoError(t, err)
				assert.Equal(t, apiaccount.LoginRequest{FirebaseUID: "firebase-uid-1"}, received.Get())
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("アカウントサービスが404を返したとき、見つからないエラーになる", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.LoginFn = func(apiaccount.LoginRequest) (int, any) { return 404, nil }

				got, err := client.LoginPlayer(context.Background(), apiaccount.LoginRequest{FirebaseUID: "firebase-uid-1"})

				assert.ErrorIs(t, err, apiaccountclient.ErrNotFound)
				assert.Nil(t, got)
			})

			t.Run("呼び出しが取り消し済みのとき、取り消しを示すエラーになる", func(t *testing.T) {
				client, _ := apiaccountclienttest.NewFakeClient(t)

				got, err := client.LoginPlayer(apiaccountclienttest.NewCanceledContext(), apiaccount.LoginRequest{FirebaseUID: "firebase-uid-1"})

				assert.ErrorIs(t, err, context.Canceled)
				assert.Nil(t, got)
			})
		})
	})
}

func TestClientGetPlayerByFirebaseUID(t *testing.T) {
	t.Run("[アカウントサービス連携]FirebaseUIDによるプレイヤー検索", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("アカウントサービスが200とplayer_idがplayer-1のプレイヤー情報を返したとき、返ったプレイヤー情報のplayer_idはplayer-1になる", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.GetPlayerByFirebaseUIDFn = func(string) (int, any) { return 200, apiaccountclienttest.NewDummyPlayer() }

				got, err := client.GetPlayerByFirebaseUID(context.Background(), "firebase-uid-1")

				require.NoError(t, err)
				assert.Equal(t, "player-1", got.PlayerID)
			})

			t.Run("FirebaseUIDがfirebase-uid-1のとき、アカウントサービスにFirebaseUIDがfirebase-uid-1の検索リクエストが届く", func(t *testing.T) {
				var received apiaccountclienttest.Captured[string]
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.GetPlayerByFirebaseUIDFn = func(firebaseUID string) (int, any) {
					received.Set(firebaseUID)
					return 200, apiaccountclienttest.NewDummyPlayer()
				}

				_, err := client.GetPlayerByFirebaseUID(context.Background(), "firebase-uid-1")

				require.NoError(t, err)
				assert.Equal(t, "firebase-uid-1", received.Get())
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("アカウントサービスが404を返したとき、見つからないエラーになる", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.GetPlayerByFirebaseUIDFn = func(string) (int, any) { return 404, nil }

				got, err := client.GetPlayerByFirebaseUID(context.Background(), "firebase-uid-1")

				assert.ErrorIs(t, err, apiaccountclient.ErrNotFound)
				assert.Nil(t, got)
			})

			t.Run("呼び出しが取り消し済みのとき、取り消しを示すエラーになる", func(t *testing.T) {
				client, _ := apiaccountclienttest.NewFakeClient(t)

				got, err := client.GetPlayerByFirebaseUID(apiaccountclienttest.NewCanceledContext(), "firebase-uid-1")

				assert.ErrorIs(t, err, context.Canceled)
				assert.Nil(t, got)
			})
		})
	})
}

func TestClientGetPlayer(t *testing.T) {
	t.Run("[アカウントサービス連携]自分のプレイヤー情報の取得", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("アカウントサービスが200とplayer_idがplayer-1のプレイヤー情報を返したとき、返ったプレイヤー情報のplayer_idはplayer-1になる", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.GetPlayerFn = func() (int, any) { return 200, apiaccountclienttest.NewDummyPlayer() }

				got, err := client.GetPlayer(context.Background())

				require.NoError(t, err)
				assert.Equal(t, "player-1", got.PlayerID)
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("呼び出しが取り消し済みのとき、取り消しを示すエラーになる", func(t *testing.T) {
				client, _ := apiaccountclienttest.NewFakeClient(t)

				got, err := client.GetPlayer(apiaccountclienttest.NewCanceledContext())

				assert.ErrorIs(t, err, context.Canceled)
				assert.Nil(t, got)
			})
		})
	})
}

func TestClientValidateNameForOnboarding(t *testing.T) {
	t.Run("[アカウントサービス連携]オンボーディングでの表示名の検証", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("アカウントサービスが204を返したとき、エラーなしで完了する", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.ValidateNameForOnboardingFn = func(apiaccount.ValidateNameForOnboardingRequest) (int, any) { return 204, nil }

				err := client.ValidateNameForOnboarding(context.Background(), apiaccount.ValidateNameForOnboardingRequest{Name: "テスト太郎"})

				assert.NoError(t, err)
			})

			t.Run("表示名が「テスト太郎」のとき、アカウントサービスに表示名「テスト太郎」の検証リクエストが届く", func(t *testing.T) {
				var received apiaccountclienttest.Captured[apiaccount.ValidateNameForOnboardingRequest]
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.ValidateNameForOnboardingFn = func(req apiaccount.ValidateNameForOnboardingRequest) (int, any) {
					received.Set(req)
					return 204, nil
				}

				err := client.ValidateNameForOnboarding(context.Background(), apiaccount.ValidateNameForOnboardingRequest{Name: "テスト太郎"})

				require.NoError(t, err)
				assert.Equal(t, apiaccount.ValidateNameForOnboardingRequest{Name: "テスト太郎"}, received.Get())
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("アカウントサービスが400を返したとき、不正なリクエストのエラーになる", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.ValidateNameForOnboardingFn = func(apiaccount.ValidateNameForOnboardingRequest) (int, any) { return 400, nil }

				err := client.ValidateNameForOnboarding(context.Background(), apiaccount.ValidateNameForOnboardingRequest{Name: "テスト太郎"})

				assert.ErrorIs(t, err, apiaccountclient.ErrBadRequest)
			})

			t.Run("呼び出しが取り消し済みのとき、取り消しを示すエラーになる", func(t *testing.T) {
				client, _ := apiaccountclienttest.NewFakeClient(t)

				err := client.ValidateNameForOnboarding(apiaccountclienttest.NewCanceledContext(), apiaccount.ValidateNameForOnboardingRequest{Name: "テスト太郎"})

				assert.ErrorIs(t, err, context.Canceled)
			})
		})
	})
}
