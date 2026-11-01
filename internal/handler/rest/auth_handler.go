package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// AuthHandler はプレイヤーの登録とログインの REST の受け口である。
type AuthHandler struct {
	authInteractor *usecase.AuthInteractor
}

// NewAuthHandler は AuthHandler を生成する。
func NewAuthHandler(authInteractor *usecase.AuthInteractor) *AuthHandler {
	return &AuthHandler{authInteractor: authInteractor}
}

// Register は新規プレイヤーを登録する。
func (h *AuthHandler) Register(c *gin.Context) {
	var req apiaccount.RegisterRequest
	if !bindRequestBody(c, &req) || respondIfEmpty(c, "firebase_uid", req.FirebaseUID) {
		return
	}

	info, err := h.authInteractor.Register(c.Request.Context(), req.FirebaseUID)
	if err != nil {
		respondError(c, err)
		return
	}
	respondPlayer(c, http.StatusCreated, info)
}

// GetPlayerByFirebaseUID は Firebase UID でプレイヤーを検索する。
func (h *AuthHandler) GetPlayerByFirebaseUID(c *gin.Context) {
	info, err := h.authInteractor.GetByFirebaseUID(c.Request.Context(), c.Param("firebaseUID"))
	if err != nil {
		respondError(c, err)
		return
	}
	respondPlayer(c, http.StatusOK, info)
}

// Login は登録済みのプレイヤーのログインを受け付ける。
func (h *AuthHandler) Login(c *gin.Context) {
	var req apiaccount.LoginRequest
	if !bindRequestBody(c, &req) || respondIfEmpty(c, "firebase_uid", req.FirebaseUID) {
		return
	}

	info, err := h.authInteractor.Login(c.Request.Context(), req.FirebaseUID)
	if err != nil {
		respondError(c, err)
		return
	}
	respondPlayer(c, http.StatusOK, info)
}
