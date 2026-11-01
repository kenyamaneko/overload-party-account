// Package apiaccountclient は、アカウントサービスの REST 契約の呼び出しを、
// 経路や HTTP ステータスを隠したエラー値つきの操作として提供する。
package apiaccountclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

const (
	// jsonMediaType は契約が成功応答の本文に定める種類。
	jsonMediaType = "application/json"

	// minServerErrorStatus と maxServerErrorStatus は、サーバー内部エラーを示すステータスの範囲。
	minServerErrorStatus = 500
	maxServerErrorStatus = 599
)

// 応答から判別できる原因ごとのエラー値。errors.Is で判別する。
var (
	// ErrNotFound は、アカウントサービスが 404 を返したことを示す。
	ErrNotFound = errors.New("apiaccountclient: not found")

	// ErrUnauthorized は、アカウントサービスが 401 を返したことを示す。
	ErrUnauthorized = errors.New("apiaccountclient: unauthorized")

	// ErrBadRequest は、アカウントサービスが 400 を返したことを示す。
	ErrBadRequest = errors.New("apiaccountclient: bad request")

	// ErrConflict は、アカウントサービスが 409 を返したことを示す。
	ErrConflict = errors.New("apiaccountclient: conflict")

	// ErrInternalServer は、アカウントサービスが 500 から 599 を返したことを示す。
	ErrInternalServer = errors.New("apiaccountclient: internal server error")

	// ErrConnectionFailed は、アカウントサービスに接続できなかったことを示す。
	ErrConnectionFailed = errors.New("apiaccountclient: connection failed")

	// ErrInvalidBody は、成功の応答の本文が契約の形式でないことを示す。
	ErrInvalidBody = errors.New("apiaccountclient: invalid response body")

	// ErrInvalidContentType は、成功の応答の Content-Type が契約の形式でないことを示す。
	ErrInvalidContentType = errors.New("apiaccountclient: invalid response content type")

	// ErrInvalidBaseURL は、基底URLが URL として解釈できない、またはスキームかホストが無いことを示す。
	ErrInvalidBaseURL = errors.New("apiaccountclient: invalid base URL")
)

// Client はアカウントサービスの REST 契約を呼び出す。
type Client struct {
	api *apiaccount.Client
}

// Option は Client の生成時に指定する設定。
type Option func(*config)

type config struct {
	httpClient apiaccount.HttpRequestDoer
	editors    []apiaccount.RequestEditorFn
}

// WithHTTPClient はリクエストの送信に使う HTTP クライアントを差し替える。
// 指定しないときは http.DefaultClient を使う。
func WithHTTPClient(doer apiaccount.HttpRequestDoer) Option {
	return func(c *config) { c.httpClient = doer }
}

// WithRequestEditorFn はすべてのリクエストを送信前に編集する関数を追加する。
// 指定した順に適用する。
func WithRequestEditorFn(fn apiaccount.RequestEditorFn) Option {
	return func(c *config) { c.editors = append(c.editors, fn) }
}

// New は baseURL のアカウントサービスに接続する Client を生成する。
// baseURL が URL として解釈できない、またはスキームかホストが無いときは ErrInvalidBaseURL を返す。
func New(baseURL string, opts ...Option) (*Client, error) {
	if err := validateBaseURL(baseURL); err != nil {
		return nil, err
	}

	cfg := &config{httpClient: http.DefaultClient}
	for _, opt := range opts {
		opt(cfg)
	}

	apiOpts := make([]apiaccount.ClientOption, 0, 1+len(cfg.editors))
	apiOpts = append(apiOpts, apiaccount.WithHTTPClient(connectionDoer{next: cfg.httpClient}))
	for _, editor := range cfg.editors {
		apiOpts = append(apiOpts, apiaccount.WithRequestEditorFn(editor))
	}

	api, err := apiaccount.NewClient(baseURL, apiOpts...)
	if err != nil {
		return nil, fmt.Errorf("apiaccountclient: new client: %w", err)
	}
	return &Client{api: api}, nil
}

// RegisterPlayer は新規プレイヤーを登録する。
func (c *Client) RegisterPlayer(ctx context.Context, req apiaccount.RegisterRequest) (*apiaccount.PlayerResponse, error) {
	return callJSON[apiaccount.PlayerResponse]("RegisterPlayer", http.StatusCreated, func() (*http.Response, error) {
		return c.api.RegisterPlayer(ctx, apiaccount.RegisterPlayerJSONRequestBody(req))
	})
}

// LoginPlayer は登録済みのプレイヤーをログインさせる。
func (c *Client) LoginPlayer(ctx context.Context, req apiaccount.LoginRequest) (*apiaccount.PlayerResponse, error) {
	return callJSON[apiaccount.PlayerResponse]("LoginPlayer", http.StatusOK, func() (*http.Response, error) {
		return c.api.LoginPlayer(ctx, apiaccount.LoginPlayerJSONRequestBody(req))
	})
}

// GetPlayerByFirebaseUID は Firebase UID のプレイヤーを検索する。
func (c *Client) GetPlayerByFirebaseUID(ctx context.Context, firebaseUID string) (*apiaccount.PlayerResponse, error) {
	return callJSON[apiaccount.PlayerResponse]("GetPlayerByFirebaseUID", http.StatusOK, func() (*http.Response, error) {
		return c.api.GetPlayerByFirebaseUID(ctx, firebaseUID)
	})
}

