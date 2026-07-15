package apiaccountclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
	"github.com/kenyamaneko/overload-party-account/packages/api-account/apiaccountclient"
	"github.com/kenyamaneko/overload-party-account/packages/api-account/apiaccountserverfake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_RegisterPlayer(t *testing.T) {
	t.Run("RegisterPlayer", func(t *testing.T) {
		cases := []struct {
			name       string
			status     int
			wantTarget error
		}{
			{
				name:       "400 を受けたとき、ErrBadRequest になる",
				status:     http.StatusBadRequest,
				wantTarget: apiaccountclient.ErrBadRequest,
			},
			{
				name:       "409 を受けたとき、ErrConflict になる",
				status:     http.StatusConflict,
				wantTarget: apiaccountclient.ErrConflict,
			},
			{
				name:       "500 を受けたとき、ErrInternalServer になる",
				status:     http.StatusInternalServerError,
				wantTarget: apiaccountclient.ErrInternalServer,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				srv := apiaccountserverfake.NewServer()
				defer srv.Close()
				srv.RegisterFn = func(_ apiaccount.RegisterRequest) (int, any) { return tc.status, nil }

				c := newTestClient(t, srv.URL())
				_, err := c.RegisterPlayer(context.Background(), apiaccount.RegisterRequest{})
				assertSentinel(t, err, tc.wantTarget)
			})
		}
	})
}

func TestClient_LoginPlayer(t *testing.T) {
	t.Run("LoginPlayer", func(t *testing.T) {
		cases := []struct {
			name       string
			status     int
			wantTarget error
		}{
			{
				name:       "400 を受けたとき、ErrBadRequest になる",
				status:     http.StatusBadRequest,
				wantTarget: apiaccountclient.ErrBadRequest,
			},
			{
				name:       "404 を受けたとき、ErrNotFound になる",
				status:     http.StatusNotFound,
				wantTarget: apiaccountclient.ErrNotFound,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				srv := apiaccountserverfake.NewServer()
				defer srv.Close()
				srv.LoginFn = func(_ apiaccount.LoginRequest) (int, any) { return tc.status, nil }

				c := newTestClient(t, srv.URL())
				_, err := c.LoginPlayer(context.Background(), apiaccount.LoginRequest{})
				assertSentinel(t, err, tc.wantTarget)
			})
		}
	})
}

func TestClient_GetPlayerByFirebaseUID(t *testing.T) {
	t.Run("GetPlayerByFirebaseUID", func(t *testing.T) {
		t.Run("404 を受けたとき、ErrNotFound になる", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.FindByFirebaseUIDFn = func(_ string) (int, any) { return http.StatusNotFound, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.GetPlayerByFirebaseUID(context.Background(), "uid")
			assertSentinel(t, err, apiaccountclient.ErrNotFound)
		})
	})
}

func TestClient_AwardGameExp(t *testing.T) {
	t.Run("AwardGameExp", func(t *testing.T) {
		t.Run("204 を受けたとき、エラーにならない", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.AwardGameExpFn = func(_ apiaccount.AwardGameExpRequest) (int, any) { return http.StatusNoContent, nil }

			c := newTestClient(t, srv.URL())
			err := c.AwardGameExp(context.Background(), apiaccount.AwardGameExpRequest{})
			require.NoError(t, err)
		})

		t.Run("400 を受けたとき、ErrBadRequest になる", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.AwardGameExpFn = func(_ apiaccount.AwardGameExpRequest) (int, any) { return http.StatusBadRequest, nil }

			c := newTestClient(t, srv.URL())
			err := c.AwardGameExp(context.Background(), apiaccount.AwardGameExpRequest{})
			assertSentinel(t, err, apiaccountclient.ErrBadRequest)
		})
	})
}

func TestClient_GetPlayer(t *testing.T) {
	t.Run("GetPlayer", func(t *testing.T) {
		t.Run("401 を受けたとき、ErrUnauthorized になる", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.GetPlayerFn = func() (int, any) { return http.StatusUnauthorized, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.GetPlayer(context.Background())
			assertSentinel(t, err, apiaccountclient.ErrUnauthorized)
		})
	})
}

