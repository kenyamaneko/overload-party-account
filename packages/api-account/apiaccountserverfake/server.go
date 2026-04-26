// Package apiaccountserverfake は account サービスの HTTP 契約を実装する
// httptest.Server ラッパー。consumer (gateway 等) が accountclient を使う
// handler テストで、実 account サービスを起動せずに REST 呼び出しを検証する
// ためのテストダブルを提供する。
//
// 各 endpoint は Fn field (func callback) で status + response body を制御する。
// Fn が nil の endpoint は既定値を返す (happy-path を仮定した最低限の応答)。
//
// Request / Response 型は api-account の公開型 (apiaccount.RegisterRequest /
// Player / PlayerResponse 等) を再利用するため、本パッケージは自前の型を
// 宣言していない。
//
// Go 1.22 ServeMux は `/players/{playerID}/{resource}` 配下のパターン衝突を
// 検知して登録を拒否することがあるため、`/players/` 配下は 1 つの catch-all
// handler で受け、path を自前で dispatch する。
package apiaccountserverfake

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// Server は account HTTP 契約を実装する httptest.Server wrapper。
type Server struct {
	mu  sync.Mutex
	srv *httptest.Server

	// RegisterFn: POST /internal/v1/auth/register。既定は 201 + 空 Player。
	RegisterFn func(req apiaccount.RegisterRequest) (int, any)

	// LoginFn: POST /internal/v1/auth/login。既定は 200 + 空 Player。
	LoginFn func(req apiaccount.LoginRequest) (int, any)

	// FindByFirebaseUIDFn: GET /internal/v1/auth/by-firebase-uid/{uid}。
	// 既定は 200 + 空 Player。未登録を擬似したい場合は Fn で 404 を返す。
	FindByFirebaseUIDFn func(firebaseUID string) (int, any)

	// GetPlayerFn: GET /internal/v1/players/{playerID}。既定は 200 + 空 PlayerResponse。
	GetPlayerFn func(playerID string) (int, any)

	// UpdateNameFn: PUT /internal/v1/players/{playerID}/name。既定は 200 + 空 Player。
	UpdateNameFn func(playerID string, req apiaccount.UpdateNameRequest) (int, any)

	// GetBattleLimitFn: GET /internal/v1/players/{playerID}/battle-limit。
	// 既定は 200 + 空 BattleLimitResponse。
	GetBattleLimitFn func(playerID string) (int, any)

	// IncrementBattleCountFn: POST /internal/v1/players/{playerID}/battle-limit/increment。
	// 既定は 204 No Content。
	IncrementBattleCountFn func(playerID string) (int, any)

	// UpdatePremiumFn: PUT /internal/v1/players/{playerID}/premium。既定は 204 No Content。
	UpdatePremiumFn func(playerID string, req apiaccount.UpdatePremiumRequest) (int, any)

	// UpdateFactionFn: PUT /internal/v1/players/{playerID}/faction。既定は 204 No Content。
	UpdateFactionFn func(playerID string, req apiaccount.UpdateFactionRequest) (int, any)

	// GrantFactionFn: POST /internal/v1/players/{playerID}/factions。既定は 204 No Content。
	GrantFactionFn func(playerID string, req apiaccount.FactionGrantRequest) (int, any)

	// ListFactionsFn: GET /internal/v1/players/{playerID}/factions。
	// 既定は 200 + 空の ListFactionsResponse。
	ListFactionsFn func(playerID string) (int, any)

	// AddExpFn: POST /internal/v1/players/{playerID}/exp。既定は 204 No Content。
	AddExpFn func(playerID string, req apiaccount.AddExpRequest) (int, any)

	// AwardGameExpFn: POST /internal/v1/players/award-game-exp。既定は 204 No Content。
	AwardGameExpFn func(req apiaccount.AwardGameExpRequest) (int, any)

	// GetSettingsFn: GET /internal/v1/players/{playerID}/settings。既定は 200 + 空 PlayerSettings。
	GetSettingsFn func(playerID string) (int, any)

	// UpdateSettingsFn: PUT /internal/v1/players/{playerID}/settings。既定は 200 + 空 PlayerSettings。
	UpdateSettingsFn func(playerID string, req apiaccount.UpdateSettingsRequest) (int, any)
}

