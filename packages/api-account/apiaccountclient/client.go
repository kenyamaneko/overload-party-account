// Package apiaccountclient は consumer から wire 詳細 (REST path / HTTP status code) を
// 隠蔽するため、生成 client を sentinel error 変換でラップする。
package apiaccountclient

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// Sentinel errors. wire 契約 (OpenAPI spec) で定義された status code の意味を sentinel として export する。
var (
	// ErrNotFound は status 404。
	ErrNotFound = errors.New("apiaccountclient: not found")

	// ErrUnauthorized は status 401。
	ErrUnauthorized = errors.New("apiaccountclient: unauthorized")

	// ErrForbidden は status 403。
	ErrForbidden = errors.New("apiaccountclient: forbidden")

	// ErrBadRequest は status 400。
	ErrBadRequest = errors.New("apiaccountclient: bad request")

	// ErrConflict は status 409。registerPlayer で重複登録時に返る。
	ErrConflict = errors.New("apiaccountclient: conflict")

	// ErrInternalServer は status 5xx。
	ErrInternalServer = errors.New("apiaccountclient: internal server error")
)

// Client は overload-party-account REST API の sentinel-error converted client。
type Client struct {
	api *apiaccount.ClientWithResponses
}

// Option は Client 構築時の設定。
type Option func(*config)

type config struct {
	httpClient apiaccount.HttpRequestDoer
	editors    []apiaccount.RequestEditorFn
}

// WithHTTPClient は基底 HTTP doer を差し替える。デフォルトは http.DefaultClient。
func WithHTTPClient(doer apiaccount.HttpRequestDoer) Option {
	return func(c *config) { c.httpClient = doer }
}

// WithRequestEditorFn は全リクエストに適用する RequestEditor を追加する。
func WithRequestEditorFn(fn apiaccount.RequestEditorFn) Option {
	return func(c *config) { c.editors = append(c.editors, fn) }
}

// New は baseURL に接続する Client を生成する。
func New(baseURL string, opts ...Option) (*Client, error) {
	cfg := &config{}
	for _, opt := range opts {
		opt(cfg)
	}

	apiOpts := make([]apiaccount.ClientOption, 0, 1+len(cfg.editors))
	if cfg.httpClient != nil {
		apiOpts = append(apiOpts, apiaccount.WithHTTPClient(cfg.httpClient))
	}
	for _, ed := range cfg.editors {
		apiOpts = append(apiOpts, apiaccount.WithRequestEditorFn(ed))
	}

	api, err := apiaccount.NewClientWithResponses(baseURL, apiOpts...)
	if err != nil {
		return nil, fmt.Errorf("apiaccountclient: new: %w", err)
	}
	return &Client{api: api}, nil
}

