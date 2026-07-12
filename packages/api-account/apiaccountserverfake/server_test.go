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

func TestServer(t *testing.T) {
	t.Run("サーバフェイク", func(t *testing.T) {
		t.Run("RegisterFn は request body を typed で受け取り、name 未設定の Player を返す", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()

			var gotReq apiaccount.RegisterRequest
			srv.RegisterFn = func(req apiaccount.RegisterRequest) (int, any) {
				gotReq = req
				return http.StatusCreated, apiaccount.PlayerResponse{
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

			var decoded apiaccount.PlayerResponse
			require.NoError(t, json.NewDecoder(resp.Body).Decode(&decoded))
			assert.Equal(t, "p-new", decoded.PlayerID)
			assert.Nil(t, decoded.Name)
		})

		t.Run("FindByFirebaseUID は path variable から Firebase UID を抽出して Fn に渡す", func(t *testing.T) {
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()

			var gotUID string
			srv.FindByFirebaseUIDFn = func(firebaseUID string) (int, any) {
				gotUID = firebaseUID
				return http.StatusOK, apiaccount.PlayerResponse{PlayerID: "p-1", FirebaseUID: firebaseUID}
			}

			req, _ := http.NewRequest(http.MethodGet, srv.URL()+"/internal/v1/auth/by-firebase-uid/uid-xyz", nil)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "uid-xyz", gotUID)
		})

		t.Run("UpdateNameFn は body のみを typed で受け取る", func(t *testing.T) {
			// player-scoped endpoint の playerID は path になく JWT sub から取得する規約のため、
			// fake の Fn には露出せず body だけを受け取る。
			srv := apiaccountserverfake.NewServer()
			defer srv.Close()

			var gotReq apiaccount.UpdateNameRequest
			srv.UpdateNameFn = func(req apiaccount.UpdateNameRequest) (int, any) {
				gotReq = req
				name := req.Name
				return http.StatusOK, apiaccount.PlayerResponse{PlayerID: "p-me", Name: &name}
			}

			reqBody := []byte(`{"name":"bob"}`)
			req, _ := http.NewRequest(http.MethodPut, srv.URL()+"/api/v1/account/me/name", bytes.NewReader(reqBody))
			req.Header.Set("Content-Type", "application/json")
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, "bob", gotReq.Name)
		})

		t.Run("AwardGameExpFn は playerID を持たず body のみで player 2 名を受け取る", func(t *testing.T) {
			// award-game-exp は battle が直接呼ぶサーバー間バッチのため /internal 配下に置き、
			// player 2 名を body で渡す。
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
		})
	})
}
