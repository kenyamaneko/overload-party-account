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

func TestClientGetBattleLimit(t *testing.T) {
	t.Run("[アカウントサービス連携]バトル回数制限の取得", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("アカウントサービスが当日の回数3のバトル回数制限を返したとき、返ったバトル回数制限の当日の回数は3になる", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.GetBattleLimitFn = func() (int, any) {
					return 200, apiaccount.BattleLimitResponse{CanBattle: true, DailyBattleCount: 3, DailyBattleLimit: 5}
				}

				got, err := client.GetBattleLimit(context.Background())

				require.NoError(t, err)
				assert.Equal(t, int64(3), got.DailyBattleCount)
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("アカウントサービスが404を返したとき、見つからないエラーになる", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.GetBattleLimitFn = func() (int, any) { return 404, nil }

				got, err := client.GetBattleLimit(context.Background())

				assert.ErrorIs(t, err, apiaccountclient.ErrNotFound)
				assert.Nil(t, got)
			})

			t.Run("呼び出しが取り消し済みのとき、取り消しを示すエラーになる", func(t *testing.T) {
				client, _ := apiaccountclienttest.NewFakeClient(t)

				got, err := client.GetBattleLimit(apiaccountclienttest.NewCanceledContext())

				assert.ErrorIs(t, err, context.Canceled)
				assert.Nil(t, got)
			})
		})
	})
}

func TestClientIncrementBattleCount(t *testing.T) {
	t.Run("[アカウントサービス連携]バトル回数の加算", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("アカウントサービスが204を返したとき、エラーなしで完了する", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.IncrementBattleCountFn = func() (int, any) { return 204, nil }

				err := client.IncrementBattleCount(context.Background())

				assert.NoError(t, err)
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("アカウントサービスが404を返したとき、見つからないエラーになる", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.IncrementBattleCountFn = func() (int, any) { return 404, nil }

				err := client.IncrementBattleCount(context.Background())

				assert.ErrorIs(t, err, apiaccountclient.ErrNotFound)
			})

			t.Run("呼び出しが取り消し済みのとき、取り消しを示すエラーになる", func(t *testing.T) {
				client, _ := apiaccountclienttest.NewFakeClient(t)

				err := client.IncrementBattleCount(apiaccountclienttest.NewCanceledContext())

				assert.ErrorIs(t, err, context.Canceled)
			})
		})
	})
}

func TestClientRevertBattleCount(t *testing.T) {
	revertRequest := apiaccount.RevertBattleCountRequest{
		GameID:           "game-1",
		Player1ID:        "player-1",
		Player2ID:        "player-2",
		ConsumedAtMillis: 1700000000000,
	}

	t.Run("[アカウントサービス連携]停止で無効になった対戦の消費バトル回数の返却", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("アカウントサービスが204を返したとき、エラーなしで完了する", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.RevertBattleCountFn = func(apiaccount.RevertBattleCountRequest) (int, any) { return 204, nil }

				err := client.RevertBattleCount(context.Background(), revertRequest)

				assert.NoError(t, err)
			})

			receivedFieldTests := []struct {
				name        string
				assertField func(t *testing.T, got apiaccount.RevertBattleCountRequest)
			}{
				{name: "対戦IDがgame-1のとき、アカウントサービスに対戦IDgame-1が届く", assertField: func(t *testing.T, got apiaccount.RevertBattleCountRequest) {
					assert.Equal(t, "game-1", got.GameID)
				}},
				{name: "1人目のプレイヤーIDがplayer-1のとき、アカウントサービスに1人目のプレイヤーIDplayer-1が届く", assertField: func(t *testing.T, got apiaccount.RevertBattleCountRequest) {
					assert.Equal(t, "player-1", got.Player1ID)
				}},
				{name: "2人目のプレイヤーIDがplayer-2のとき、アカウントサービスに2人目のプレイヤーIDplayer-2が届く", assertField: func(t *testing.T, got apiaccount.RevertBattleCountRequest) {
					assert.Equal(t, "player-2", got.Player2ID)
				}},
				{name: "消費時刻が1700000000000ミリ秒のとき、アカウントサービスに消費時刻1700000000000ミリ秒が届く", assertField: func(t *testing.T, got apiaccount.RevertBattleCountRequest) {
					assert.Equal(t, int64(1700000000000), got.ConsumedAtMillis)
				}},
			}
			for _, tc := range receivedFieldTests {
				t.Run(tc.name, func(t *testing.T) {
					var received apiaccountclienttest.Captured[apiaccount.RevertBattleCountRequest]
					client, server := apiaccountclienttest.NewFakeClient(t)
					server.RevertBattleCountFn = func(req apiaccount.RevertBattleCountRequest) (int, any) {
						received.Set(req)
						return 204, nil
					}

					err := client.RevertBattleCount(context.Background(), revertRequest)

					require.NoError(t, err)
					tc.assertField(t, received.Get())
				})
			}
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("アカウントサービスが400を返したとき、不正なリクエストのエラーになる", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.RevertBattleCountFn = func(apiaccount.RevertBattleCountRequest) (int, any) { return 400, nil }

				err := client.RevertBattleCount(context.Background(), revertRequest)

				assert.ErrorIs(t, err, apiaccountclient.ErrBadRequest)
			})

			t.Run("呼び出しが取り消し済みのとき、取り消しを示すエラーになる", func(t *testing.T) {
				client, _ := apiaccountclienttest.NewFakeClient(t)

				err := client.RevertBattleCount(apiaccountclienttest.NewCanceledContext(), revertRequest)

				assert.ErrorIs(t, err, context.Canceled)
			})
		})
	})
}