// NewServer は起動済み Server を返す。テスト終了時に Close() すること。
func NewServer() *Server {
	s := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /internal/v1/auth/register", s.handleRegister)
	mux.HandleFunc("POST /internal/v1/auth/login", s.handleLogin)
	mux.HandleFunc("GET /internal/v1/auth/by-firebase-uid/{firebaseUID}", s.handleFindByFirebaseUIDRoute)
	// /players/ 配下はパターン衝突回避のため manual dispatch する。
	mux.HandleFunc("/internal/v1/players/", s.playersDispatcher)
	s.srv = httptest.NewServer(mux)
	return s
}

// URL は httptest.Server のベース URL を返す。
func (s *Server) URL() string { return s.srv.URL }

// Close は内部 httptest.Server を閉じる。
func (s *Server) Close() { s.srv.Close() }

// handleFindByFirebaseUIDRoute は GET /internal/v1/auth/by-firebase-uid/{firebaseUID}
// の Go 1.22 ServeMux ハンドラ。path value から UID を抜き出して既存の
// handleFindByFirebaseUID に委譲する。
func (s *Server) handleFindByFirebaseUIDRoute(w http.ResponseWriter, r *http.Request) {
	s.handleFindByFirebaseUID(w, r, r.PathValue("firebaseUID"))
}

// playersDispatcher は /internal/v1/players/ 配下の全 route を受けて
// method + path 構造で個別 handler に dispatch する。Go 1.22 ServeMux が
// `{playerID}/{resource}` 配下のサブツリーで衝突を拒否することがあるため、
// 自前でルーティングする必要がある。
func (s *Server) playersDispatcher(w http.ResponseWriter, r *http.Request) {
	const prefix = "/internal/v1/players/"
	rest := strings.TrimPrefix(r.URL.Path, prefix)
	parts := strings.Split(rest, "/")

	switch {
	case len(parts) == 1 && parts[0] == "award-game-exp" && r.Method == http.MethodPost:
		s.handleAwardGameExp(w, r)
		return
	case len(parts) == 1 && r.Method == http.MethodGet:
		s.handleGetPlayer(w, r, parts[0])
		return
	case len(parts) == 2:
		s.routePlayerSubresource(w, r, parts[0], parts[1])
		return
	case len(parts) == 3 && parts[1] == "battle-limit" && parts[2] == "increment" && r.Method == http.MethodPost:
		s.handleIncrementBattleCount(w, r, parts[0])
		return
	}
	http.NotFound(w, r)
}

func (s *Server) routePlayerSubresource(w http.ResponseWriter, r *http.Request, playerID, resource string) {
	switch resource {
	case "name":
		if r.Method == http.MethodPut {
			s.handleUpdateName(w, r, playerID)
			return
		}
	case "battle-limit":
		if r.Method == http.MethodGet {
			s.handleGetBattleLimit(w, r, playerID)
			return
		}
	case "premium":
		if r.Method == http.MethodPut {
			s.handleUpdatePremium(w, r, playerID)
			return
		}
	case "faction":
		if r.Method == http.MethodPut {
			s.handleUpdateFaction(w, r, playerID)
			return
		}
	case "factions":
		switch r.Method {
		case http.MethodPost:
			s.handleGrantFaction(w, r, playerID)
			return
		case http.MethodGet:
			s.handleListFactions(w, r, playerID)
			return
		}
	case "exp":
		if r.Method == http.MethodPost {
			s.handleAddExp(w, r, playerID)
			return
		}
	case "settings":
		switch r.Method {
		case http.MethodGet:
			s.handleGetSettings(w, r, playerID)
			return
		case http.MethodPut:
			s.handleUpdateSettings(w, r, playerID)
			return
		}
	}
	http.NotFound(w, r)
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req apiaccount.RegisterRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	fn := s.RegisterFn
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusCreated, apiaccount.Player{})
		return
	}
	status, body := fn(req)
	writeJSON(w, status, body)
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req apiaccount.LoginRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	fn := s.LoginFn
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusOK, apiaccount.Player{})
		return
	}
	status, body := fn(req)
	writeJSON(w, status, body)
}

