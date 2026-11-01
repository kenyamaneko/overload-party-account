package rest_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/handler/handlertest"
	"github.com/kenyamaneko/overload-party-account/internal/router/routertest"
)

func decodeStringList(t *testing.T, rec *httptest.ResponseRecorder, field string) []string {
	t.Helper()
	var list []string
	require.NoError(t, json.Unmarshal(handlertest.DecodeObject(t, rec)[field], &list))
	return list
}

func assertFactions(t *testing.T, rec *httptest.ResponseRecorder, expected []string) {
	t.Helper()
	assert.ElementsMatch(t, expected, decodeStringList(t, rec, "factions"))
}

const (
	grantFactionPath         = "/api/v1/account/me/factions"
	selectInitialFactionPath = "/api/v1/account/me/factions/select"
)

func newFactionTestServer() *routertest.Server {
	return routertest.NewSeededServer(
		map[string]int64{routertest.ConfigKeyExpCoefficient: 100},
		routertest.NewPlayerSeedA(),
		routertest.NewPlayerSeedB(),
	)
}

func TestListFactions(t *testing.T) {
	t.Run("[陣営API]所持陣営一覧", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("認証済みプレイヤーが自分の所持陣営一覧を取得するとき", func(t *testing.T) {
				t.Run("所持陣営が0件のとき、factionsが空の配列になる", func(t *testing.T) {
					rec := newFactionTestServer().GetFactions(t, routertest.TokenA)
					handlertest.AssertJSONField(t, rec, "factions", `[]`)
				})
				t.Run("所持陣営がSHEの1件のとき、factionsがSHEだけになる", func(t *testing.T) {
					s := newFactionTestServer()
					s.Memory.AddFaction(routertest.PlayerAID, "SHE")
					assertFactions(t, s.GetFactions(t, routertest.TokenA), []string{"SHE"})
				})
				t.Run("所持陣営がSHEとTenkiの2件のとき、factionsがSHEとTenkiの2件になる", func(t *testing.T) {
					s := newFactionTestServer()
					s.Memory.AddFaction(routertest.PlayerAID, "SHE")
					s.Memory.AddFaction(routertest.PlayerAID, "Tenki")
					assertFactions(t, s.GetFactions(t, routertest.TokenA), []string{"SHE", "Tenki"})
				})
			})
			t.Run("プレイヤーAを指定して所持陣営一覧を取得するとき", func(t *testing.T) {
				listPlayerA := func(t *testing.T, s *routertest.Server) *httptest.ResponseRecorder {
					return s.Call(t, http.MethodGet, "/internal/v1/players/"+routertest.PlayerAID+"/factions", routertest.NoAuth, "")
				}

				t.Run("プレイヤーAの所持陣営が0件のとき、factionsが空の配列になる", func(t *testing.T) {
					handlertest.AssertJSONField(t, listPlayerA(t, newFactionTestServer()), "factions", `[]`)
				})
				t.Run("プレイヤーAの所持陣営がSHEとTenkiの2件のとき、factionsがSHEとTenkiの2件になる", func(t *testing.T) {
					s := newFactionTestServer()
					s.Memory.AddFaction(routertest.PlayerAID, "SHE")
					s.Memory.AddFaction(routertest.PlayerAID, "Tenki")
					assertFactions(t, listPlayerA(t, s), []string{"SHE", "Tenki"})
				})
				t.Run("プレイヤーAがSHE、プレイヤーBがTenkiを所持するとき、factionsがSHEだけになる", func(t *testing.T) {
					s := newFactionTestServer()
					s.Memory.AddFaction(routertest.PlayerAID, "SHE")
					s.Memory.AddFaction(routertest.PlayerBID, "Tenki")
					assertFactions(t, listPlayerA(t, s), []string{"SHE"})
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("プレイヤーIDにnot-a-uuidを指定して所持陣営一覧を取得すると、プレイヤーID不正を示す400の応答になる", func(t *testing.T) {
				rec := newFactionTestServer().Call(t, http.MethodGet, "/internal/v1/players/not-a-uuid/factions", routertest.NoAuth, "")
				handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "invalid player id")
			})
		})
	})
}

