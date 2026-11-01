package rest_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/handler/handlertest"
	"github.com/kenyamaneko/overload-party-account/internal/router/routertest"
)

const settingsPath = "/api/v1/account/me/settings"

func newSettingsTestServer() *routertest.Server {
	s := routertest.NewSeededServer(map[string]int64{routertest.ConfigKeyExpCoefficient: 100}, routertest.NewPlayerSeedA())
	s.Memory.AddSettings(domain.PlayerSettings{
		PlayerID:      routertest.PlayerAID,
		Language:      "ja",
		BgmVolume:     50,
		SeVolume:      50,
		IsPushEnabled: true,
		UpdatedAt:     routertest.FixedNow,
	})
	return s
}

func TestUpdatePlayerSettings(t *testing.T) {
	t.Run("[プレイヤー設定API]プレイヤー設定の更新", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("プレイヤー設定が保存済みのとき", func(t *testing.T) {
				t.Run("1項目だけを指定して更新するとき", func(t *testing.T) {
					cases := []struct {
						name         string
						body         string
						field        string
						expectedJSON string
					}{
						{name: "languageだけenのとき、応答のlanguageがenになる", body: `{"language":"en"}`, field: "language", expectedJSON: `"en"`},
						{name: "push_enabledだけfalseのとき、応答のpush_enabledがfalseになる", body: `{"push_enabled":false}`, field: "push_enabled", expectedJSON: `false`},
						{name: "bgm_volumeだけ0のとき、応答のbgm_volumeが0になる", body: `{"bgm_volume":0}`, field: "bgm_volume", expectedJSON: `0`},
						{name: "bgm_volumeだけ100のとき、応答のbgm_volumeが100になる", body: `{"bgm_volume":100}`, field: "bgm_volume", expectedJSON: `100`},
						{name: "se_volumeだけ0のとき、応答のse_volumeが0になる", body: `{"se_volume":0}`, field: "se_volume", expectedJSON: `0`},
						{name: "se_volumeだけ100のとき、応答のse_volumeが100になる", body: `{"se_volume":100}`, field: "se_volume", expectedJSON: `100`},
					}
					for _, tc := range cases {
						t.Run(tc.name, func(t *testing.T) {
							rec := newSettingsTestServer().PutAs(t, routertest.TokenA, settingsPath, tc.body)
							handlertest.AssertJSONField(t, rec, tc.field, tc.expectedJSON)
						})
					}
				})
				t.Run("languageがen、bgm_volumeが80、se_volumeが20、push_enabledがfalseの4項目を指定して更新するとき", func(t *testing.T) {
					update := func(t *testing.T) *httptest.ResponseRecorder {
						return newSettingsTestServer().PutAs(t, routertest.TokenA, settingsPath, `{"language":"en","bgm_volume":80,"se_volume":20,"push_enabled":false}`)
					}

					t.Run("応答のlanguageがenになる", func(t *testing.T) {
						handlertest.AssertJSONField(t, update(t), "language", `"en"`)
					})
					t.Run("応答のbgm_volumeが80になる", func(t *testing.T) {
						handlertest.AssertJSONField(t, update(t), "bgm_volume", `80`)
					})
					t.Run("応答のse_volumeが20になる", func(t *testing.T) {
						handlertest.AssertJSONField(t, update(t), "se_volume", `20`)
					})
					t.Run("応答のpush_enabledがfalseになる", func(t *testing.T) {
						handlertest.AssertJSONField(t, update(t), "push_enabled", `false`)
					})
				})
				t.Run("languageだけenを指定して更新するとき", func(t *testing.T) {
					update := func(t *testing.T) (*routertest.Server, *httptest.ResponseRecorder) {
						s := newSettingsTestServer()
						return s, s.PutAs(t, routertest.TokenA, settingsPath, `{"language":"en"}`)
					}

					t.Run("ステータスが200になる", func(t *testing.T) {
						_, rec := update(t)
						handlertest.AssertStatus(t, rec, http.StatusOK)
					})
					t.Run("続けてプレイヤー設定を取得すると、languageがenになる", func(t *testing.T) {
						s, _ := update(t)
						handlertest.AssertJSONField(t, s.GetSettings(t, routertest.TokenA), "language", `"en"`)
					})
				})
			})
			t.Run("保存済みの設定が、言語ja、BGM音量50、SE音量50、通知許可有効で、1項目だけを指定して更新するとき", func(t *testing.T) {
				cases := []struct {
					name         string
					body         string
					field        string
					expectedJSON string
				}{
					{name: "languageだけenのとき、応答のbgm_volumeが50のままになる", body: `{"language":"en"}`, field: "bgm_volume", expectedJSON: `50`},
					{name: "languageだけenのとき、応答のse_volumeが50のままになる", body: `{"language":"en"}`, field: "se_volume", expectedJSON: `50`},
					{name: "languageだけenのとき、応答のpush_enabledがtrueのままになる", body: `{"language":"en"}`, field: "push_enabled", expectedJSON: `true`},
					{name: "bgm_volumeだけ80のとき、応答のlanguageがjaのままになる", body: `{"bgm_volume":80}`, field: "language", expectedJSON: `"ja"`},
					{name: "bgm_volumeだけ80のとき、応答のse_volumeが50のままになる", body: `{"bgm_volume":80}`, field: "se_volume", expectedJSON: `50`},
					{name: "bgm_volumeだけ80のとき、応答のpush_enabledがtrueのままになる", body: `{"bgm_volume":80}`, field: "push_enabled", expectedJSON: `true`},
					{name: "se_volumeだけ20のとき、応答のlanguageがjaのままになる", body: `{"se_volume":20}`, field: "language", expectedJSON: `"ja"`},
					{name: "se_volumeだけ20のとき、応答のbgm_volumeが50のままになる", body: `{"se_volume":20}`, field: "bgm_volume", expectedJSON: `50`},
					{name: "se_volumeだけ20のとき、応答のpush_enabledがtrueのままになる", body: `{"se_volume":20}`, field: "push_enabled", expectedJSON: `true`},
					{name: "push_enabledだけfalseのとき、応答のlanguageがjaのままになる", body: `{"push_enabled":false}`, field: "language", expectedJSON: `"ja"`},
					{name: "push_enabledだけfalseのとき、応答のbgm_volumeが50のままになる", body: `{"push_enabled":false}`, field: "bgm_volume", expectedJSON: `50`},
					{name: "push_enabledだけfalseのとき、応答のse_volumeが50のままになる", body: `{"push_enabled":false}`, field: "se_volume", expectedJSON: `50`},
					{name: "bgm_volumeだけ0のとき、応答のlanguageがjaのままになる", body: `{"bgm_volume":0}`, field: "language", expectedJSON: `"ja"`},
					{name: "bgm_volumeだけ0のとき、応答のse_volumeが50のままになる", body: `{"bgm_volume":0}`, field: "se_volume", expectedJSON: `50`},
					{name: "bgm_volumeだけ0のとき、応答のpush_enabledがtrueのままになる", body: `{"bgm_volume":0}`, field: "push_enabled", expectedJSON: `true`},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						rec := newSettingsTestServer().PutAs(t, routertest.TokenA, settingsPath, tc.body)
						handlertest.AssertJSONField(t, rec, tc.field, tc.expectedJSON)
					})
				}
			})
		})
		t.Run("異常系", func(t *testing.T) {
			t.Run("更新する項目が1つも指定されないとき", func(t *testing.T) {
				cases := []struct {
					name string
					body string
				}{
					{name: "空のオブジェクトのとき、更新項目の不足を示す400の応答になる", body: `{}`},
					{name: "4項目すべてがnullのとき、更新項目の不足を示す400の応答になる", body: `{"language":null,"bgm_volume":null,"se_volume":null,"push_enabled":null}`},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						rec := newSettingsTestServer().PutAs(t, routertest.TokenA, settingsPath, tc.body)
						handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "at least one settings field is required")
					})
				}
			})
			t.Run("リクエストボディが無いとき、リクエストボディ不正を示す400の応答になる", func(t *testing.T) {
				rec := newSettingsTestServer().PutAs(t, routertest.TokenA, settingsPath, "")
				handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "invalid request body")
			})
			t.Run("languageが空文字のとき、languageの不足を示す400の応答になる", func(t *testing.T) {
				rec := newSettingsTestServer().PutAs(t, routertest.TokenA, settingsPath, `{"language":""}`)
				handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "language is required")
			})
			t.Run("languageが10文字を超えるabcdefghijklmnopqrstのとき、languageの不正を示す400の応答になる", func(t *testing.T) {
				rec := newSettingsTestServer().PutAs(t, routertest.TokenA, settingsPath, `{"language":"abcdefghijklmnopqrst"}`)
				handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, "language is invalid")
			})
			t.Run("音量が範囲の外のとき", func(t *testing.T) {
				cases := []struct {
					name    string
					body    string
					message string
				}{
					{name: "bgm_volumeが-1のとき、bgm_volumeの不正を示す400の応答になる", body: `{"bgm_volume":-1}`, message: "bgm_volume is invalid"},
					{name: "bgm_volumeが101のとき、bgm_volumeの不正を示す400の応答になる", body: `{"bgm_volume":101}`, message: "bgm_volume is invalid"},
					{name: "se_volumeが-1のとき、se_volumeの不正を示す400の応答になる", body: `{"se_volume":-1}`, message: "se_volume is invalid"},
					{name: "se_volumeが101のとき、se_volumeの不正を示す400の応答になる", body: `{"se_volume":101}`, message: "se_volume is invalid"},
				}
				for _, tc := range cases {
					t.Run(tc.name, func(t *testing.T) {
						rec := newSettingsTestServer().PutAs(t, routertest.TokenA, settingsPath, tc.body)
						handlertest.AssertErrorResponse(t, rec, http.StatusBadRequest, tc.message)
					})
				}
			})
		})
	})
}
