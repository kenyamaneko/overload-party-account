package rest

import (
	"net/http"
	"unicode/utf8"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/service"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

const maxUsernameRunes = 50

// AuthHandler は認証関連の REST エンドポイントを処理します。
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler は AuthHandler を生成します。
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Register は新規プレイヤー登録を処理します。
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
	if n := utf8.RuneCountInString(req.Username); n < 1 || n > maxUsernameRunes {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username must be 1-50 characters"})
		return
	}

	player, err := h.authService.Register(c.Request.Context(), req.FirebaseUID, req.Username)
	if err != nil {
		respondError(c, err)
		return
	}

	c.JSON(http.StatusCreated, player)
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
