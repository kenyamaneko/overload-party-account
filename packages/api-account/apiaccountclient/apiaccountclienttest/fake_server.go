package apiaccountclienttest

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

const (
	invalidRequestBodyMessage = "invalid request body"
	healthStatusOK            = "ok"
)

// FakeServer は、アカウントサービスの REST 契約を実装する起動済みの偽のアカウントサービス。
// Fn フィールドは操作の応答のステータスと本文を返し、本文が nil のときは本文なしで応答する。
type FakeServer struct {
	srv *httptest.Server

	// RegisterFn は registerPlayer の応答を決める。既定では 201 と空のプレイヤー情報を返す。
	RegisterFn func(req apiaccount.RegisterRequest) (int, any)

	// LoginFn は loginPlayer の応答を決める。既定では 200 と空のプレイヤー情報を返す。
	LoginFn func(req apiaccount.LoginRequest) (int, any)

	// GetPlayerByFirebaseUIDFn は getPlayerByFirebaseUID の応答を決める。既定では 200 と空のプレイヤー情報を返す。
	GetPlayerByFirebaseUIDFn func(firebaseUID string) (int, any)

	// AwardGameExpFn は awardGameExp の応答を決める。既定では 204 を返す。
	AwardGameExpFn func(req apiaccount.AwardGameExpRequest) (int, any)

	// RevertBattleCountFn は revertBattleCount の応答を決める。既定では 204 を返す。
	RevertBattleCountFn func(req apiaccount.RevertBattleCountRequest) (int, any)

	// GetPlayerFn は getPlayer の応答を決める。既定では 200 と空のプレイヤー情報を返す。
	GetPlayerFn func() (int, any)

	// ValidateNameForOnboardingFn は validateNameForOnboarding の応答を決める。既定では 204 を返す。
	ValidateNameForOnboardingFn func(req apiaccount.ValidateNameForOnboardingRequest) (int, any)

	// GetBattleLimitFn は getBattleLimit の応答を決める。既定では 200 と空のバトル回数制限を返す。
	GetBattleLimitFn func() (int, any)

	// IncrementBattleCountFn は incrementBattleCount の応答を決める。既定では 204 を返す。
	IncrementBattleCountFn func() (int, any)

	// ListFactionsFn は listFactions の応答を決める。既定では 200 と空の所持陣営一覧を返す。
	ListFactionsFn func() (int, any)
}

// NewFakeServer は起動済みの FakeServer を返す。テストの終了時に Close を呼ぶ。
func NewFakeServer() *FakeServer {
	s := &FakeServer{
		RegisterFn: func(apiaccount.RegisterRequest) (int, any) {
			return http.StatusCreated, apiaccount.PlayerResponse{}
		},
		LoginFn: func(apiaccount.LoginRequest) (int, any) {
			return http.StatusOK, apiaccount.PlayerResponse{}
		},
		GetPlayerByFirebaseUIDFn: func(string) (int, any) {
			return http.StatusOK, apiaccount.PlayerResponse{}
		},
		AwardGameExpFn: func(apiaccount.AwardGameExpRequest) (int, any) {
			return http.StatusNoContent, nil
		},
		RevertBattleCountFn: func(apiaccount.RevertBattleCountRequest) (int, any) {
			return http.StatusNoContent, nil
		},
		GetPlayerFn: func() (int, any) {
			return http.StatusOK, apiaccount.PlayerResponse{}
		},
		ValidateNameForOnboardingFn: func(apiaccount.ValidateNameForOnboardingRequest) (int, any) {
			return http.StatusNoContent, nil
		},
		GetBattleLimitFn: func() (int, any) {
			return http.StatusOK, apiaccount.BattleLimitResponse{}
		},
		IncrementBattleCountFn: func() (int, any) {
			return http.StatusNoContent, nil
		},
		ListFactionsFn: func() (int, any) {
			return http.StatusOK, emptyFactionListing()
		},
	}
	s.srv = httptest.NewServer(apiaccount.Handler(contractHandler{server: s}))
	return s
}

// URL は FakeServer の基底 URL を返す。
func (s *FakeServer) URL() string { return s.srv.URL }