// GetHealth はサービス死活監視用エンドポイントを叩く。
func (c *Client) GetHealth(ctx context.Context) (*apiaccount.HealthResponse, error) {
	resp, err := c.api.GetHealthWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("apiaccountclient: GetHealth: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, newStatusError("GetHealth", resp.StatusCode())
}

// RegisterPlayer は新規プレイヤーを登録する。spec は 201 Created を返す。
func (c *Client) RegisterPlayer(ctx context.Context, req apiaccount.RegisterRequest) (*apiaccount.PlayerResponse, error) {
	resp, err := c.api.RegisterPlayerWithResponse(ctx, apiaccount.RegisterPlayerJSONRequestBody(req))
	if err != nil {
		return nil, fmt.Errorf("apiaccountclient: RegisterPlayer: %w", err)
	}
	if resp.JSON201 != nil {
		return resp.JSON201, nil
	}
	return nil, newStatusError("RegisterPlayer", resp.StatusCode())
}

// LoginPlayer は既存プレイヤーをログインさせる。
func (c *Client) LoginPlayer(ctx context.Context, req apiaccount.LoginRequest) (*apiaccount.PlayerResponse, error) {
	resp, err := c.api.LoginPlayerWithResponse(ctx, apiaccount.LoginPlayerJSONRequestBody(req))
	if err != nil {
		return nil, fmt.Errorf("apiaccountclient: LoginPlayer: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, newStatusError("LoginPlayer", resp.StatusCode())
}

// GetPlayerByFirebaseUID は Firebase UID で player を引く。
func (c *Client) GetPlayerByFirebaseUID(ctx context.Context, firebaseUID string) (*apiaccount.PlayerResponse, error) {
	resp, err := c.api.GetPlayerByFirebaseUIDWithResponse(ctx, firebaseUID)
	if err != nil {
		return nil, fmt.Errorf("apiaccountclient: GetPlayerByFirebaseUID: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, newStatusError("GetPlayerByFirebaseUID", resp.StatusCode())
}

// AwardGameExp はゲーム結果に基づき player に exp を付与する。spec は 204 を返す。
func (c *Client) AwardGameExp(ctx context.Context, req apiaccount.AwardGameExpRequest) error {
	resp, err := c.api.AwardGameExpWithResponse(ctx, apiaccount.AwardGameExpJSONRequestBody(req))
	if err != nil {
		return fmt.Errorf("apiaccountclient: AwardGameExp: %w", err)
	}
	if resp.StatusCode() == http.StatusNoContent {
		return nil
	}
	return newStatusError("AwardGameExp", resp.StatusCode())
}

// GetPlayer は呼出元 player 自身の情報を返す (X-Internal-Auth の sub から解決)。
func (c *Client) GetPlayer(ctx context.Context) (*apiaccount.PlayerResponse, error) {
	resp, err := c.api.GetPlayerWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("apiaccountclient: GetPlayer: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, newStatusError("GetPlayer", resp.StatusCode())
}

// UpdateName は player 表示名を更新する。
func (c *Client) UpdateName(ctx context.Context, req apiaccount.UpdateNameRequest) (*apiaccount.PlayerResponse, error) {
	resp, err := c.api.UpdateNameWithResponse(ctx, apiaccount.UpdateNameJSONRequestBody(req))
	if err != nil {
		return nil, fmt.Errorf("apiaccountclient: UpdateName: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, newStatusError("UpdateName", resp.StatusCode())
}

// ValidateNameForOnboarding は表示名のバリデーションを問い合わせる (書き込みなし)。spec は 204 を返す。
func (c *Client) ValidateNameForOnboarding(ctx context.Context, req apiaccount.ValidateNameForOnboardingRequest) error {
	resp, err := c.api.ValidateNameForOnboardingWithResponse(ctx, apiaccount.ValidateNameForOnboardingJSONRequestBody(req))
	if err != nil {
		return fmt.Errorf("apiaccountclient: ValidateNameForOnboarding: %w", err)
	}
	if resp.StatusCode() == http.StatusNoContent {
		return nil
	}
	return newStatusError("ValidateNameForOnboarding", resp.StatusCode())
}

// GetBattleLimit は 1 日のバトル回数制限と残り回数を返す。
func (c *Client) GetBattleLimit(ctx context.Context) (*apiaccount.BattleLimitResponse, error) {
	resp, err := c.api.GetBattleLimitWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("apiaccountclient: GetBattleLimit: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, newStatusError("GetBattleLimit", resp.StatusCode())
}

// IncrementBattleCount は当日のバトルカウントをインクリメントする。spec は 204 を返す。
func (c *Client) IncrementBattleCount(ctx context.Context) error {
	resp, err := c.api.IncrementBattleCountWithResponse(ctx)
	if err != nil {
		return fmt.Errorf("apiaccountclient: IncrementBattleCount: %w", err)
	}
	if resp.StatusCode() == http.StatusNoContent {
		return nil
	}
	return newStatusError("IncrementBattleCount", resp.StatusCode())
}

// UpdatePremium は player の premium 状態を更新する。spec は 204 を返す。
func (c *Client) UpdatePremium(ctx context.Context, req apiaccount.UpdatePremiumRequest) error {
	resp, err := c.api.UpdatePremiumWithResponse(ctx, apiaccount.UpdatePremiumJSONRequestBody(req))
	if err != nil {
		return fmt.Errorf("apiaccountclient: UpdatePremium: %w", err)
	}
	if resp.StatusCode() == http.StatusNoContent {
		return nil
	}
	return newStatusError("UpdatePremium", resp.StatusCode())
}

// AddExp は exp を加算する (主に dev 用)。spec は 204 を返す。
func (c *Client) AddExp(ctx context.Context, req apiaccount.AddExpRequest) error {
	resp, err := c.api.AddExpWithResponse(ctx, apiaccount.AddExpJSONRequestBody(req))
	if err != nil {
		return fmt.Errorf("apiaccountclient: AddExp: %w", err)
	}
	if resp.StatusCode() == http.StatusNoContent {
		return nil
	}
	return newStatusError("AddExp", resp.StatusCode())
}

// ListFactions は player が所有する faction 一覧を返す。
func (c *Client) ListFactions(ctx context.Context) (*apiaccount.FactionListing, error) {
	resp, err := c.api.ListFactionsWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("apiaccountclient: ListFactions: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, newStatusError("ListFactions", resp.StatusCode())
}

// GrantFaction は faction を player に付与する。spec は 204 を返す。
func (c *Client) GrantFaction(ctx context.Context, req apiaccount.FactionGrantRequest) error {
	resp, err := c.api.GrantFactionWithResponse(ctx, apiaccount.GrantFactionJSONRequestBody(req))
	if err != nil {
		return fmt.Errorf("apiaccountclient: GrantFaction: %w", err)
	}
	if resp.StatusCode() == http.StatusNoContent {
		return nil
	}
	return newStatusError("GrantFaction", resp.StatusCode())
}

// SelectInitialFaction はオンボーディングで初期 faction を選択する。spec は 200 を返す。
func (c *Client) SelectInitialFaction(ctx context.Context, req apiaccount.SelectInitialFactionRequest) error {
	resp, err := c.api.SelectInitialFactionWithResponse(ctx, apiaccount.SelectInitialFactionJSONRequestBody(req))
	if err != nil {
		return fmt.Errorf("apiaccountclient: SelectInitialFaction: %w", err)
	}
	if resp.StatusCode() == http.StatusOK {
		return nil
	}
	return newStatusError("SelectInitialFaction", resp.StatusCode())
}

// GetPlayerSettings は player の設定値を返す。
func (c *Client) GetPlayerSettings(ctx context.Context) (*apiaccount.PlayerSettingsResponse, error) {
	resp, err := c.api.GetPlayerSettingsWithResponse(ctx)
	if err != nil {
		return nil, fmt.Errorf("apiaccountclient: GetPlayerSettings: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, newStatusError("GetPlayerSettings", resp.StatusCode())
}

// UpdatePlayerSettings は player の設定値を更新する。
func (c *Client) UpdatePlayerSettings(ctx context.Context, req apiaccount.UpdateSettingsRequest) (*apiaccount.PlayerSettingsResponse, error) {
	resp, err := c.api.UpdatePlayerSettingsWithResponse(ctx, apiaccount.UpdatePlayerSettingsJSONRequestBody(req))
	if err != nil {
		return nil, fmt.Errorf("apiaccountclient: UpdatePlayerSettings: %w", err)
	}
	if resp.JSON200 != nil {
		return resp.JSON200, nil
	}
	return nil, newStatusError("UpdatePlayerSettings", resp.StatusCode())
}

// newStatusError は HTTP status code を sentinel error (errors.Is 分岐可能) に変換する。
func newStatusError(op string, code int) error {
	var sentinel error
	switch {
	case code == http.StatusUnauthorized:
		sentinel = ErrUnauthorized
	case code == http.StatusForbidden:
		sentinel = ErrForbidden
	case code == http.StatusNotFound:
		sentinel = ErrNotFound
	case code == http.StatusBadRequest:
		sentinel = ErrBadRequest
	case code == http.StatusConflict:
		sentinel = ErrConflict
	case code >= http.StatusInternalServerError:
		sentinel = ErrInternalServer
	default:
		return fmt.Errorf("apiaccountclient: %s: unexpected status %d", op, code)
	}
	return fmt.Errorf("apiaccountclient: %s: %w", op, sentinel)
}
