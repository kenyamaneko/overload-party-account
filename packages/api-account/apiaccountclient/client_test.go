package apiaccountclient_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
	"github.com/kenyamaneko/overload-party-account/packages/api-account/apiaccountclient"
)

// stubDoer は apiaccount.HttpRequestDoer を満たし、実ネットワークを介さず固定の応答を返す
// HTTP 境界のテストダブル。
type stubDoer struct {
	status      int
	contentType string
	body        []byte
}

func (d stubDoer) Do(req *http.Request) (*http.Response, error) {
	header := http.Header{}
	if d.contentType != "" {
		header.Set("Content-Type", d.contentType)
	}
	return &http.Response{
		StatusCode: d.status,
		Header:     header,
		Body:       io.NopCloser(bytes.NewReader(d.body)),
	}, nil
}

func newTestClient(t *testing.T, status int, contentType string, body []byte) *apiaccountclient.Client {
	t.Helper()
	c, err := apiaccountclient.New("http://example.invalid", apiaccountclient.WithHTTPClient(stubDoer{
		status:      status,
		contentType: contentType,
		body:        body,
	}))
	require.NoError(t, err)
	return c
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

func TestClient_StatusToSentinelError(t *testing.T) {
	t.Run("共通のステータス→sentinel error変換", func(t *testing.T) {
		cases := []struct {
			name    string
			status  int
			wantErr error
		}{
			{"レスポンスが401のとき、ErrUnauthorizedを返す", 401, apiaccountclient.ErrUnauthorized},
			{"レスポンスが404のとき、ErrNotFoundを返す", 404, apiaccountclient.ErrNotFound},
			{"レスポンスが400のとき、ErrBadRequestを返す", 400, apiaccountclient.ErrBadRequest},
			{"レスポンスが409のとき、ErrConflictを返す", 409, apiaccountclient.ErrConflict},
			{"レスポンスが500のとき、ErrInternalServerを返す", 500, apiaccountclient.ErrInternalServer},
			{"レスポンスが503(500以上)のとき、ErrInternalServerを返す", 503, apiaccountclient.ErrInternalServer},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				c := newTestClient(t, tc.status, "", nil)

				_, err := c.GetPlayer(context.Background())

				assert.ErrorIs(t, err, tc.wantErr)
			})
		}

		t.Run("401/404/400/409/500以上のいずれでもない想定外のステータスのとき、いずれのsentinelでもないエラーを返す", func(t *testing.T) {
			c := newTestClient(t, 418, "", nil)

			_, err := c.GetPlayer(context.Background())

			require.Error(t, err)
			assert.NotErrorIs(t, err, apiaccountclient.ErrUnauthorized)
			assert.NotErrorIs(t, err, apiaccountclient.ErrNotFound)
			assert.NotErrorIs(t, err, apiaccountclient.ErrBadRequest)
			assert.NotErrorIs(t, err, apiaccountclient.ErrConflict)
			assert.NotErrorIs(t, err, apiaccountclient.ErrInternalServer)
		})
	})
}

