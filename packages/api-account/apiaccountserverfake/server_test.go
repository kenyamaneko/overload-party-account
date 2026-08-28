package apiaccountserverfake_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
	"github.com/kenyamaneko/overload-party-account/packages/api-account/apiaccountserverfake"
)

func doRequest(t *testing.T, method, url string, body any) *http.Response {
	t.Helper()

	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, url, reader)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return resp
}

func decodeBody(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	require.NoError(t, json.NewDecoder(resp.Body).Decode(v))
}

func TestServer_DefaultResponses(t *testing.T) {
	t.Run("Fnがnilのときの既定応答", func(t *testing.T) {
		t.Run("新規プレイヤー登録は既定のステータスコードとゼロ値のJSONボディを返す", func(t *testing.T) {
			s := apiaccountserverfake.NewServer()
			defer s.Close()

			resp := doRequest(t, http.MethodPost, s.URL()+"/internal/v1/auth/register", apiaccount.RegisterRequest{FirebaseUID: "fb-1"})

			assert.Equal(t, 201, resp.StatusCode)
			var body apiaccount.PlayerResponse
			decodeBody(t, resp, &body)
			assert.Equal(t, "", body.PlayerID)
		})

		t.Run("プレイヤーログインは既定のステータスコードとゼロ値のJSONボディを返す", func(t *testing.T) {
			s := apiaccountserverfake.NewServer()
			defer s.Close()

			resp := doRequest(t, http.MethodPost, s.URL()+"/internal/v1/auth/login", apiaccount.LoginRequest{FirebaseUID: "fb-1"})

			assert.Equal(t, 200, resp.StatusCode)
			var body apiaccount.PlayerResponse
			decodeBody(t, resp, &body)
			assert.Equal(t, "", body.PlayerID)
		})

		t.Run("Firebase UIDによるプレイヤー検索は既定のステータスコードとゼロ値のJSONボディを返す", func(t *testing.T) {
			s := apiaccountserverfake.NewServer()
			defer s.Close()

			resp := doRequest(t, http.MethodGet, s.URL()+"/internal/v1/auth/by-firebase-uid/fb-1", nil)

			assert.Equal(t, 200, resp.StatusCode)
			var body apiaccount.PlayerResponse
			decodeBody(t, resp, &body)
			assert.Equal(t, "", body.PlayerID)
		})

		t.Run("認証済みプレイヤー自身の情報取得は既定のステータスコードとゼロ値のJSONボディを返す", func(t *testing.T) {
			s := apiaccountserverfake.NewServer()
			defer s.Close()

			resp := doRequest(t, http.MethodGet, s.URL()+"/api/v1/account/me", nil)

			assert.Equal(t, 200, resp.StatusCode)
			var body apiaccount.PlayerResponse
			decodeBody(t, resp, &body)
			assert.Equal(t, "", body.PlayerID)
		})

		t.Run("プレイヤー名の変更は既定のステータスコードとゼロ値のJSONボディを返す", func(t *testing.T) {
			s := apiaccountserverfake.NewServer()
			defer s.Close()

			resp := doRequest(t, http.MethodPut, s.URL()+"/api/v1/account/me/name", apiaccount.UpdateNameRequest{Name: "名前"})

			assert.Equal(t, 200, resp.StatusCode)
			var body apiaccount.PlayerResponse
			decodeBody(t, resp, &body)
			assert.Equal(t, "", body.PlayerID)
		})

		t.Run("1日のバトル回数制限の取得は既定のステータスコードとゼロ値のJSONボディを返す", func(t *testing.T) {
			s := apiaccountserverfake.NewServer()
			defer s.Close()

			resp := doRequest(t, http.MethodGet, s.URL()+"/api/v1/account/me/battle-limit", nil)

			assert.Equal(t, 200, resp.StatusCode)
			var body apiaccount.BattleLimitResponse
			decodeBody(t, resp, &body)
			assert.Equal(t, int64(0), body.DailyBattleLimit)
		})

		t.Run("プレイヤー設定の取得は既定のステータスコードとゼロ値のJSONボディを返す", func(t *testing.T) {
			s := apiaccountserverfake.NewServer()
			defer s.Close()

			resp := doRequest(t, http.MethodGet, s.URL()+"/api/v1/account/me/settings", nil)

			assert.Equal(t, 200, resp.StatusCode)
			var body apiaccount.PlayerSettingsResponse
			decodeBody(t, resp, &body)
			assert.Equal(t, "", body.PlayerID)
		})

		t.Run("プレイヤー設定の更新は既定のステータスコードとゼロ値のJSONボディを返す", func(t *testing.T) {
			s := apiaccountserverfake.NewServer()
			defer s.Close()

			resp := doRequest(t, http.MethodPut, s.URL()+"/api/v1/account/me/settings", apiaccount.UpdateSettingsRequest{})

			assert.Equal(t, 200, resp.StatusCode)
			var body apiaccount.PlayerSettingsResponse
			decodeBody(t, resp, &body)
			assert.Equal(t, "", body.PlayerID)
		})

		t.Run("所持陣営一覧の取得のみ、既定のレスポンスボディは空配列になる", func(t *testing.T) {
			s := apiaccountserverfake.NewServer()
			defer s.Close()

			resp := doRequest(t, http.MethodGet, s.URL()+"/api/v1/account/me/factions", nil)

			assert.Equal(t, 200, resp.StatusCode)
			var body apiaccount.FactionListing
			decodeBody(t, resp, &body)
			assert.Equal(t, []string{}, body.Factions)
		})

		t.Run("経験値の加算は既定で204を返し、ボディを持たない応答になる", func(t *testing.T) {
			s := apiaccountserverfake.NewServer()
			defer s.Close()

			resp := doRequest(t, http.MethodPost, s.URL()+"/api/v1/account/me/exp", apiaccount.AddExpRequest{ExpGain: 1})
			defer resp.Body.Close()

			assert.Equal(t, 204, resp.StatusCode)
			b, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Empty(t, b)
		})

		t.Run("初期陣営の選択は既定で200を返し、ボディを持たない応答になる", func(t *testing.T) {
			s := apiaccountserverfake.NewServer()
			defer s.Close()

			resp := doRequest(t, http.MethodPost, s.URL()+"/api/v1/account/me/factions/select", apiaccount.SelectInitialFactionRequest{FactionID: "SHE"})
			defer resp.Body.Close()

			assert.Equal(t, 200, resp.StatusCode)
			b, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Empty(t, b)
		})
	})
}

