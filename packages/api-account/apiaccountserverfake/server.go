// Package apiaccountserverfake は account サービスの HTTP 契約を実装する
// httptest.Server ラッパー。consumer (gateway 等) が accountclient を使う
// handler テストで、実 account サービスを起動せずに REST 呼び出しを検証する
// ためのテストダブルを提供する。
//
// 各 endpoint は Fn field (func callback) で status + response body を制御する。
// Fn が nil の endpoint は既定値を返す (happy-path を仮定した最低限の応答)。
//
// Request / Response 型は api-account の公開型 (apiaccount.RegisterRequest /
// PlayerResponse 等) を再利用するため、本パッケージは自前の型を宣言していない。
//
// player-scoped endpoint (/api/v1/account/me/*) は ADR-037 Phase 3 で path / body
// から playerID を取り除き JWT sub クレーム一本に置換したため、Fn callback の
// 引数からも playerID は除外する。fake は JWT 検証をスキップするので、consumer 側
// テストは X-Internal-Auth header を付ける必要すらない。
package apiaccountserverfake

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// Server は account HTTP 契約を実装する httptest.Server wrapper。
type Server struct {
	mu  sync.Mutex
	srv *httptest.Server

	// RegisterFn: POST /internal/v1/auth/register。既定は 201 + 空 PlayerResponse。
	RegisterFn func(req apiaccount.RegisterRequest) (int, any)

	// LoginFn: POST /internal/v1/auth/login。既定は 200 + 空 PlayerResponse。
	LoginFn func(req apiaccount.LoginRequest) (int, any)

	// FindByFirebaseUIDFn: GET /internal/v1/auth/by-firebase-uid/{uid}。
	// 既定は 200 + 空 PlayerResponse。未登録を擬似したい場合は Fn で 404 を返す。
	FindByFirebaseUIDFn func(firebaseUID string) (int, any)

	// AwardGameExpFn: POST /internal/v1/players/award-game-exp。既定は 204 No Content。
	AwardGameExpFn func(req apiaccount.AwardGameExpRequest) (int, any)

	// GetPlayerByIDFn: GET /internal/v1/players/{playerID}。既定は 200 + 空 PlayerResponse。
	// 未登録を擬似したい場合は Fn で 404 を返す。
	GetPlayerByIDFn func(playerID string) (int, any)

	// GetPlayerFn: GET /api/v1/account/me。既定は 200 + 空 PlayerResponse。
	GetPlayerFn func() (int, any)

	// UpdateNameFn: PUT /api/v1/account/me/name。既定は 200 + 空 PlayerResponse。
	UpdateNameFn func(req apiaccount.UpdateNameRequest) (int, any)

	// ValidateNameForOnboardingFn: POST /api/v1/account/me/onboarding/name/validate。既定は 204 No Content。
	ValidateNameForOnboardingFn func(req apiaccount.ValidateNameForOnboardingRequest) (int, any)

	// GetBattleLimitFn: GET /api/v1/account/me/battle-limit。
	// 既定は 200 + 空 BattleLimitResponse。
	GetBattleLimitFn func() (int, any)

	// IncrementBattleCountFn: POST /api/v1/account/me/battle-limit/increment。
	// 既定は 204 No Content。
	IncrementBattleCountFn func() (int, any)

	// UpdatePremiumFn: PUT /api/v1/account/me/premium。既定は 204 No Content。
	UpdatePremiumFn func(req apiaccount.UpdatePremiumRequest) (int, any)

	// GrantFactionFn: POST /api/v1/account/me/factions。既定は 204 No Content。
	GrantFactionFn func(req apiaccount.FactionGrantRequest) (int, any)

	// SelectInitialFactionFn: POST /api/v1/account/me/factions/select。既定は 200。
	SelectInitialFactionFn func(req apiaccount.SelectInitialFactionRequest) (int, any)

	// ListFactionsFn: GET /api/v1/account/me/factions。
	// 既定は 200 + 空の ListFactionsResponse。
	ListFactionsFn func() (int, any)

	// AddExpFn: POST /api/v1/account/me/exp。既定は 204 No Content。
	AddExpFn func(req apiaccount.AddExpRequest) (int, any)

	// GetSettingsFn: GET /api/v1/account/me/settings。既定は 200 + 空 PlayerSettings。
	GetSettingsFn func() (int, any)

	// UpdateSettingsFn: PUT /api/v1/account/me/settings。既定は 200 + 空 PlayerSettings。
	UpdateSettingsFn func(req apiaccount.UpdateSettingsRequest) (int, any)
}