func TestClient_SuccessResponses(t *testing.T) {
	t.Run("各メソッドの成功時の戻り値", func(t *testing.T) {
		t.Run("GetHealthは200のとき、ヘルスレスポンスを返す", func(t *testing.T) {
			c := newTestClient(t, 200, "application/json", mustJSON(t, apiaccount.HealthResponse{Status: "ok"}))

			resp, err := c.GetHealth(context.Background())

			require.NoError(t, err)
			assert.Equal(t, "ok", resp.Status)
		})

		t.Run("RegisterPlayerは201のとき、登録されたプレイヤー情報を返す", func(t *testing.T) {
			c := newTestClient(t, 201, "application/json", mustJSON(t, apiaccount.PlayerResponse{PlayerID: "player-1"}))

			resp, err := c.RegisterPlayer(context.Background(), apiaccount.RegisterRequest{FirebaseUID: "fb-1"})

			require.NoError(t, err)
			assert.Equal(t, "player-1", resp.PlayerID)
		})

		t.Run("LoginPlayerは200のとき、プレイヤー情報を返す", func(t *testing.T) {
			c := newTestClient(t, 200, "application/json", mustJSON(t, apiaccount.PlayerResponse{PlayerID: "player-1"}))

			resp, err := c.LoginPlayer(context.Background(), apiaccount.LoginRequest{FirebaseUID: "fb-1"})

			require.NoError(t, err)
			assert.Equal(t, "player-1", resp.PlayerID)
		})

		t.Run("GetPlayerByFirebaseUIDは200のとき、プレイヤー情報を返す", func(t *testing.T) {
			c := newTestClient(t, 200, "application/json", mustJSON(t, apiaccount.PlayerResponse{PlayerID: "player-1"}))

			resp, err := c.GetPlayerByFirebaseUID(context.Background(), "fb-1")

			require.NoError(t, err)
			assert.Equal(t, "player-1", resp.PlayerID)
		})

		t.Run("AwardGameExpは204のとき、エラーを返さない", func(t *testing.T) {
			c := newTestClient(t, 204, "", nil)

			err := c.AwardGameExp(context.Background(), apiaccount.AwardGameExpRequest{
				Player1ID: "p1", Player2ID: "p2", WinnerNum: 1, Reason: "normal", MatchType: "pvp",
			})

			require.NoError(t, err)
		})

		t.Run("RevertBattleCountは204のとき、エラーを返さない", func(t *testing.T) {
			c := newTestClient(t, 204, "", nil)

			err := c.RevertBattleCount(context.Background(), apiaccount.RevertBattleCountRequest{
				GameID: "game-1", Player1ID: "p1", Player2ID: "p2", ConsumedAtMillis: 1000,
			})

			require.NoError(t, err)
		})

		t.Run("GetPlayerは200のとき、プレイヤー情報を返す", func(t *testing.T) {
			c := newTestClient(t, 200, "application/json", mustJSON(t, apiaccount.PlayerResponse{PlayerID: "player-1"}))

			resp, err := c.GetPlayer(context.Background())

			require.NoError(t, err)
			assert.Equal(t, "player-1", resp.PlayerID)
		})

		t.Run("UpdateNameは200のとき、更新後のプレイヤー情報を返す", func(t *testing.T) {
			c := newTestClient(t, 200, "application/json", mustJSON(t, apiaccount.PlayerResponse{PlayerID: "player-1"}))

			resp, err := c.UpdateName(context.Background(), apiaccount.UpdateNameRequest{Name: "新しい名前"})

			require.NoError(t, err)
			assert.Equal(t, "player-1", resp.PlayerID)
		})

		t.Run("ValidateNameForOnboardingは204のとき、エラーを返さない", func(t *testing.T) {
			c := newTestClient(t, 204, "", nil)

			err := c.ValidateNameForOnboarding(context.Background(), apiaccount.ValidateNameForOnboardingRequest{Name: "名前"})

			require.NoError(t, err)
		})

		t.Run("GetBattleLimitは200のとき、バトル制限情報を返す", func(t *testing.T) {
			c := newTestClient(t, 200, "application/json", mustJSON(t, apiaccount.BattleLimitResponse{
				DailyBattleCount: 2, DailyBattleLimit: 5, CanBattle: true,
			}))

			resp, err := c.GetBattleLimit(context.Background())

			require.NoError(t, err)
			assert.Equal(t, int64(2), resp.DailyBattleCount)
		})

		t.Run("IncrementBattleCountは204のとき、エラーを返さない", func(t *testing.T) {
			c := newTestClient(t, 204, "", nil)

			err := c.IncrementBattleCount(context.Background())

			require.NoError(t, err)
		})

		t.Run("UpdatePremiumは204のとき、エラーを返さない", func(t *testing.T) {
			c := newTestClient(t, 204, "", nil)

			err := c.UpdatePremium(context.Background(), apiaccount.UpdatePremiumRequest{IsPremium: true})

			require.NoError(t, err)
		})

		t.Run("AddExpは204のとき、エラーを返さない", func(t *testing.T) {
			c := newTestClient(t, 204, "", nil)

			err := c.AddExp(context.Background(), apiaccount.AddExpRequest{ExpGain: 10})

			require.NoError(t, err)
		})

		t.Run("ListFactionsは200のとき、所持ファクション一覧を返す", func(t *testing.T) {
			c := newTestClient(t, 200, "application/json", mustJSON(t, apiaccount.FactionListing{Factions: []string{"SHE"}}))

			resp, err := c.ListFactions(context.Background())

			require.NoError(t, err)
			assert.Equal(t, []string{"SHE"}, resp.Factions)
		})

		t.Run("GrantFactionは204のとき、エラーを返さない", func(t *testing.T) {
			c := newTestClient(t, 204, "", nil)

			err := c.GrantFaction(context.Background(), apiaccount.FactionGrantRequest{Faction: "SHE"})

			require.NoError(t, err)
		})

		t.Run("SelectInitialFactionは200(204ではない)のとき、エラーを返さない", func(t *testing.T) {
			c := newTestClient(t, 200, "", nil)

			err := c.SelectInitialFaction(context.Background(), apiaccount.SelectInitialFactionRequest{FactionID: "SHE"})

			require.NoError(t, err)
		})

		t.Run("GetPlayerSettingsは200のとき、設定情報を返す", func(t *testing.T) {
			c := newTestClient(t, 200, "application/json", mustJSON(t, apiaccount.PlayerSettingsResponse{PlayerID: "player-1", Language: "ja"}))

			resp, err := c.GetPlayerSettings(context.Background())

			require.NoError(t, err)
			assert.Equal(t, "player-1", resp.PlayerID)
		})

		t.Run("UpdatePlayerSettingsは200のとき、更新後の設定情報を返す", func(t *testing.T) {
			c := newTestClient(t, 200, "application/json", mustJSON(t, apiaccount.PlayerSettingsResponse{PlayerID: "player-1", Language: "en"}))

			lang := "en"
			resp, err := c.UpdatePlayerSettings(context.Background(), apiaccount.UpdateSettingsRequest{Language: &lang})

			require.NoError(t, err)
			assert.Equal(t, "en", resp.Language)
		})
	})
}

