package apiaccountclient_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
	"github.com/kenyamaneko/overload-party-account/packages/api-account/apiaccountclient"
	"github.com/kenyamaneko/overload-party-account/packages/api-account/apiaccountclient/apiaccountclienttest"
)

type redirectingDoer struct {
	target *url.URL
}

func (d redirectingDoer) Do(req *http.Request) (*http.Response, error) {
	redirected := req.Clone(req.Context())
	redirected.URL.Scheme = d.target.Scheme
	redirected.URL.Host = d.target.Host
	return http.DefaultClient.Do(redirected)
}

type failingDoer struct {
	err error
}

func (d failingDoer) Do(*http.Request) (*http.Response, error) {
	return nil, d.err
}

type contextKey struct{}

func newHeaderSettingEditor(name string, value string) apiaccount.RequestEditorFn {
	return func(_ context.Context, req *http.Request) error {
		req.Header.Set(name, value)
		return nil
	}
}

func newRawServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func newClosedServerURL(t *testing.T) string {
	t.Helper()
	server := httptest.NewServer(http.NotFoundHandler())
	closedURL := server.URL
	server.Close()
	return closedURL
}

func TestClientConnectionConfig(t *testing.T) {
	t.Run("[アカウントサービス連携]接続設定", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("基底URLのホストがaccount.invalidで、接続先をアカウントサービスに差し替えるHTTPクライアントを指定し、アカウントサービスがplayer_idがplayer-1のプレイヤー情報を返すとき、自分のプレイヤー情報の取得を行うと、返ったプレイヤー情報のplayer_idはplayer-1になる", func(t *testing.T) {
				server := apiaccountclienttest.NewFakeServer()
				t.Cleanup(server.Close)
				server.GetPlayerFn = func() (int, any) { return 200, apiaccountclienttest.NewDummyPlayer() }
				target, err := url.Parse(server.URL())
				require.NoError(t, err)
				client := apiaccountclienttest.NewClient(t, "http://account.invalid", apiaccountclient.WithHTTPClient(redirectingDoer{target: target}))

				got, err := client.GetPlayer(context.Background())

				require.NoError(t, err)
				assert.Equal(t, "player-1", got.PlayerID)
			})

			t.Run("送信前のリクエスト編集の指定", func(t *testing.T) {
				t.Run("送信前のリクエスト編集を指定しないとき、自分のプレイヤー情報の取得のリクエストにX-Test-Headerが付かずに届く", func(t *testing.T) {
					server := apiaccountclienttest.NewRecordingServer(t, 200, apiaccountclienttest.NewDummyPlayer())
					client := apiaccountclienttest.NewClient(t, server.URL)

					_, err := client.GetPlayer(context.Background())

					require.NoError(t, err)
					assert.Empty(t, server.Received.Get().Header.Values("X-Test-Header"))
				})

				t.Run("ヘッダーX-Test-Headerに値value-aを設定する編集を1つ指定したとき、自分のプレイヤー情報の取得のリクエストにX-Test-Header:value-aが付いて届く", func(t *testing.T) {
					server := apiaccountclienttest.NewRecordingServer(t, 200, apiaccountclienttest.NewDummyPlayer())
					client := apiaccountclienttest.NewClient(t, server.URL, apiaccountclient.WithRequestEditorFn(newHeaderSettingEditor("X-Test-Header", "value-a")))

					_, err := client.GetPlayer(context.Background())

					require.NoError(t, err)
					assert.Equal(t, []string{"value-a"}, server.Received.Get().Header.Values("X-Test-Header"))
				})

				t.Run("ヘッダーX-Test-Header-1に値value-aを設定する編集と、ヘッダーX-Test-Header-2に値value-bを設定する編集を指定して、自分のプレイヤー情報の取得を行うと", func(t *testing.T) {
					headerTests := []struct {
						name        string
						headerName  string
						headerValue string
					}{
						{name: "リクエストにX-Test-Header-1:value-aが付いて届く", headerName: "X-Test-Header-1", headerValue: "value-a"},
						{name: "リクエストにX-Test-Header-2:value-bが付いて届く", headerName: "X-Test-Header-2", headerValue: "value-b"},
					}
					for _, tc := range headerTests {
						t.Run(tc.name, func(t *testing.T) {
							server := apiaccountclienttest.NewRecordingServer(t, 200, apiaccountclienttest.NewDummyPlayer())
							client := apiaccountclienttest.NewClient(t, server.URL,
								apiaccountclient.WithRequestEditorFn(newHeaderSettingEditor("X-Test-Header-1", "value-a")),
								apiaccountclient.WithRequestEditorFn(newHeaderSettingEditor("X-Test-Header-2", "value-b")),
							)

							_, err := client.GetPlayer(context.Background())

							require.NoError(t, err)
							assert.Equal(t, []string{tc.headerValue}, server.Received.Get().Header.Values(tc.headerName))
						})
					}
				})

				t.Run("同じヘッダーX-Test-Headerにvalue-aを設定する編集、value-bを設定する編集の順に指定したとき、自分のプレイヤー情報の取得のリクエストのX-Test-Headerはvalue-bで届く", func(t *testing.T) {
					server := apiaccountclienttest.NewRecordingServer(t, 200, apiaccountclienttest.NewDummyPlayer())
					client := apiaccountclienttest.NewClient(t, server.URL,
						apiaccountclient.WithRequestEditorFn(newHeaderSettingEditor("X-Test-Header", "value-a")),
						apiaccountclient.WithRequestEditorFn(newHeaderSettingEditor("X-Test-Header", "value-b")),
					)

					_, err := client.GetPlayer(context.Background())

					require.NoError(t, err)
					assert.Equal(t, []string{"value-b"}, server.Received.Get().Header.Values("X-Test-Header"))
				})

				t.Run("呼び出しに付けた値がcontext-value-1のとき、その値の前にfrom-を付けた値をヘッダーX-Test-Headerに設定する編集を指定すると、自分のプレイヤー情報の取得のリクエストにX-Test-Header:from-context-value-1が付いて届く", func(t *testing.T) {
					server := apiaccountclienttest.NewRecordingServer(t, 200, apiaccountclienttest.NewDummyPlayer())
					editor := func(ctx context.Context, req *http.Request) error {
						req.Header.Set("X-Test-Header", "from-"+ctx.Value(contextKey{}).(string))
						return nil
					}
					client := apiaccountclienttest.NewClient(t, server.URL, apiaccountclient.WithRequestEditorFn(editor))
					ctx := context.WithValue(context.Background(), contextKey{}, "context-value-1")

					_, err := client.GetPlayer(ctx)

					require.NoError(t, err)
					assert.Equal(t, []string{"from-context-value-1"}, server.Received.Get().Header.Values("X-Test-Header"))
				})
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("クライアントを作成すると", func(t *testing.T) {
				invalidBaseURLTests := []struct {
					name    string
					baseURL string
				}{
					{name: "基底URLが%zzのとき、基底URLが不正であることを示すエラーになる", baseURL: "%zz"},
					{name: "基底URLにスキームが無い「／／localhost:8080」のとき、基底URLが不正であることを示すエラーになる", baseURL: "//localhost:8080"},
					{name: "基底URLにホストが無い「http:／／」のとき、基底URLが不正であることを示すエラーになる", baseURL: "http://"},
				}
				for _, tc := range invalidBaseURLTests {
					t.Run(tc.name, func(t *testing.T) {
						got, err := apiaccountclient.New(tc.baseURL)

						assert.ErrorIs(t, err, apiaccountclient.ErrInvalidBaseURL)
						assert.Nil(t, got)
					})
				}
			})

			t.Run("自分のプレイヤー情報の取得を行うと", func(t *testing.T) {
				failingComponentTests := []struct {
					name      string
					newOption func(cause error) apiaccountclient.Option
				}{
					{name: "指定したHTTPクライアントがエラーtest-component-failureを返したとき、エラーtest-component-failureが原因であると判別できるエラーを返す", newOption: func(cause error) apiaccountclient.Option {
						return apiaccountclient.WithHTTPClient(failingDoer{err: cause})
					}},
					{name: "送信前のリクエスト編集がエラーtest-component-failureを返したとき、エラーtest-component-failureが原因であると判別できるエラーを返す", newOption: func(cause error) apiaccountclient.Option {
						return apiaccountclient.WithRequestEditorFn(func(context.Context, *http.Request) error { return cause })
					}},
				}
				for _, tc := range failingComponentTests {
					t.Run(tc.name, func(t *testing.T) {
						cause := errors.New("test-component-failure")
						server := apiaccountclienttest.NewFakeServer()
						t.Cleanup(server.Close)
						client := apiaccountclienttest.NewClient(t, server.URL(), tc.newOption(cause))

						got, err := client.GetPlayer(context.Background())

						assert.ErrorIs(t, err, cause)
						assert.Nil(t, got)
					})
				}
			})
		})
	})
}