// NewServer は起動済み Server を返す。テスト終了時に Close() すること。
func NewServer() *Server {
	s := &Server{}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /internal/v1/auth/register", s.handleRegister)
	mux.HandleFunc("POST /internal/v1/auth/login", s.handleLogin)
	mux.HandleFunc("GET /internal/v1/auth/by-firebase-uid/{firebaseUID}", s.handleFindByFirebaseUID)
	mux.HandleFunc("POST /internal/v1/players/award-game-exp", s.handleAwardGameExp)
	mux.HandleFunc("GET /internal/v1/players/{playerID}", s.handleGetPlayerByID)

	mux.HandleFunc("GET /api/v1/account/me", s.handleGetPlayer)
	mux.HandleFunc("PUT /api/v1/account/me/name", s.handleUpdateName)
	mux.HandleFunc("POST /api/v1/account/me/onboarding/name/validate", s.handleValidateNameForOnboarding)
	mux.HandleFunc("GET /api/v1/account/me/battle-limit", s.handleGetBattleLimit)
	mux.HandleFunc("POST /api/v1/account/me/battle-limit/increment", s.handleIncrementBattleCount)
	mux.HandleFunc("PUT /api/v1/account/me/premium", s.handleUpdatePremium)
	mux.HandleFunc("POST /api/v1/account/me/exp", s.handleAddExp)
	mux.HandleFunc("POST /api/v1/account/me/factions", s.handleGrantFaction)
	mux.HandleFunc("POST /api/v1/account/me/factions/select", s.handleSelectInitialFaction)
	mux.HandleFunc("GET /api/v1/account/me/factions", s.handleListFactions)
	mux.HandleFunc("GET /api/v1/account/me/settings", s.handleGetSettings)
	mux.HandleFunc("PUT /api/v1/account/me/settings", s.handleUpdateSettings)
	s.srv = httptest.NewServer(mux)
	return s
}

// URL は httptest.Server のベース URL を返す。
func (s *Server) URL() string { return s.srv.URL }

// Close は内部 httptest.Server を閉じる。
func (s *Server) Close() { s.srv.Close() }

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req apiaccount.RegisterRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	fn := s.RegisterFn
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusCreated, apiaccount.PlayerResponse{})
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
		writeJSON(w, http.StatusOK, apiaccount.PlayerResponse{})
		return
	}
	status, body := fn(req)
	writeJSON(w, status, body)
}

func (s *Server) handleFindByFirebaseUID(w http.ResponseWriter, r *http.Request) {
	firebaseUID := r.PathValue("firebaseUID")
	s.mu.Lock()
	fn := s.FindByFirebaseUIDFn
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusOK, apiaccount.PlayerResponse{})
		return
	}
	status, body := fn(firebaseUID)
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

func (s *Server) handleGetPlayerByID(w http.ResponseWriter, r *http.Request) {
	playerID := r.PathValue("playerID")
	s.mu.Lock()
	fn := s.GetPlayerByIDFn
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusOK, apiaccount.PlayerResponse{})
		return
	}
	status, body := fn(playerID)
	writeJSON(w, status, body)
}

func (s *Server) handleGetPlayer(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	fn := s.GetPlayerFn
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusOK, apiaccount.PlayerResponse{})
		return
	}
	status, body := fn()
	writeJSON(w, status, body)
}

func (s *Server) handleUpdateName(w http.ResponseWriter, r *http.Request) {
	var req apiaccount.UpdateNameRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	fn := s.UpdateNameFn
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusOK, apiaccount.PlayerResponse{})
		return
	}
	status, body := fn(req)
	writeJSON(w, status, body)
}

func (s *Server) handleValidateNameForOnboarding(w http.ResponseWriter, r *http.Request) {
	var req apiaccount.ValidateNameForOnboardingRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	fn := s.ValidateNameForOnboardingFn
	s.mu.Unlock()
	if fn == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	status, body := fn(req)
	writeJSON(w, status, body)
}

func (s *Server) handleGetBattleLimit(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	fn := s.GetBattleLimitFn
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusOK, apiaccount.BattleLimitResponse{})
		return
	}
	status, body := fn()
	writeJSON(w, status, body)
}

func (s *Server) handleIncrementBattleCount(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	fn := s.IncrementBattleCountFn
	s.mu.Unlock()
	if fn == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	status, body := fn()
	writeJSON(w, status, body)
}

func (s *Server) handleUpdatePremium(w http.ResponseWriter, r *http.Request) {
	var req apiaccount.UpdatePremiumRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	fn := s.UpdatePremiumFn
	s.mu.Unlock()
	if fn == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	status, body := fn(req)
	writeJSON(w, status, body)
}

func (s *Server) handleGrantFaction(w http.ResponseWriter, r *http.Request) {
	var req apiaccount.FactionGrantRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	fn := s.GrantFactionFn
	s.mu.Unlock()
	if fn == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	status, body := fn(req)
	writeJSON(w, status, body)
}

func (s *Server) handleSelectInitialFaction(w http.ResponseWriter, r *http.Request) {
	var req apiaccount.SelectInitialFactionRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	fn := s.SelectInitialFactionFn
	s.mu.Unlock()
	if fn == nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	status, body := fn(req)
	writeJSON(w, status, body)
}

func (s *Server) handleListFactions(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	fn := s.ListFactionsFn
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusOK, apiaccount.ListFactionsResponse{Factions: []string{}})
		return
	}
	status, body := fn()
	writeJSON(w, status, body)
}

func (s *Server) handleAddExp(w http.ResponseWriter, r *http.Request) {
	var req apiaccount.AddExpRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	fn := s.AddExpFn
	s.mu.Unlock()
	if fn == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	status, body := fn(req)
	writeJSON(w, status, body)
}

func (s *Server) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	fn := s.GetSettingsFn
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusOK, apiaccount.PlayerSettingsResponse{})
		return
	}
	status, body := fn()
	writeJSON(w, status, body)
}

func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req apiaccount.UpdateSettingsRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	s.mu.Lock()
	fn := s.UpdateSettingsFn
	s.mu.Unlock()
	if fn == nil {
		writeJSON(w, http.StatusOK, apiaccount.PlayerSettingsResponse{})
		return
	}
	status, body := fn(req)
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