func TestClient_NonSuccessAppliesCommonConversion(t *testing.T) {
	t.Run("成功ステータス以外が返ったとき、共通変換規則に従ってエラーを返す", func(t *testing.T) {
		cases := []struct {
			name   string
			invoke func(c *apiaccountclient.Client) error
		}{
			{"RegisterPlayer", func(c *apiaccountclient.Client) error {
				_, err := c.RegisterPlayer(context.Background(), apiaccount.RegisterRequest{FirebaseUID: "fb-1"})
				return err
			}},
			{"LoginPlayer", func(c *apiaccountclient.Client) error {
				_, err := c.LoginPlayer(context.Background(), apiaccount.LoginRequest{FirebaseUID: "fb-1"})
				return err
			}},
			{"GetPlayerByFirebaseUID", func(c *apiaccountclient.Client) error {
				_, err := c.GetPlayerByFirebaseUID(context.Background(), "fb-1")
				return err
			}},
			{"AwardGameExp", func(c *apiaccountclient.Client) error {
				return c.AwardGameExp(context.Background(), apiaccount.AwardGameExpRequest{Player1ID: "p1", Player2ID: "p2", WinnerNum: 1, Reason: "normal", MatchType: "pvp"})
			}},
			{"RevertBattleCount", func(c *apiaccountclient.Client) error {
				return c.RevertBattleCount(context.Background(), apiaccount.RevertBattleCountRequest{GameID: "g1", Player1ID: "p1", Player2ID: "p2", ConsumedAtMillis: 1000})
			}},
			{"GetHealth", func(c *apiaccountclient.Client) error {
				_, err := c.GetHealth(context.Background())
				return err
			}},
			{"UpdateName", func(c *apiaccountclient.Client) error {
				_, err := c.UpdateName(context.Background(), apiaccount.UpdateNameRequest{Name: "名前"})
				return err
			}},
			{"ValidateNameForOnboarding", func(c *apiaccountclient.Client) error {
				return c.ValidateNameForOnboarding(context.Background(), apiaccount.ValidateNameForOnboardingRequest{Name: "名前"})
			}},
			{"GetBattleLimit", func(c *apiaccountclient.Client) error {
				_, err := c.GetBattleLimit(context.Background())
				return err
			}},
			{"IncrementBattleCount", func(c *apiaccountclient.Client) error {
				return c.IncrementBattleCount(context.Background())
			}},
			{"UpdatePremium", func(c *apiaccountclient.Client) error {
				return c.UpdatePremium(context.Background(), apiaccount.UpdatePremiumRequest{IsPremium: true})
			}},
			{"AddExp", func(c *apiaccountclient.Client) error {
				return c.AddExp(context.Background(), apiaccount.AddExpRequest{ExpGain: 10})
			}},
			{"ListFactions", func(c *apiaccountclient.Client) error {
				_, err := c.ListFactions(context.Background())
				return err
			}},
			{"GrantFaction", func(c *apiaccountclient.Client) error {
				return c.GrantFaction(context.Background(), apiaccount.FactionGrantRequest{Faction: "SHE"})
			}},
			{"SelectInitialFaction", func(c *apiaccountclient.Client) error {
				return c.SelectInitialFaction(context.Background(), apiaccount.SelectInitialFactionRequest{FactionID: "SHE"})
			}},
			{"GetPlayerSettings", func(c *apiaccountclient.Client) error {
				_, err := c.GetPlayerSettings(context.Background())
				return err
			}},
			{"UpdatePlayerSettings", func(c *apiaccountclient.Client) error {
				_, err := c.UpdatePlayerSettings(context.Background(), apiaccount.UpdateSettingsRequest{})
				return err
			}},
		}

		for _, tc := range cases {
			t.Run(tc.name+"が404を受け取ったとき、ErrNotFoundを返す", func(t *testing.T) {
				c := newTestClient(t, 404, "", nil)

				err := tc.invoke(c)

				assert.ErrorIs(t, err, apiaccountclient.ErrNotFound)
			})
		}
	})
}

func TestClient_EndpointSpecificSuccessStatusIsNotInterchangeable(t *testing.T) {
	t.Run("成功ステータスはメソッドごとに固有で、他の2xxを成功として扱わない", func(t *testing.T) {
		t.Run("RegisterPlayerは200を受け取ったとき、成功として扱わずエラーを返す", func(t *testing.T) {
			c := newTestClient(t, 200, "application/json", mustJSON(t, apiaccount.PlayerResponse{PlayerID: "player-1"}))

			_, err := c.RegisterPlayer(context.Background(), apiaccount.RegisterRequest{FirebaseUID: "fb-1"})

			require.Error(t, err)
		})

		t.Run("SelectInitialFactionは204を受け取ったとき、成功として扱わずエラーを返す", func(t *testing.T) {
			c := newTestClient(t, 204, "", nil)

			err := c.SelectInitialFaction(context.Background(), apiaccount.SelectInitialFactionRequest{FactionID: "SHE"})

			require.Error(t, err)
		})
	})
}