func TestClientAwardGameExp(t *testing.T) {
	awardRequest := apiaccount.AwardGameExpRequest{
		Player1ID: "player-1",
		Player2ID: "player-2",
		WinnerNum: 1,
		Reason:    "reason-a",
		MatchType: "match-type-a",
	}

	t.Run("[アカウントサービス連携]対戦結果による経験値付与", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("アカウントサービスが204を返したとき、エラーなしで完了する", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.AwardGameExpFn = func(apiaccount.AwardGameExpRequest) (int, any) { return 204, nil }

				err := client.AwardGameExp(context.Background(), awardRequest)

				assert.NoError(t, err)
			})

			receivedFieldTests := []struct {
				name        string
				assertField func(t *testing.T, got apiaccount.AwardGameExpRequest)
			}{
				{name: "1人目のプレイヤーIDがplayer-1のとき、アカウントサービスに1人目のプレイヤーIDplayer-1が届く", assertField: func(t *testing.T, got apiaccount.AwardGameExpRequest) {
					assert.Equal(t, "player-1", got.Player1ID)
				}},
				{name: "2人目のプレイヤーIDがplayer-2のとき、アカウントサービスに2人目のプレイヤーIDplayer-2が届く", assertField: func(t *testing.T, got apiaccount.AwardGameExpRequest) {
					assert.Equal(t, "player-2", got.Player2ID)
				}},
				{name: "勝者番号が1のとき、アカウントサービスに勝者番号1が届く", assertField: func(t *testing.T, got apiaccount.AwardGameExpRequest) {
					assert.EqualValues(t, 1, got.WinnerNum)
				}},
				{name: "理由がreason-aのとき、アカウントサービスに理由reason-aが届く", assertField: func(t *testing.T, got apiaccount.AwardGameExpRequest) {
					assert.Equal(t, "reason-a", got.Reason)
				}},
				{name: "対戦種別がmatch-type-aのとき、アカウントサービスに対戦種別match-type-aが届く", assertField: func(t *testing.T, got apiaccount.AwardGameExpRequest) {
					assert.Equal(t, "match-type-a", got.MatchType)
				}},
			}
			for _, tc := range receivedFieldTests {
				t.Run(tc.name, func(t *testing.T) {
					var received apiaccountclienttest.Captured[apiaccount.AwardGameExpRequest]
					client, server := apiaccountclienttest.NewFakeClient(t)
					server.AwardGameExpFn = func(req apiaccount.AwardGameExpRequest) (int, any) {
						received.Set(req)
						return 204, nil
					}

					err := client.AwardGameExp(context.Background(), awardRequest)

					require.NoError(t, err)
					tc.assertField(t, received.Get())
				})
			}
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("アカウントサービスが404を返したとき、見つからないエラーになる", func(t *testing.T) {
				client, server := apiaccountclienttest.NewFakeClient(t)
				server.AwardGameExpFn = func(apiaccount.AwardGameExpRequest) (int, any) { return 404, nil }

				err := client.AwardGameExp(context.Background(), awardRequest)

				assert.ErrorIs(t, err, apiaccountclient.ErrNotFound)
			})

			t.Run("呼び出しが取り消し済みのとき、取り消しを示すエラーになる", func(t *testing.T) {
				client, _ := apiaccountclienttest.NewFakeClient(t)

				err := client.AwardGameExp(apiaccountclienttest.NewCanceledContext(), awardRequest)

				assert.ErrorIs(t, err, context.Canceled)
			})
		})
	})
}