func TestClient_UpdateName(t *testing.T) {
	t.Run("UpdateName", func(t *testing.T) {
		cases := []struct {
			name       string
			status     int
			wantTarget error
		}{
			{
				name:       "400 を受けたとき、ErrBadRequest になる",
				status:     http.StatusBadRequest,
				wantTarget: apiaccountclient.ErrBadRequest,
			},
			{
				name:       "401 を受けたとき、ErrUnauthorized になる",
				status:     http.StatusUnauthorized,
				wantTarget: apiaccountclient.ErrUnauthorized,
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				srv := apiaccountserverfake.NewServer()
				defer srv.Close()
				srv.UpdateNameFn = func(_ apiaccount.UpdateNameRequest) (int, any) { return tc.status, nil }

				c := newTestClient(t, srv.URL())
				_, err := c.UpdateName(context.Background(), apiaccount.UpdateNameRequest{})
				assertSentinel(t, err, tc.wantTarget)
			})
		}
	})
}

func TestClient_ValidateNameForOnboarding(t *testing.T) {
	t.Run("ValidateNameForOnboarding", func(t *testing.T) {
		t.Run("204 を受けたとき、エラーにならない", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.ValidateNameForOnboardingFn = func(_ apiaccount.ValidateNameForOnboardingRequest) (int, any) {
				return http.StatusNoContent, nil
			}

			c := newTestClient(t, srv.URL())
			err := c.ValidateNameForOnboarding(context.Background(), apiaccount.ValidateNameForOnboardingRequest{})
			require.NoError(t, err)
		})

		t.Run("400 を受けたとき、ErrBadRequest になる", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.ValidateNameForOnboardingFn = func(_ apiaccount.ValidateNameForOnboardingRequest) (int, any) { return http.StatusBadRequest, nil }

			c := newTestClient(t, srv.URL())
			err := c.ValidateNameForOnboarding(context.Background(), apiaccount.ValidateNameForOnboardingRequest{})
			assertSentinel(t, err, apiaccountclient.ErrBadRequest)
		})
	})
}

func TestClient_GetBattleLimit(t *testing.T) {
	t.Run("GetBattleLimit", func(t *testing.T) {
		t.Run("401 を受けたとき、ErrUnauthorized になる", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.GetBattleLimitFn = func() (int, any) { return http.StatusUnauthorized, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.GetBattleLimit(context.Background())
			assertSentinel(t, err, apiaccountclient.ErrUnauthorized)
		})
	})
}

func TestClient_IncrementBattleCount(t *testing.T) {
	t.Run("IncrementBattleCount", func(t *testing.T) {
		t.Run("204 を受けたとき、エラーにならない", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.IncrementBattleCountFn = func() (int, any) { return http.StatusNoContent, nil }

			c := newTestClient(t, srv.URL())
			err := c.IncrementBattleCount(context.Background())
			require.NoError(t, err)
		})

		t.Run("401 を受けたとき、ErrUnauthorized になる", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.IncrementBattleCountFn = func() (int, any) { return http.StatusUnauthorized, nil }

			c := newTestClient(t, srv.URL())
			err := c.IncrementBattleCount(context.Background())
			assertSentinel(t, err, apiaccountclient.ErrUnauthorized)
		})
	})
}

func TestClient_UpdatePremium(t *testing.T) {
	t.Run("UpdatePremium", func(t *testing.T) {
		t.Run("204 を受けたとき、エラーにならない", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.UpdatePremiumFn = func(_ apiaccount.UpdatePremiumRequest) (int, any) { return http.StatusNoContent, nil }

			c := newTestClient(t, srv.URL())
			err := c.UpdatePremium(context.Background(), apiaccount.UpdatePremiumRequest{})
			require.NoError(t, err)
		})

		t.Run("400 を受けたとき、ErrBadRequest になる", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.UpdatePremiumFn = func(_ apiaccount.UpdatePremiumRequest) (int, any) { return http.StatusBadRequest, nil }

			c := newTestClient(t, srv.URL())
			err := c.UpdatePremium(context.Background(), apiaccount.UpdatePremiumRequest{})
			assertSentinel(t, err, apiaccountclient.ErrBadRequest)
		})
	})
}

func TestClient_AddExp(t *testing.T) {
	t.Run("AddExp", func(t *testing.T) {
		t.Run("204 を受けたとき、エラーにならない", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.AddExpFn = func(_ apiaccount.AddExpRequest) (int, any) { return http.StatusNoContent, nil }

			c := newTestClient(t, srv.URL())
			err := c.AddExp(context.Background(), apiaccount.AddExpRequest{})
			require.NoError(t, err)
		})

		t.Run("400 を受けたとき、ErrBadRequest になる", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.AddExpFn = func(_ apiaccount.AddExpRequest) (int, any) { return http.StatusBadRequest, nil }

			c := newTestClient(t, srv.URL())
			err := c.AddExp(context.Background(), apiaccount.AddExpRequest{})
			assertSentinel(t, err, apiaccountclient.ErrBadRequest)
		})
	})
}