func TestClientCommunicationFailure(t *testing.T) {
	t.Run("[アカウントサービス連携]通信エラーの伝達", func(t *testing.T) {
		t.Run("異常系", func(t *testing.T) {
			t.Run("自分のプレイヤー情報の取得", func(t *testing.T) {
				t.Run("アカウントサービスとの接続に失敗したとき", func(t *testing.T) {
					connectionFailureTests := []struct {
						name       string
						newBaseURL func(t *testing.T) string
					}{
						{name: "接続できないとき、接続できないことを示すエラーになる", newBaseURL: newClosedServerURL},
						{name: "200を返し、本文の途中で接続が切れたとき、接続できないことを示すエラーになる", newBaseURL: func(t *testing.T) string {
							return newRawServer(t, func(w http.ResponseWriter, _ *http.Request) {
								w.Header().Set("Content-Type", "application/json")
								w.Header().Set("Content-Length", "1000")
								w.WriteHeader(200)
								_, _ = w.Write([]byte(`{"player_id":`))
							}).URL
						}},
					}
					for _, tc := range connectionFailureTests {
						t.Run(tc.name, func(t *testing.T) {
							client := apiaccountclienttest.NewClient(t, tc.newBaseURL(t))

							got, err := client.GetPlayer(context.Background())

							assert.ErrorIs(t, err, apiaccountclient.ErrConnectionFailed)
							assert.Nil(t, got)
						})
					}
				})

				t.Run("アカウントサービスが200を返し、本文がnot-jsonのとき、応答の本文が契約の形式でないことを示すエラーになる", func(t *testing.T) {
					server := newRawServer(t, func(w http.ResponseWriter, _ *http.Request) {
						w.Header().Set("Content-Type", "application/json")
						w.WriteHeader(200)
						_, _ = w.Write([]byte("not-json"))
					})
					client := apiaccountclienttest.NewClient(t, server.URL)

					got, err := client.GetPlayer(context.Background())

					assert.ErrorIs(t, err, apiaccountclient.ErrInvalidBody)
					assert.Nil(t, got)
				})

				t.Run("アカウントサービスが200を返し、Content-TypeがJSONとして読み取れないとき", func(t *testing.T) {
					contentTypeTests := []struct {
						name        string
						contentType string
					}{
						{name: "text／plainのとき、Content-Typeが契約の形式でないことを示すエラーになる", contentType: "text/plain"},
						{name: "application／json;charsetのとき、Content-Typeが契約の形式でないことを示すエラーになる", contentType: "application/json;charset"},
					}
					for _, tc := range contentTypeTests {
						t.Run(tc.name, func(t *testing.T) {
							server := newRawServer(t, func(w http.ResponseWriter, _ *http.Request) {
								w.Header().Set("Content-Type", tc.contentType)
								w.WriteHeader(200)
								_, _ = w.Write([]byte("{}"))
							})
							client := apiaccountclienttest.NewClient(t, server.URL)

							got, err := client.GetPlayer(context.Background())

							assert.ErrorIs(t, err, apiaccountclient.ErrInvalidContentType)
							assert.Nil(t, got)
						})
					}
				})
			})
		})
	})
}

