package apiaccountserverfake_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
	"github.com/kenyamaneko/overload-party-account/packages/api-account/apiaccountserverfake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Fn 未設定の endpoint は既定応答を返す (空成功応答 or 204)。全 15 endpoint を
// 網羅的に既定動作で叩いて一通り応答が得られることを固定する。
func TestServer_DefaultResponses(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		reqBody    []byte
		wantStatus int
	}{
		{name: "Register 既定は 201", method: http.MethodPost, path: "/internal/v1/auth/register", reqBody: []byte(`{}`), wantStatus: http.StatusCreated},
		{name: "Login 既定は 200", method: http.MethodPost, path: "/internal/v1/auth/login", reqBody: []byte(`{}`), wantStatus: http.StatusOK},
		{name: "FindByFirebaseUID 既定は 200", method: http.MethodGet, path: "/internal/v1/auth/by-firebase-uid/uid-1", reqBody: nil, wantStatus: http.StatusOK},
		{name: "GetPlayer 既定は 200", method: http.MethodGet, path: "/internal/v1/players/p-1", reqBody: nil, wantStatus: http.StatusOK},
		{name: "UpdateName 既定は 200", method: http.MethodPut, path: "/internal/v1/players/p-1/name", reqBody: []byte(`{}`), wantStatus: http.StatusOK},
		{name: "GetBattleLimit 既定は 200", method: http.MethodGet, path: "/internal/v1/players/p-1/battle-limit", reqBody: nil, wantStatus: http.StatusOK},
		{name: "IncrementBattleCount 既定は 204", method: http.MethodPost, path: "/internal/v1/players/p-1/battle-limit/increment", reqBody: nil, wantStatus: http.StatusNoContent},
		{name: "UpdatePremium 既定は 204", method: http.MethodPut, path: "/internal/v1/players/p-1/premium", reqBody: []byte(`{}`), wantStatus: http.StatusNoContent},
		{name: "UpdateFaction 既定は 204", method: http.MethodPut, path: "/internal/v1/players/p-1/faction", reqBody: []byte(`{}`), wantStatus: http.StatusNoContent},
		{name: "GrantFaction 既定は 204", method: http.MethodPost, path: "/internal/v1/players/p-1/factions", reqBody: []byte(`{}`), wantStatus: http.StatusNoContent},
		{name: "ListFactions 既定は 200", method: http.MethodGet, path: "/internal/v1/players/p-1/factions", reqBody: nil, wantStatus: http.StatusOK},
		{name: "AddExp 既定は 204", method: http.MethodPost, path: "/internal/v1/players/p-1/exp", reqBody: []byte(`{}`), wantStatus: http.StatusNoContent},
		{name: "AwardGameExp 既定は 204", method: http.MethodPost, path: "/internal/v1/players/award-game-exp", reqBody: []byte(`{}`), wantStatus: http.StatusNoContent},
		{name: "GetSettings 既定は 200", method: http.MethodGet, path: "/internal/v1/players/p-1/settings", reqBody: nil, wantStatus: http.StatusOK},
		{name: "UpdateSettings 既定は 200", method: http.MethodPut, path: "/internal/v1/players/p-1/settings", reqBody: []byte(`{}`), wantStatus: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()

			req, _ := http.NewRequest(tt.method, srv.URL()+tt.path, bytes.NewReader(tt.reqBody))
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
		})
	}
}

// RegisterFn は request body を typed に受け取って Player を返せる。
// 代表例として body round trip と auto-defaults を固定する。
// Register は name を取らず firebase_uid のみで動くため、Player.Name は nil
// (オンボーディング前) のまま返ることを固定する。
func TestServer_RegisterFn_RoundTrip(t *testing.T) {
	srv := apiaccountserverfake.NewServer()
	defer srv.Close()

	var gotReq apiaccount.RegisterRequest
	srv.RegisterFn = func(req apiaccount.RegisterRequest) (int, any) {
		gotReq = req
		return http.StatusCreated, apiaccount.Player{
			PlayerID:    "p-new",
			FirebaseUID: req.FirebaseUID,
			// Register 直後 name は未確定。
			Name: nil,
		}
	}

	reqBody, _ := json.Marshal(apiaccount.RegisterRequest{FirebaseUID: "uid-42"})
	req, _ := http.NewRequest(http.MethodPost, srv.URL()+"/internal/v1/auth/register", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, "uid-42", gotReq.FirebaseUID)

	var decoded apiaccount.Player
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&decoded))
	assert.Equal(t, "p-new", decoded.PlayerID)
	assert.Nil(t, decoded.Name)
}

// RegisterFn で 409 Conflict を擬似できる (既存プレイヤー衝突シナリオ)。
// accountclient は 409 を ErrPlayerAlreadyRegistered に変換するため、fake で
// ここを再現できることが contract 整合の鍵。
func TestServer_RegisterFn_CanReturn409(t *testing.T) {
	srv := apiaccountserverfake.NewServer()
	defer srv.Close()

	srv.RegisterFn = func(_ apiaccount.RegisterRequest) (int, any) {
		return http.StatusConflict, nil
	}

	reqBody := []byte(`{"firebase_uid":"dup","username":"x"}`)
	req, _ := http.NewRequest(http.MethodPost, srv.URL()+"/internal/v1/auth/register", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusConflict, resp.StatusCode)
}

// path variable 展開が正しく動く (playerID がハンドラーに届く)。
// UpdateName endpoint を代表例として固定する。他 endpoint も同ロジック。
func TestServer_PathVariables_ExtractPlayerID(t *testing.T) {
	srv := apiaccountserverfake.NewServer()
	defer srv.Close()

	var gotPlayerID string
	var gotReq apiaccount.UpdateNameRequest
	srv.UpdateNameFn = func(playerID string, req apiaccount.UpdateNameRequest) (int, any) {
		gotPlayerID = playerID
		gotReq = req
		return http.StatusOK, apiaccount.Player{PlayerID: playerID, Name: &req.Name}
	}

	reqBody := []byte(`{"name":"bob"}`)
	req, _ := http.NewRequest(http.MethodPut, srv.URL()+"/internal/v1/players/p-123/name", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "p-123", gotPlayerID)
	assert.Equal(t, "bob", gotReq.Name)
}

// AwardGameExpFn は playerID を持たない (body のみで player 2 名を渡す)。
// award-game-exp だけ path pattern が特殊なので個別テストで covering する。
func TestServer_AwardGameExpFn_NoPlayerID(t *testing.T) {
	srv := apiaccountserverfake.NewServer()
	defer srv.Close()

	var gotReq apiaccount.AwardGameExpRequest
	srv.AwardGameExpFn = func(req apiaccount.AwardGameExpRequest) (int, any) {
		gotReq = req
		return http.StatusNoContent, nil
	}

	reqBody, _ := json.Marshal(apiaccount.AwardGameExpRequest{
		Player1ID: "p-a", Player2ID: "p-b", WinnerNum: 1, Reason: "win", MatchType: "ranked",
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL()+"/internal/v1/players/award-game-exp", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	assert.Equal(t, "p-a", gotReq.Player1ID)
	assert.Equal(t, "p-b", gotReq.Player2ID)
	assert.Equal(t, int64(1), gotReq.WinnerNum)
}
