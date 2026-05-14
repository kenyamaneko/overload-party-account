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

// 以下の TestClient_<Endpoint>_StatusMapping 群は、SDK の固有責務である
// 「OpenAPI spec で宣言された 4xx/5xx status を sentinel error に変換する」契約を
// endpoint ごとに検証する。各テスト関数は data/openapi.yaml が宣言する代表的な
// error status を取り上げ、errors.Is で意図した sentinel に一致することを確認する。
//
// account は 18 endpoint と多いため、各 endpoint で代表 status 1-3 件を検証する。
// statusError の switch logic は共通実装で各 endpoint test が交差カバーする。

func TestClient_RegisterPlayer_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "400 を受けたとき ErrBadRequest",
			status:     http.StatusBadRequest,
			wantTarget: apiaccountclient.ErrBadRequest,
		},
		{
			name:       "409 を受けたとき ErrConflict",
			status:     http.StatusConflict,
			wantTarget: apiaccountclient.ErrConflict,
		},
		{
			name:       "500 を受けたとき ErrInternalServer",
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
}

func TestClient_LoginPlayer_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "400 を受けたとき ErrBadRequest",
			status:     http.StatusBadRequest,
			wantTarget: apiaccountclient.ErrBadRequest,
		},
		{
			name:       "404 を受けたとき ErrNotFound",
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
}

func TestClient_GetPlayerByFirebaseUID_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "404 を受けたとき ErrNotFound",
			status:     http.StatusNotFound,
			wantTarget: apiaccountclient.ErrNotFound,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.FindByFirebaseUIDFn = func(_ string) (int, any) { return tc.status, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.GetPlayerByFirebaseUID(context.Background(), "uid")
			assertSentinel(t, err, tc.wantTarget)
		})
	}
}

func TestClient_AwardGameExp_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "400 を受けたとき ErrBadRequest",
			status:     http.StatusBadRequest,
			wantTarget: apiaccountclient.ErrBadRequest,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.AwardGameExpFn = func(_ apiaccount.AwardGameExpRequest) (int, any) { return tc.status, nil }

			c := newTestClient(t, srv.URL())
			err := c.AwardGameExp(context.Background(), apiaccount.AwardGameExpRequest{})
			assertSentinel(t, err, tc.wantTarget)
		})
	}
}

func TestClient_GetPlayer_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "401 を受けたとき ErrUnauthorized",
			status:     http.StatusUnauthorized,
			wantTarget: apiaccountclient.ErrUnauthorized,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.GetPlayerFn = func() (int, any) { return tc.status, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.GetPlayer(context.Background())
			assertSentinel(t, err, tc.wantTarget)
		})
	}
}

func TestClient_UpdateName_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "400 を受けたとき ErrBadRequest",
			status:     http.StatusBadRequest,
			wantTarget: apiaccountclient.ErrBadRequest,
		},
		{
			name:       "401 を受けたとき ErrUnauthorized",
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
}

func TestClient_ValidateNameForOnboarding_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "400 を受けたとき ErrBadRequest",
			status:     http.StatusBadRequest,
			wantTarget: apiaccountclient.ErrBadRequest,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.ValidateNameForOnboardingFn = func(_ apiaccount.ValidateNameForOnboardingRequest) (int, any) { return tc.status, nil }

			c := newTestClient(t, srv.URL())
			err := c.ValidateNameForOnboarding(context.Background(), apiaccount.ValidateNameForOnboardingRequest{})
			assertSentinel(t, err, tc.wantTarget)
		})
	}
}

func TestClient_GetBattleLimit_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "401 を受けたとき ErrUnauthorized",
			status:     http.StatusUnauthorized,
			wantTarget: apiaccountclient.ErrUnauthorized,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.GetBattleLimitFn = func() (int, any) { return tc.status, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.GetBattleLimit(context.Background())
			assertSentinel(t, err, tc.wantTarget)
		})
	}
}