// AwardGameExp は対戦結果に応じた経験値を両プレイヤーに付与する。
func (c *Client) AwardGameExp(ctx context.Context, req apiaccount.AwardGameExpRequest) error {
	return callWithoutBody("AwardGameExp", http.StatusNoContent, func() (*http.Response, error) {
		return c.api.AwardGameExp(ctx, apiaccount.AwardGameExpJSONRequestBody(req))
	})
}

// RevertBattleCount は停止で無効になった対戦の消費バトル回数を両プレイヤーに戻す。
func (c *Client) RevertBattleCount(ctx context.Context, req apiaccount.RevertBattleCountRequest) error {
	return callWithoutBody("RevertBattleCount", http.StatusNoContent, func() (*http.Response, error) {
		return c.api.RevertBattleCount(ctx, apiaccount.RevertBattleCountJSONRequestBody(req))
	})
}

// GetPlayer は呼び出し元のプレイヤー自身の情報を返す。
func (c *Client) GetPlayer(ctx context.Context) (*apiaccount.PlayerResponse, error) {
	return callJSON[apiaccount.PlayerResponse]("GetPlayer", http.StatusOK, func() (*http.Response, error) {
		return c.api.GetPlayer(ctx)
	})
}

// ValidateNameForOnboarding はオンボーディングで入力された表示名を、保存せずに検証する。
func (c *Client) ValidateNameForOnboarding(ctx context.Context, req apiaccount.ValidateNameForOnboardingRequest) error {
	return callWithoutBody("ValidateNameForOnboarding", http.StatusNoContent, func() (*http.Response, error) {
		return c.api.ValidateNameForOnboarding(ctx, apiaccount.ValidateNameForOnboardingJSONRequestBody(req))
	})
}

// GetBattleLimit は 1 日のバトル回数制限と残り回数を返す。
func (c *Client) GetBattleLimit(ctx context.Context) (*apiaccount.BattleLimitResponse, error) {
	return callJSON[apiaccount.BattleLimitResponse]("GetBattleLimit", http.StatusOK, func() (*http.Response, error) {
		return c.api.GetBattleLimit(ctx)
	})
}

// IncrementBattleCount は当日のバトル回数を 1 増やす。
func (c *Client) IncrementBattleCount(ctx context.Context) error {
	return callWithoutBody("IncrementBattleCount", http.StatusNoContent, func() (*http.Response, error) {
		return c.api.IncrementBattleCount(ctx)
	})
}

// ListFactions はプレイヤーが所持する陣営の一覧を返す。
func (c *Client) ListFactions(ctx context.Context) (*apiaccount.FactionListing, error) {
	return callJSON[apiaccount.FactionListing]("ListFactions", http.StatusOK, func() (*http.Response, error) {
		return c.api.ListFactions(ctx)
	})
}

// connectionDoer は、リクエストの送信に失敗したエラーに ErrConnectionFailed を付ける。
type connectionDoer struct {
	next apiaccount.HttpRequestDoer
}

func (d connectionDoer) Do(req *http.Request) (*http.Response, error) {
	resp, err := d.next.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrConnectionFailed, err)
	}
	return resp, nil
}

func validateBaseURL(baseURL string) error {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrInvalidBaseURL, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%w: %q has no scheme or host", ErrInvalidBaseURL, baseURL)
	}
	return nil
}

// callJSON は send の応答が wantStatus のとき、JSON の本文を T に読み取って返す。
func callJSON[T any](op string, wantStatus int, send func() (*http.Response, error)) (*T, error) {
	resp, err := send()
	if err != nil {
		return nil, fmt.Errorf("apiaccountclient: %s: %w", op, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		return nil, newStatusError(op, resp.StatusCode)
	}
	if !isJSONContentType(resp.Header.Get("Content-Type")) {
		return nil, fmt.Errorf("apiaccountclient: %s: %w: %q", op, ErrInvalidContentType, resp.Header.Get("Content-Type"))
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("apiaccountclient: %s: read body: %w: %w", op, ErrConnectionFailed, err)
	}
	var body T
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("apiaccountclient: %s: %w: %w", op, ErrInvalidBody, err)
	}
	return &body, nil
}

// callWithoutBody は send の応答が wantStatus のとき、本文を読まずに成功とする。
func callWithoutBody(op string, wantStatus int, send func() (*http.Response, error)) error {
	resp, err := send()
	if err != nil {
		return fmt.Errorf("apiaccountclient: %s: %w", op, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != wantStatus {
		return newStatusError(op, resp.StatusCode)
	}
	return nil
}

func isJSONContentType(contentType string) bool {
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == jsonMediaType
}

// newStatusError は想定外の HTTP ステータスを、errors.Is で判別できるエラーに変換する。
// 契約に無いステータスは、ステータスコードを含むエラーにする。
func newStatusError(op string, code int) error {
	var cause error
	switch {
	case code == http.StatusUnauthorized:
		cause = ErrUnauthorized
	case code == http.StatusNotFound:
		cause = ErrNotFound
	case code == http.StatusBadRequest:
		cause = ErrBadRequest
	case code == http.StatusConflict:
		cause = ErrConflict
	case code >= minServerErrorStatus && code <= maxServerErrorStatus:
		cause = ErrInternalServer
	default:
		return fmt.Errorf("apiaccountclient: %s: unexpected status %d", op, code)
	}
	return fmt.Errorf("apiaccountclient: %s: %w", op, cause)
}
