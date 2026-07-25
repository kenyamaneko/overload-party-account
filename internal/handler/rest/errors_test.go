//go:build integration

package rest

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	factionRepo := postgres.NewFactionRepository(sharedPg.Pool)
	tx := postgres.NewTxManager(sharedPg.Pool)
	gameConfig := &fakeGameConfigRepo{values: map[string]int64{usecase.ConfigKeyExpFormulaCoefficient: 60}}

	playerInteractor := usecase.NewPlayerInteractor(playerRepo, playerRepo, playerRepo, playerRepo, viewRepo, gameConfig, tx)
	settingsInteractor := usecase.NewPlayerSettingsInteractor(settingsRepo)
	factionInteractor := usecase.NewFactionInteractor(playerRepo, factionRepo, tx)
	playerH := NewPlayerHandler(playerInteractor)
	settingsH := NewPlayerSettingsHandler(settingsInteractor)
	factionH := NewFactionHandler(factionInteractor)

	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set(internalauth.PlayerIDContextKey, playerID) })
	r.GET("/me", playerH.GetPlayer)
	r.PUT("/me/settings", settingsH.UpdateSettings)
	r.POST("/me/factions/select", factionH.SelectInitialFaction)
	return r
}

func TestHandlerErrorStatusContract(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("エラー種別の HTTP ステータス写像", func(t *testing.T) {
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
				name:       "存在するプレイヤーを取得するとき、200 になる",
				seed:       func(t *testing.T) { seedPlayer(t, contractPlayerID, "uid-1") },
				playerID:   contractPlayerID,
				method:     http.MethodGet,
				path:       "/me",
				wantStatus: http.StatusOK,
			},
			{
				name:             "存在しないプレイヤーを取得するとき、not-found を 404 に写像する",
				seed:             func(t *testing.T) {},
				playerID:         missingPlayerID,
				method:           http.MethodGet,
				path:             "/me",
				wantStatus:       http.StatusNotFound,
				wantBodyContains: port.ErrNotFound.Error(),
			},
			{
				name:             "更新対象のフィールドを一つも指定しない設定更新のとき、400 になり、応答本文に \"at least one settings field is required\" が含まれる",
				seed:             func(t *testing.T) {},
				playerID:         contractPlayerID,
				method:           http.MethodPut,
				path:             "/me/settings",
				body:             "{}",
				wantStatus:       http.StatusBadRequest,
				wantBodyContains: "at least one settings field is required",
			},
			{
				name: "初期陣営選択済みで再選択するとき、conflict を 409 に写像する",
				seed: func(t *testing.T) {
					seedPlayer(t, contractPlayerID, "uid-1")
					first := httptest.NewRequest(http.MethodPost, "/me/factions/select", strings.NewReader(`{"faction_id":"SHE"}`))
					first.Header.Set("Content-Type", "application/json")
					fw := httptest.NewRecorder()
					newContractEngine(contractPlayerID).ServeHTTP(fw, first)
					require.Equal(t, http.StatusOK, fw.Code)
				},
				playerID:         contractPlayerID,
				method:           http.MethodPost,
				path:             "/me/factions/select",
				body:             `{"faction_id":"SHE"}`,
				wantStatus:       http.StatusConflict,
				wantBodyContains: usecase.ErrFactionAlreadySelected.Error(),
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
	})
}