func TestGrantFaction(t *testing.T) {
	t.Run("[陣営API]所持陣営の付与", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("陣営Sugarを付与するとき", func(t *testing.T) {
				grant := func(t *testing.T) (*routertest.Server, *httptest.ResponseRecorder) {
					s := newFactionTestServer()
					return s, s.PostAs(t, routertest.TokenA, grantFactionPath, `{"faction":"Sugar"}`)
				}

				t.Run("ステータスが204になる", func(t *testing.T) {
					_, rec := grant(t)
					handlertest.AssertStatus(t, rec, http.StatusNoContent)
				})
				t.Run("続けて所持陣営一覧を取得すると、factionsにSugarが含まれる", func(t *testing.T) {
					s, _ := grant(t)
					assert.Contains(t, decodeStringList(t, s.GetFactions(t, routertest.TokenA), "factions"), "Sugar")
				})
			})
			t.Run("陣営Sugarを2回続けて付与するとき", func(t *testing.T) {
				grantTwice := func(t *testing.T) (*routertest.Server, *httptest.ResponseRecorder) {
					s := newFactionTestServer()
					s.PostAs(t, routertest.TokenA, grantFactionPath, `{"faction":"Sugar"}`)
					return s, s.PostAs(t, routertest.TokenA, grantFactionPath, `{"faction":"Sugar"}`)
				}

				t.Run("2回目のステータスが204になる", func(t *testing.T) {
					_, rec := grantTwice(t)
					handlertest.AssertStatus(t, rec, http.StatusNoContent)
				})
				t.Run("続けて所持陣営一覧を取得すると、factionsにSugarが1件だけ含まれる", func(t *testing.T) {
					s, _ := grantTwice(t)
					assertFactions(t, s.GetFactions(t, routertest.TokenA), []string{"Sugar"})
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("factionが指定されないとき", func(t *testing.T) {
				cases := []struct {
					name string
					body string
				}{
					{name: "空文字のとき、factionの不足を示す400の応答になる", body: `{"faction":""}`},
					{name: "キーが無いとき、factionの不足を示す400の応答になる", body: `{}`},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						rec := newFactionTestServer().PostAs(t, routertest.TokenA, grantFactionPath, tc.body)
						handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "faction is required")
					})
				}
			})
			t.Run("リクエストボディが無いとき、リクエストボディ不正を示す400の応答になる", func(t *testing.T) {
				rec := newFactionTestServer().PostAs(t, routertest.TokenA, grantFactionPath, "")
				handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "invalid request body")
			})
			t.Run("許可されていない陣営を付与するとき", func(t *testing.T) {
				cases := []struct {
					name string
					body string
				}{
					{name: "Unknownのとき、factionの不正を示す400の応答になる", body: `{"faction":"Unknown"}`},
					{name: "Neutralのとき、factionの不正を示す400の応答になる", body: `{"faction":"Neutral"}`},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						rec := newFactionTestServer().PostAs(t, routertest.TokenA, grantFactionPath, tc.body)
						handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "faction is invalid")
					})
				}
			})
		})
	})
}

func TestSelectInitialFaction(t *testing.T) {
	t.Run("[陣営API]初期陣営の選択", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("SHEを選ぶとき", func(t *testing.T) {
				selectSHE := func(t *testing.T) (*routertest.Server, *httptest.ResponseRecorder) {
					s := newFactionTestServer()
					return s, s.PostAs(t, routertest.TokenA, selectInitialFactionPath, `{"faction_id":"SHE"}`)
				}

				t.Run("ステータスが200になる", func(t *testing.T) {
					_, rec := selectSHE(t)
					handlertest.AssertStatus(t, rec, http.StatusOK)
				})
				t.Run("続けてプレイヤー情報を取得すると、initial_factionがSHEになる", func(t *testing.T) {
					s, _ := selectSHE(t)
					handlertest.AssertJSONField(t, s.GetPlayer(t, routertest.TokenA), "initial_faction", `"SHE"`)
				})
				t.Run("続けて所持陣営一覧を取得すると、factionsにSHEが含まれる", func(t *testing.T) {
					s, _ := selectSHE(t)
					assert.Contains(t, decodeStringList(t, s.GetFactions(t, routertest.TokenA), "factions"), "SHE")
				})
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("faction_idが指定されないとき", func(t *testing.T) {
				cases := []struct {
					name string
					body string
				}{
					{name: "空文字のとき、faction_idの不足を示す400の応答になる", body: `{"faction_id":""}`},
					{name: "キーが無いとき、faction_idの不足を示す400の応答になる", body: `{}`},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						rec := newFactionTestServer().PostAs(t, routertest.TokenA, selectInitialFactionPath, tc.body)
						handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "faction_id is required")
					})
				}
			})
			t.Run("リクエストボディが無いとき、リクエストボディ不正を示す400の応答になる", func(t *testing.T) {
				rec := newFactionTestServer().PostAs(t, routertest.TokenA, selectInitialFactionPath, "")
				handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "invalid request body")
			})
			t.Run("SHEを選択済みのプレイヤーがTenkiを選ぼうとしたあと、プレイヤー情報を取得すると、initial_factionがSHEのままになる", func(t *testing.T) {
				s := newFactionTestServer()
				s.Memory.AddInitialFaction(routertest.PlayerAID, "SHE")
				s.PostAs(t, routertest.TokenA, selectInitialFactionPath, `{"faction_id":"Tenki"}`)
				handlertest.AssertJSONField(t, s.GetPlayer(t, routertest.TokenA), "initial_faction", `"SHE"`)
			})
		})
	})
}
