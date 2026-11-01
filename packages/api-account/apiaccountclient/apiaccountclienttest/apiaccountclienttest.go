// Package apiaccountclienttest は、アカウントサービスのクライアントを使うテストが共有する部品と、偽のアカウントサービスを提供する。
package apiaccountclienttest

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
	"github.com/kenyamaneko/overload-party-account/packages/api-account/apiaccountclient"
)

// Captured は、複数のゴルーチンから安全に読み書きできる値の入れ物。
type Captured[T any] struct {
	mu    sync.Mutex
	value T
}

// Set は、入れ物の値を置き換える。
func (c *Captured[T]) Set(value T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = value
}

// Get は、入れ物の値を返す。
func (c *Captured[T]) Get() T {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

// ReceivedRequest は、テスト用サーバーが受け取ったリクエストのヘッダー。
type ReceivedRequest struct {
	Header http.Header
}

// RecordingServer は、最後に受け取ったリクエストを記録するテスト用サーバー。
type RecordingServer struct {
	*httptest.Server
	Received Captured[ReceivedRequest]
}

// NewRecordingServer は、指定したステータスと本文を JSON で返し、受け取ったリクエストを記録するテスト用サーバーを起動する。
func NewRecordingServer(t *testing.T, status int, responseBody any) *RecordingServer {
	t.Helper()
	recorder := &RecordingServer{}
	recorder.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorder.Received.Set(ReceivedRequest{Header: r.Header.Clone()})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		assert.NoError(t, json.NewEncoder(w).Encode(responseBody))
	}))
	t.Cleanup(recorder.Close)
	return recorder
}

// NewFakeClient は、偽のアカウントサービスを起動し、その基底 URL を指すクライアントと偽サーバーを返す。
func NewFakeClient(t *testing.T) (*apiaccountclient.Client, *FakeServer) {
	t.Helper()
	server := NewFakeServer()
	t.Cleanup(server.Close)
	client, err := apiaccountclient.New(server.URL())
	require.NoError(t, err)
	return client, server
}

// NewClient は、指定した基底 URL とオプションでクライアントを作成する。
func NewClient(t *testing.T, baseURL string, opts ...apiaccountclient.Option) *apiaccountclient.Client {
	t.Helper()
	client, err := apiaccountclient.New(baseURL, opts...)
	require.NoError(t, err)
	return client
}

// NewCanceledContext は、取り消し済みの context.Context を返す。
func NewCanceledContext() context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	return ctx
}

// NewDummyPlayer は、テスト用のダミー値を全項目に持つプレイヤー情報を返す。
func NewDummyPlayer() apiaccount.PlayerResponse {
	return apiaccount.PlayerResponse{
		CreatedAt:        time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		EquippedIconNo:   NewPointer(int64(7)),
		Exp:              450,
		FirebaseUID:      "firebase-uid-1",
		InitialFaction:   NewPointer("faction-a"),
		IsPremium:        true,
		Level:            2,
		LevelExpCurrent:  50,
		LevelExpRequired: 500,
		Name:             NewPointer("テスト太郎"),
		OnboardingStatus: apiaccount.OnboardingStatus("name_set"),
		PlayerID:         "player-1",
		PremiumExpiresAt: NewPointer(time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC)),
		UpdatedAt:        time.Date(2026, 1, 3, 4, 5, 6, 0, time.UTC),
	}
}

// NewPointer は、値のコピーを指すポインタを返す。
func NewPointer[T any](value T) *T {
	return &value
}