func TestClientResponseStatusJudgement(t *testing.T) {
	allKnownErrors := []error{
		apiaccountclient.ErrBadRequest,
		apiaccountclient.ErrUnauthorized,
		apiaccountclient.ErrNotFound,
		apiaccountclient.ErrConflict,
		apiaccountclient.ErrInternalServer,
		apiaccountclient.ErrConnectionFailed,
		apiaccountclient.ErrInvalidBody,
		apiaccountclient.ErrInvalidContentType,
	}

	t.Run("[アカウントサービス連携]応答ステータスの判定", func(t *testing.T) {
		t.Run("異常系", func(t *testing.T) {
			t.Run("自分のプレイヤー情報の取得", func(t *testing.T) {
				t.Run("アカウントサービスが400を返したとき、不正なリクエストのエラーになる", func(t *testing.T) {
					client, server := apiaccountclienttest.NewFakeClient(t)
					server.GetPlayerFn = func() (int, any) { return 400, nil }

					got, err := client.GetPlayer(context.Background())

					assert.ErrorIs(t, err, apiaccountclient.ErrBadRequest)
					assert.Nil(t, got)
				})

				t.Run("アカウントサービスが401を返したとき、認証失敗のエラーになる", func(t *testing.T) {
					client, server := apiaccountclienttest.NewFakeClient(t)
					server.GetPlayerFn = func() (int, any) { return 401, nil }

					got, err := client.GetPlayer(context.Background())

					assert.ErrorIs(t, err, apiaccountclient.ErrUnauthorized)
					assert.Nil(t, got)
				})

				t.Run("アカウントサービスが404を返したとき、見つからないエラーになる", func(t *testing.T) {
					client, server := apiaccountclienttest.NewFakeClient(t)
					server.GetPlayerFn = func() (int, any) { return 404, nil }

					got, err := client.GetPlayer(context.Background())

					assert.ErrorIs(t, err, apiaccountclient.ErrNotFound)
					assert.Nil(t, got)
				})

				t.Run("アカウントサービスが409を返したとき、競合のエラーになる", func(t *testing.T) {
					client, server := apiaccountclienttest.NewFakeClient(t)
					server.GetPlayerFn = func() (int, any) { return 409, nil }

					got, err := client.GetPlayer(context.Background())

					assert.ErrorIs(t, err, apiaccountclient.ErrConflict)
					assert.Nil(t, got)
				})

				t.Run("アカウントサービスが500から599のステータスを返したとき", func(t *testing.T) {
					serverErrorStatusTests := []struct {
						name   string
						status int
					}{
						{name: "500のとき、サーバー内部エラーになる", status: 500},
						{name: "599のとき、サーバー内部エラーになる", status: 599},
					}
					for _, tc := range serverErrorStatusTests {
						t.Run(tc.name, func(t *testing.T) {
							client, server := apiaccountclienttest.NewFakeClient(t)
							server.GetPlayerFn = func() (int, any) { return tc.status, nil }

							got, err := client.GetPlayer(context.Background())

							assert.ErrorIs(t, err, apiaccountclient.ErrInternalServer)
							assert.Nil(t, got)
						})
					}
				})

				t.Run("アカウントサービスが想定外のステータスを返したとき", func(t *testing.T) {
					unexpectedStatusTests := []struct {
						name   string
						status int
					}{
						{name: "201のとき、想定外ステータスのエラーになる", status: 201},
						{name: "399のとき、想定外ステータスのエラーになる", status: 399},
						{name: "402のとき、想定外ステータスのエラーになる", status: 402},
						{name: "403のとき、想定外ステータスのエラーになる", status: 403},
						{name: "405のとき、想定外ステータスのエラーになる", status: 405},
						{name: "408のとき、想定外ステータスのエラーになる", status: 408},
						{name: "410のとき、想定外ステータスのエラーになる", status: 410},
						{name: "499のとき、想定外ステータスのエラーになる", status: 499},
						{name: "600のとき、想定外ステータスのエラーになる", status: 600},
					}
					for _, tc := range unexpectedStatusTests {
						t.Run(tc.name, func(t *testing.T) {
							client, server := apiaccountclienttest.NewFakeClient(t)
							server.GetPlayerFn = func() (int, any) { return tc.status, nil }

							got, err := client.GetPlayer(context.Background())

							require.Error(t, err)
							for _, known := range allKnownErrors {
								assert.NotErrorIs(t, err, known)
							}
							assert.Contains(t, err.Error(), strconv.Itoa(tc.status))
							assert.Nil(t, got)
						})
					}
				})

				t.Run("アカウントサービスが500を返した直後の呼び出しで200とplayer_idがplayer-1のプレイヤー情報を返したとき、返ったプレイヤー情報のplayer_idはplayer-1になる", func(t *testing.T) {
					client, server := apiaccountclienttest.NewFakeClient(t)
					responses := []struct {
						status int
						body   any
					}{{status: 500, body: nil}, {status: 200, body: apiaccountclienttest.NewDummyPlayer()}}
					var calls atomic.Int32
					server.GetPlayerFn = func() (int, any) {
						response := responses[calls.Add(1)-1]
						return response.status, response.body
					}
					_, firstErr := client.GetPlayer(context.Background())
					require.ErrorIs(t, firstErr, apiaccountclient.ErrInternalServer)

					got, err := client.GetPlayer(context.Background())

					require.NoError(t, err)
					assert.Equal(t, "player-1", got.PlayerID)
				})
			})

			t.Run("プレイヤー登録で、アカウントサービスが想定外のステータスとプレイヤー情報を返したとき", func(t *testing.T) {
				registerStatusTests := []struct {
					name   string
					status int
				}{
					{name: "200のとき、想定外ステータスのエラーになる", status: 200},
					{name: "202のとき、想定外ステータスのエラーになる", status: 202},
				}
				for _, tc := range registerStatusTests {
					t.Run(tc.name, func(t *testing.T) {
						client, server := apiaccountclienttest.NewFakeClient(t)
						server.RegisterFn = func(apiaccount.RegisterRequest) (int, any) { return tc.status, apiaccountclienttest.NewDummyPlayer() }

						got, err := client.RegisterPlayer(context.Background(), apiaccount.RegisterRequest{FirebaseUID: "firebase-uid-1"})

						require.Error(t, err)
						for _, known := range allKnownErrors {
							assert.NotErrorIs(t, err, known)
						}
						assert.Contains(t, err.Error(), strconv.Itoa(tc.status))
						assert.Nil(t, got)
					})
				}
			})

			t.Run("対戦結果による経験値付与で、アカウントサービスが想定外のステータスを返したとき", func(t *testing.T) {
				awardStatusTests := []struct {
					name   string
					status int
				}{
					{name: "203のとき、想定外ステータスのエラーになる", status: 203},
					{name: "205のとき、想定外ステータスのエラーになる", status: 205},
				}
				for _, tc := range awardStatusTests {
					t.Run(tc.name, func(t *testing.T) {
						client, server := apiaccountclienttest.NewFakeClient(t)
						server.AwardGameExpFn = func(apiaccount.AwardGameExpRequest) (int, any) { return tc.status, nil }

						err := client.AwardGameExp(context.Background(), apiaccount.AwardGameExpRequest{})

						require.Error(t, err)
						for _, known := range allKnownErrors {
							assert.NotErrorIs(t, err, known)
						}
						assert.Contains(t, err.Error(), strconv.Itoa(tc.status))
					})
				}
			})
		})
	})
}