func TestClient_IncrementBattleCount_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "401 を受けたとき ErrUnauthorized",
			status:     http.StatusUnauthorized,
			wantTarget: apiaccountclient.ErrUnauthorized,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.IncrementBattleCountFn = func() (int, any) { return tc.status, nil }

			c := newTestClient(t, srv.URL())
			err := c.IncrementBattleCount(context.Background())
			assertSentinel(t, err, tc.wantTarget)
		})
	}
}

func TestClient_UpdatePremium_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "400 を受けたとき ErrBadRequest",
			status:     http.StatusBadRequest,
			wantTarget: apiaccountclient.ErrBadRequest,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.UpdatePremiumFn = func(_ apiaccount.UpdatePremiumRequest) (int, any) { return tc.status, nil }

			c := newTestClient(t, srv.URL())
			err := c.UpdatePremium(context.Background(), apiaccount.UpdatePremiumRequest{})
			assertSentinel(t, err, tc.wantTarget)
		})
	}
}

func TestClient_AddExp_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "400 を受けたとき ErrBadRequest",
			status:     http.StatusBadRequest,
			wantTarget: apiaccountclient.ErrBadRequest,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.AddExpFn = func(_ apiaccount.AddExpRequest) (int, any) { return tc.status, nil }

			c := newTestClient(t, srv.URL())
			err := c.AddExp(context.Background(), apiaccount.AddExpRequest{})
			assertSentinel(t, err, tc.wantTarget)
		})
	}
}

func TestClient_ListFactions_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "401 を受けたとき ErrUnauthorized",
			status:     http.StatusUnauthorized,
			wantTarget: apiaccountclient.ErrUnauthorized,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.ListFactionsFn = func() (int, any) { return tc.status, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.ListFactions(context.Background())
			assertSentinel(t, err, tc.wantTarget)
		})
	}
}

func TestClient_GrantFaction_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "400 を受けたとき ErrBadRequest",
			status:     http.StatusBadRequest,
			wantTarget: apiaccountclient.ErrBadRequest,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.GrantFactionFn = func(_ apiaccount.FactionGrantRequest) (int, any) { return tc.status, nil }

			c := newTestClient(t, srv.URL())
			err := c.GrantFaction(context.Background(), apiaccount.FactionGrantRequest{})
			assertSentinel(t, err, tc.wantTarget)
		})
	}
}

func TestClient_SelectInitialFaction_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "400 を受けたとき ErrBadRequest",
			status:     http.StatusBadRequest,
			wantTarget: apiaccountclient.ErrBadRequest,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.SelectInitialFactionFn = func(_ apiaccount.SelectInitialFactionRequest) (int, any) { return tc.status, nil }

			c := newTestClient(t, srv.URL())
			err := c.SelectInitialFaction(context.Background(), apiaccount.SelectInitialFactionRequest{})
			assertSentinel(t, err, tc.wantTarget)
		})
	}
}

func TestClient_GetPlayerSettings_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "401 を受けたとき ErrUnauthorized",
			status:     http.StatusUnauthorized,
			wantTarget: apiaccountclient.ErrUnauthorized,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.GetSettingsFn = func() (int, any) { return tc.status, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.GetPlayerSettings(context.Background())
			assertSentinel(t, err, tc.wantTarget)
		})
	}
}

func TestClient_UpdatePlayerSettings_StatusMapping(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		wantTarget error
	}{
		{
			name:       "400 を受けたとき ErrBadRequest",
			status:     http.StatusBadRequest,
			wantTarget: apiaccountclient.ErrBadRequest,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()
			srv.UpdateSettingsFn = func(_ apiaccount.UpdateSettingsRequest) (int, any) { return tc.status, nil }

			c := newTestClient(t, srv.URL())
			_, err := c.UpdatePlayerSettings(context.Background(), apiaccount.UpdateSettingsRequest{})
			assertSentinel(t, err, tc.wantTarget)
		})
	}
}

// TestClient_RequestEditor は Option pattern の契約 (WithRequestEditorFn で渡した
// editor が全リクエストに適用される) を検証する。X-Internal-Auth header 注入の
// 接続点として SDK が機能することを担保する。
func TestClient_RequestEditor(t *testing.T) {
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