// Close は FakeServer を停止する。
func (s *FakeServer) Close() { s.srv.Close() }

type response struct {
	status int
	body   any
}

type contractHandler struct {
	server *FakeServer
}

var _ apiaccount.ServerInterface = contractHandler{}

func (h contractHandler) GetHealth(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, response{http.StatusOK, apiaccount.HealthResponse{Status: healthStatusOK}})
}

func (h contractHandler) RegisterPlayer(w http.ResponseWriter, r *http.Request) {
	replyWithBody(w, r, h.server.RegisterFn)
}

func (h contractHandler) LoginPlayer(w http.ResponseWriter, r *http.Request) {
	replyWithBody(w, r, h.server.LoginFn)
}

func (h contractHandler) GetPlayerByFirebaseUID(w http.ResponseWriter, _ *http.Request, firebaseUID string) {
	reply(w, func() (int, any) { return h.server.GetPlayerByFirebaseUIDFn(firebaseUID) })
}

func (h contractHandler) ListPlayerFactions(w http.ResponseWriter, _ *http.Request, _ string) {
	writeResponse(w, response{http.StatusOK, emptyFactionListing()})
}

func (h contractHandler) AwardGameExp(w http.ResponseWriter, r *http.Request) {
	replyWithBody(w, r, h.server.AwardGameExpFn)
}

func (h contractHandler) RevertBattleCount(w http.ResponseWriter, r *http.Request) {
	replyWithBody(w, r, h.server.RevertBattleCountFn)
}

func (h contractHandler) GetPlayer(w http.ResponseWriter, _ *http.Request) {
	reply(w, h.server.GetPlayerFn)
}

func (h contractHandler) UpdateName(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, response{http.StatusOK, apiaccount.PlayerResponse{}})
}

func (h contractHandler) ValidateNameForOnboarding(w http.ResponseWriter, r *http.Request) {
	replyWithBody(w, r, h.server.ValidateNameForOnboardingFn)
}

func (h contractHandler) GetBattleLimit(w http.ResponseWriter, _ *http.Request) {
	reply(w, h.server.GetBattleLimitFn)
}

func (h contractHandler) IncrementBattleCount(w http.ResponseWriter, _ *http.Request) {
	reply(w, h.server.IncrementBattleCountFn)
}

func (h contractHandler) UpdatePremium(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, response{http.StatusNoContent, nil})
}

func (h contractHandler) AddExp(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, response{http.StatusNoContent, nil})
}

func (h contractHandler) ListFactions(w http.ResponseWriter, _ *http.Request) {
	reply(w, h.server.ListFactionsFn)
}

func (h contractHandler) GrantFaction(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, response{http.StatusNoContent, nil})
}

func (h contractHandler) SelectInitialFaction(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, response{http.StatusOK, nil})
}

func (h contractHandler) GetPlayerSettings(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, response{http.StatusOK, apiaccount.PlayerSettingsResponse{}})
}

func (h contractHandler) UpdatePlayerSettings(w http.ResponseWriter, _ *http.Request) {
	writeResponse(w, response{http.StatusOK, apiaccount.PlayerSettingsResponse{}})
}

func emptyFactionListing() apiaccount.FactionListing {
	return apiaccount.FactionListing{Factions: []string{}}
}

func replyWithBody[Req any](w http.ResponseWriter, r *http.Request, fn func(Req) (int, any)) {
	var req Req
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeResponse(w, response{http.StatusBadRequest, apiaccount.ErrorResponse{Error: invalidRequestBodyMessage}})
		return
	}
	reply(w, func() (int, any) { return fn(req) })
}

func reply(w http.ResponseWriter, call func() (int, any)) {
	status, body := call()
	writeResponse(w, response{status, body})
}

func writeResponse(w http.ResponseWriter, res response) {
	if res.body == nil {
		w.WriteHeader(res.status)
		return
	}
	payload, err := json.Marshal(res.body)
	if err != nil {
		writeResponse(w, response{http.StatusInternalServerError, apiaccount.ErrorResponse{Error: err.Error()}})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(res.status)
	if _, err := w.Write(payload); err != nil {
		slog.Error("apiaccountclienttest: write response body", "error", err)
	}
}