func TestClient_ListFactions(t *testing.T) {
	t.Run("ListFactions", func(t *testing.T) {
		t.Run("401 を受けたとき、ErrUnauthorized になる", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.ListFactionsFn = func() (int, any) { return http.StatusUnauthorized, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.ListFactions(context.Background())
			assertSentinel(t, err, apiaccountclient.ErrUnauthorized)
		})
	})
}

func TestClient_GrantFaction(t *testing.T) {
	t.Run("GrantFaction", func(t *testing.T) {
		t.Run("204 を受けたとき、エラーにならない", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.GrantFactionFn = func(_ apiaccount.FactionGrantRequest) (int, any) { return http.StatusNoContent, nil }

			c := newTestClient(t, srv.URL())
			err := c.GrantFaction(context.Background(), apiaccount.FactionGrantRequest{})
			require.NoError(t, err)
		})

		t.Run("400 を受けたとき、ErrBadRequest になる", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.GrantFactionFn = func(_ apiaccount.FactionGrantRequest) (int, any) { return http.StatusBadRequest, nil }

			c := newTestClient(t, srv.URL())
			err := c.GrantFaction(context.Background(), apiaccount.FactionGrantRequest{})
			assertSentinel(t, err, apiaccountclient.ErrBadRequest)
		})
	})
}

func TestClient_SelectInitialFaction(t *testing.T) {
	t.Run("SelectInitialFaction", func(t *testing.T) {
		t.Run("200 を受けたとき、エラーにならない", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.SelectInitialFactionFn = func(_ apiaccount.SelectInitialFactionRequest) (int, any) { return http.StatusOK, nil }

			c := newTestClient(t, srv.URL())
			err := c.SelectInitialFaction(context.Background(), apiaccount.SelectInitialFactionRequest{})
			require.NoError(t, err)
		})

		t.Run("400 を受けたとき、ErrBadRequest になる", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.SelectInitialFactionFn = func(_ apiaccount.SelectInitialFactionRequest) (int, any) { return http.StatusBadRequest, nil }

			c := newTestClient(t, srv.URL())
			err := c.SelectInitialFaction(context.Background(), apiaccount.SelectInitialFactionRequest{})
			assertSentinel(t, err, apiaccountclient.ErrBadRequest)
		})
	})
}

func TestClient_GetPlayerSettings(t *testing.T) {
	t.Run("GetPlayerSettings", func(t *testing.T) {
		t.Run("401 を受けたとき、ErrUnauthorized になる", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.GetSettingsFn = func() (int, any) { return http.StatusUnauthorized, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.GetPlayerSettings(context.Background())
			assertSentinel(t, err, apiaccountclient.ErrUnauthorized)
		})
	})
}

func TestClient_UpdatePlayerSettings(t *testing.T) {
	t.Run("UpdatePlayerSettings", func(t *testing.T) {
		t.Run("400 を受けたとき、ErrBadRequest になる", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.UpdateSettingsFn = func(_ apiaccount.UpdateSettingsRequest) (int, any) { return http.StatusBadRequest, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.UpdatePlayerSettings(context.Background(), apiaccount.UpdateSettingsRequest{})
			assertSentinel(t, err, apiaccountclient.ErrBadRequest)
		})
	})
}

func TestClient_RequestEditor(t *testing.T) {
	t.Run("リクエストエディタの適用", func(t *testing.T) {
		t.Run("設定したヘッダが送信先の全リクエストに付与される", func(t *testing.T) {
			// X-Internal-Auth header 注入の接続点として SDK が機能することを担保する。
			var gotHeader string
			spy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotHeader = r.Header.Get("X-Internal-Auth")
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"factions":[]}`))
			}))
			defer spy.Close()

			c, err := apiaccountclient.New(spy.URL,
				apiaccountclient.WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
					req.Header.Set("X-Internal-Auth", "test-token")
					return nil
				}),
			)
			require.NoError(t, err)

			_, err = c.ListFactions(context.Background())
			require.NoError(t, err)
			assert.Equal(t, "test-token", gotHeader)
		})
	})
}

func newTestClient(t *testing.T, baseURL string) *apiaccountclient.Client {
	t.Helper()
	c, err := apiaccountclient.New(baseURL)
	require.NoError(t, err)
	return c
}

func assertSentinel(t *testing.T, gotErr, wantTarget error) {
	t.Helper()
	require.Error(t, gotErr)
	assert.ErrorIs(t, gotErr, wantTarget)
}