func TestServer_FnOverride(t *testing.T) {
	t.Run("Fnが設定されているとき、Fnが返すステータスコードとレスポンスボディをそのまま応答する", func(t *testing.T) {
		s := apiaccountserverfake.NewServer()
		defer s.Close()
		s.GetPlayerFn = func() (int, any) {
			return 404, map[string]string{"error": "not found"}
		}

		resp := doRequest(t, http.MethodGet, s.URL()+"/api/v1/account/me", nil)
		defer resp.Body.Close()

		assert.Equal(t, 404, resp.StatusCode)
		var body map[string]string
		decodeBody(t, resp, &body)
		assert.Equal(t, "not found", body["error"])
	})

	t.Run("POST/PUT系エンドポイントでは、リクエストボディをデコードしてFnの引数として渡す", func(t *testing.T) {
		s := apiaccountserverfake.NewServer()
		defer s.Close()
		var received apiaccount.UpdateNameRequest
		s.UpdateNameFn = func(req apiaccount.UpdateNameRequest) (int, any) {
			received = req
			return 200, apiaccount.PlayerResponse{PlayerID: "player-1"}
		}

		resp := doRequest(t, http.MethodPut, s.URL()+"/api/v1/account/me/name", apiaccount.UpdateNameRequest{Name: "新しい名前"})
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
		assert.Equal(t, "新しい名前", received.Name)
	})
}

func TestServer_PlayerScopedEndpointsSkipJWTVerification(t *testing.T) {
	t.Run("player-scopedエンドポイントは、X-Internal-Authヘッダを付けなくても応答する", func(t *testing.T) {
		s := apiaccountserverfake.NewServer()
		defer s.Close()

		resp := doRequest(t, http.MethodGet, s.URL()+"/api/v1/account/me", nil)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)
	})
}
