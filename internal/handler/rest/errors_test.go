package rest

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

// TestRespondError_SentinelToStatusMapping は respondError のエラー分類契約を
// サーバー側で固定する。
func TestRespondError_SentinelToStatusMapping(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{
			name:       "ErrNotFound は 404",
			err:        port.ErrNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "ErrPlayerNotFound は 404",
			err:        usecase.ErrPlayerNotFound,
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "wrap された ErrNotFound も errors.Is 経由で 404",
			err:        fmt.Errorf("lookup player: %w", port.ErrNotFound),
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "ErrPlayerAlreadyRegistered は 409",
			err:        usecase.ErrPlayerAlreadyRegistered,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "ErrFactionAlreadySelected は 409",
			err:        usecase.ErrFactionAlreadySelected,
			wantStatus: http.StatusConflict,
		},
		{
			name:       "ErrBattleLimitExceeded は 429",
			err:        usecase.ErrBattleLimitExceeded,
			wantStatus: http.StatusTooManyRequests,
		},
		{
			name:       "ErrInvalidFaction は 400",
			err:        usecase.ErrInvalidFaction,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "ErrInvalidName は 400",
			err:        domain.ErrInvalidName,
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "subscriber 内部の ErrFactionConflict は handler 契約外で 500",
			err:        usecase.ErrFactionConflict,
			wantStatus: http.StatusInternalServerError,
		},
		{
			name:       "未分類の汎用エラーは 500 にフォールバック",
			err:        errors.New("unexpected failure"),
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

			respondError(c, tt.err)

			wantBody, marshalErr := json.Marshal(map[string]string{"error": tt.err.Error()})
			assert.NoError(t, marshalErr)
			assert.Equal(t, tt.wantStatus, w.Code)
			assert.JSONEq(t, string(wantBody), w.Body.String())
		})
	}
}
