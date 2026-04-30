package rest

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/service"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

// AuthHandler は認証関連の REST エンドポイントを処理します。
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler は AuthHandler を生成します。
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Register は新規プレイヤー登録を処理します。表示名は受け取らず、
// オンボーディング完了時の player-onboarded イベントで確定します。
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

// GetPlayerByFirebaseUID は Firebase UID でプレイヤーを検索します。
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

// Login は既存プレイヤーのログインを処理します。
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
