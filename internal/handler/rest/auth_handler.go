package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// AuthHandler は認証関連の REST エンドポイントを処理する。
type AuthHandler struct {
	authService *usecase.AuthInteractor
}

// NewAuthHandler は AuthHandler を生成する。
func NewAuthHandler(authService *usecase.AuthInteractor) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Register は新規プレイヤー登録を処理する。
func (h *AuthHandler) Register(c *gin.Context) {
	var req apiaccount.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.FirebaseUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "firebase_uid is required"})
		return
	}

	player, err := h.authService.Register(c.Request.Context(), req.FirebaseUID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, player)
}

// GetPlayerByFirebaseUID は Firebase UID でプレイヤーを検索する。
func (h *AuthHandler) GetPlayerByFirebaseUID(c *gin.Context) {
	firebaseUID := c.Param("firebaseUID")
	if firebaseUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "firebaseUID is required"})
		return
	}
	player, err := h.authService.FindByFirebaseUID(c.Request.Context(), firebaseUID)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(http.StatusOK, player)
}

// Login は既存プレイヤーのログインを処理する。
func (h *AuthHandler) Login(c *gin.Context) {
	var req apiaccount.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.FirebaseUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "firebase_uid is required"})
		return
	}

	player, err := h.authService.Login(c.Request.Context(), req.FirebaseUID)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusOK, player)
}
