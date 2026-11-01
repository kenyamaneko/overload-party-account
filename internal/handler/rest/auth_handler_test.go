package rest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kenyamaneko/overload-party-account/internal/handler/handlertest"
	"github.com/kenyamaneko/overload-party-account/internal/router/routertest"
)

func newAuthTestServer() *routertest.Server {
	s := routertest.NewServer(routertest.Options{
		GameConfig: map[string]int64{routertest.ConfigKeyExpCoefficient: 100},
	})
	s.Memory.AddPlayer(routertest.NewPlayerSeedA())
	return s
}

func TestRegisterPlayer(t *testing.T) {
	t.Run("[プレイヤー認証API]新規登録", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("FirebaseUIDがfirebase-newで未登録のとき、登録すると", func(t *testing.T) {
				register := func(t *testing.T) (*routertest.Server, *httptest.ResponseRecorder) {
					s := newAuthTestServer()
					return s, s.Call(t, http.MethodPost, "/internal/v1/auth/register", routertest.NoAuth, `{"firebase_uid":"firebase-new"}`)
				}

				t.Run("ステータスが201になる", func(t *testing.T) {
					_, rec := register(t)
					handlertest.AssertStatus(t, rec, http.StatusCreated)
				})
				t.Run("応答のfirebase_uidがfirebase-newになる", func(t *testing.T) {
					_, rec := register(t)
					handlertest.AssertJSONField(t, rec, "firebase_uid", `"firebase-new"`)
				})
				t.Run("応答のlevelが1になる", func(t *testing.T) {
					_, rec := register(t)
					handlertest.AssertJSONField(t, rec, "level", `1`)
				})
				t.Run("応答のexpが0になる", func(t *testing.T) {
					_, rec := register(t)
					handlertest.AssertJSONField(t, rec, "exp", `0`)
				})
				t.Run("応答のonboarding_statusがnot_startedになる", func(t *testing.T) {
					_, rec := register(t)
					handlertest.AssertJSONField(t, rec, "onboarding_status", `"not_started"`)
				})
				t.Run("続けてFirebaseUIDがfirebase-newでプレイヤー検索すると、応答のplayer_idが登録時に割り当てられたcccccccc-cccc-4ccc-8ccc-ccccccccccccになる", func(t *testing.T) {
					s, _ := register(t)
					found := s.Call(t, http.MethodGet, "/internal/v1/auth/by-firebase-uid/firebase-new", routertest.NoAuth, "")
					handlertest.AssertJSONField(t, found, "player_id", `"cccccccc-cccc-4ccc-8ccc-cccccccccccc"`)
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("firebase_uidが指定されないとき", func(t *testing.T) {
				cases := []struct {
					name string
					body string
				}{
					{name: "空文字のとき、firebase_uidの不足を示す400の応答になる", body: `{"firebase_uid":""}`},
					{name: "キーが無いとき、firebase_uidの不足を示す400の応答になる", body: `{}`},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						rec := newAuthTestServer().Call(t, http.MethodPost, "/internal/v1/auth/register", routertest.NoAuth, tc.body)
						handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "firebase_uid is required")
					})
				}
			})
			t.Run("リクエストボディを、firebase_uidが文字列のJSONオブジェクトとして読めないとき", func(t *testing.T) {
				cases := []struct {
					name string
					body string
				}{
					{name: "リクエストボディが無いとき、リクエストボディ不正を示す400の応答になる", body: ""},
					{name: `リクエストボディが{"firebase_uid":だけのとき、リクエストボディ不正を示す400の応答になる`, body: `{"firebase_uid":`},
					{name: "firebase_uidが数値123のとき、リクエストボディ不正を示す400の応答になる", body: `{"firebase_uid":123}`},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						rec := newAuthTestServer().Call(t, http.MethodPost, "/internal/v1/auth/register", routertest.NoAuth, tc.body)
						handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "invalid request body")
					})
				}
			})
		})
	})
}

func TestLoginPlayer(t *testing.T) {
	t.Run("[プレイヤー認証API]ログイン", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("player_idがaaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaaで登録済みのプレイヤーのFirebaseUIDで、ログインするとき", func(t *testing.T) {
				login := func(t *testing.T) *httptest.ResponseRecorder {
					return newAuthTestServer().Call(t, http.MethodPost, "/internal/v1/auth/login", routertest.NoAuth, `{"firebase_uid":"`+routertest.FirebaseUIDA+`"}`)
				}

				t.Run("ステータスが200になる", func(t *testing.T) {
					handlertest.AssertStatus(t, login(t), http.StatusOK)
				})
				t.Run("応答のplayer_idがaaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaaになる", func(t *testing.T) {
					handlertest.AssertJSONField(t, login(t), "player_id", `"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"`)
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("firebase_uidが指定されないとき", func(t *testing.T) {
				cases := []struct {
					name string
					body string
				}{
					{name: "空文字のとき、firebase_uidの不足を示す400の応答になる", body: `{"firebase_uid":""}`},
					{name: "キーが無いとき、firebase_uidの不足を示す400の応答になる", body: `{}`},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						rec := newAuthTestServer().Call(t, http.MethodPost, "/internal/v1/auth/login", routertest.NoAuth, tc.body)
						handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "firebase_uid is required")
					})
				}
			})
			t.Run("リクエストボディが無いとき、リクエストボディ不正を示す400の応答になる", func(t *testing.T) {
				rec := newAuthTestServer().Call(t, http.MethodPost, "/internal/v1/auth/login", routertest.NoAuth, "")
				handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "invalid request body")
			})
		})
	})
}

func TestFindPlayerByFirebaseUID(t *testing.T) {
	t.Run("[プレイヤー認証API]FirebaseUIDによるプレイヤー検索", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("player_idがaaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaaで登録済みのプレイヤーのFirebaseUIDで、検索するとき", func(t *testing.T) {
				find := func(t *testing.T) *httptest.ResponseRecorder {
					return newAuthTestServer().Call(t, http.MethodGet, "/internal/v1/auth/by-firebase-uid/"+routertest.FirebaseUIDA, routertest.NoAuth, "")
				}

				t.Run("ステータスが200になる", func(t *testing.T) {
					handlertest.AssertStatus(t, find(t), http.StatusOK)
				})
				t.Run("応答のplayer_idがaaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaaになる", func(t *testing.T) {
					handlertest.AssertJSONField(t, find(t), "player_id", `"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa"`)
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("未登録のFirebaseUIDで検索すると、プレイヤー不在を示す404の応答になる", func(t *testing.T) {
				rec := newAuthTestServer().Call(t, http.MethodGet, "/internal/v1/auth/by-firebase-uid/firebase-unknown", routertest.NoAuth, "")
				handlertest.AssertErrorResponse(t, rec, http.StatusNotFound, "player not found")
			})
		})
	})
}
