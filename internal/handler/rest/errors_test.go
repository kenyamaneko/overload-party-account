//go:build integration

package rest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/repository/postgres"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"

	internalauth "github.com/kenyamaneko/overload-party-gateway/packages/internalauth-go"
)

const (
	contractPlayerID = "11111111-1111-1111-1111-111111111111"
	missingPlayerID  = "99999999-9999-9999-9999-999999999999"
)

// newContractEngine は実 PostgreSQL repository を結線した account ハンドラを
// 認証済み player_id を注入する gin エンジンとして組む。
func newContractEngine(playerID string) *gin.Engine {
	playerRepo := postgres.NewPlayerRepository(sharedPg.Pool)
	viewRepo := postgres.NewPlayerViewRepository(sharedPg.Pool)
	settingsRepo := postgres.NewPlayerSettingsRepository(sharedPg.Pool)
	tx := postgres.NewTxManager(sharedPg.Pool)
	gameConfig := &fakeGameConfigRepo{values: map[string]int64{usecase.ConfigKeyExpFormulaCoefficient: 60}}

	playerInteractor := usecase.NewPlayerInteractor(playerRepo, playerRepo, playerRepo, playerRepo, viewRepo, gameConfig, tx)
	settingsInteractor := usecase.NewPlayerSettingsInteractor(settingsRepo)
	playerH := NewPlayerHandler(playerInteractor)
	settingsH := NewPlayerSettingsHandler(settingsInteractor)

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(internalauth.PlayerIDContextKey, playerID) })
	r.GET("/me", playerH.GetPlayer)
	r.PUT("/me/settings", settingsH.UpdateSettings)
	return r
}

// TestHandlerErrorStatusContract は実 HTTP 経路でエラー種別が期待ステータスへ写像される契約を検証する。
func TestHandlerErrorStatusContract(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name             string
		seed             func(t *testing.T)
		playerID         string
		method           string
		path             string
		body             string
		wantStatus       int
		wantBodyContains string
	}{
		{
			name:       "存在するプレイヤーの取得は 200",
			seed:       func(t *testing.T) { seedPlayer(t, contractPlayerID, "uid-1") },
			playerID:   contractPlayerID,
			method:     http.MethodGet,
			path:       "/me",
			wantStatus: http.StatusOK,
		},
		{
			name:             "存在しないプレイヤーの取得は実 not-found を 404 に写像する",
			seed:             func(t *testing.T) {},
			playerID:         missingPlayerID,
			method:           http.MethodGet,
			path:             "/me",
			wantStatus:       http.StatusNotFound,
			wantBodyContains: port.ErrNotFound.Error(),
		},
		{
			name:       "全 nil の設定更新は IsEmpty ガードで 400",
			seed:       func(t *testing.T) {},
			playerID:   contractPlayerID,
			method:     http.MethodPut,
			path:       "/me/settings",
			body:       "{}",
			wantStatus: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sharedPg.Truncate(t)
			tt.seed(t)

			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			newContractEngine(tt.playerID).ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Contains(t, w.Body.String(), tt.wantBodyContains)
		})
	}
}