func (s *Server) handleFindByFirebaseUID(w http.ResponseWriter, _ *http.Request, firebaseUID string) {
	s.mu.Lock()
	fn := s.FindByFirebaseUIDFn
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusOK, apiaccount.Player{})
		return
	}
	status, body := fn(firebaseUID)
	writeJSON(w, status, body)
}

func (s *Server) handleGetPlayer(w http.ResponseWriter, _ *http.Request, playerID string) {
	s.mu.Lock()
	fn := s.GetPlayerFn
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusOK, apiaccount.PlayerResponse{})
		return
	}
	status, body := fn(playerID)
	writeJSON(w, status, body)
}

func (s *Server) handleUpdateName(w http.ResponseWriter, r *http.Request, playerID string) {
	var req apiaccount.UpdateNameRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	fn := s.UpdateNameFn
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusOK, apiaccount.Player{})
		return
	}
	status, body := fn(playerID, req)
	writeJSON(w, status, body)
}

func (s *Server) handleGetBattleLimit(w http.ResponseWriter, _ *http.Request, playerID string) {
	s.mu.Lock()
	fn := s.GetBattleLimitFn
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusOK, apiaccount.BattleLimitResponse{})
		return
	}
	status, body := fn(playerID)
	writeJSON(w, status, body)
}

func (s *Server) handleIncrementBattleCount(w http.ResponseWriter, _ *http.Request, playerID string) {
	s.mu.Lock()
	fn := s.IncrementBattleCountFn
	s.mu.Unlock()
	if fn == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	status, body := fn(playerID)
	writeJSON(w, status, body)
}

func (s *Server) handleUpdatePremium(w http.ResponseWriter, r *http.Request, playerID string) {
	var req apiaccount.UpdatePremiumRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	fn := s.UpdatePremiumFn
	s.mu.Unlock()
	if fn == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	status, body := fn(playerID, req)
	writeJSON(w, status, body)
}

func (s *Server) handleUpdateFaction(w http.ResponseWriter, r *http.Request, playerID string) {
	var req apiaccount.UpdateFactionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	fn := s.UpdateFactionFn
	s.mu.Unlock()
	if fn == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	status, body := fn(playerID, req)
	writeJSON(w, status, body)
}

func (s *Server) handleGrantFaction(w http.ResponseWriter, r *http.Request, playerID string) {
	var req apiaccount.FactionGrantRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	fn := s.GrantFactionFn
	s.mu.Unlock()
	if fn == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	status, body := fn(playerID, req)
	writeJSON(w, status, body)
}

func (s *Server) handleListFactions(w http.ResponseWriter, _ *http.Request, playerID string) {
	s.mu.Lock()
	fn := s.ListFactionsFn
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusOK, apiaccount.ListFactionsResponse{Factions: []string{}})
		return
	}
	status, body := fn(playerID)
	writeJSON(w, status, body)
}

func (s *Server) handleAddExp(w http.ResponseWriter, r *http.Request, playerID string) {
	var req apiaccount.AddExpRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	fn := s.AddExpFn
	s.mu.Unlock()
	if fn == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	status, body := fn(playerID, req)
	writeJSON(w, status, body)
}

func (s *Server) handleAwardGameExp(w http.ResponseWriter, r *http.Request) {
	var req apiaccount.AwardGameExpRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	fn := s.AwardGameExpFn
	s.mu.Unlock()
	if fn == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	status, body := fn(req)
	writeJSON(w, status, body)
}

func (s *Server) handleGetSettings(w http.ResponseWriter, _ *http.Request, playerID string) {
	s.mu.Lock()
	fn := s.GetSettingsFn
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusOK, apiaccount.PlayerSettings{})
		return
	}
	status, body := fn(playerID)
	writeJSON(w, status, body)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request, playerID string) {
	var req apiaccount.UpdateSettingsRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	fn := s.UpdateSettingsFn
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusOK, apiaccount.PlayerSettings{})
		return
	}
	status, body := fn(playerID, req)
	writeJSON(w, status, body)
}

// writeJSON は status code を書き、body が非 nil なら Content-Type: application/json
// で JSON encode して送る。body が nil の場合は body 無しでレスポンスを終わる
// (accountclient は 4xx/5xx のエラー body を読まないため、body=nil でも応答は成立する)。
func writeJSON(w http.ResponseWriter, status int, body any) {
	if body == nil {
		w.WriteHeader(status)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
