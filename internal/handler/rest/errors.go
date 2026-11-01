package rest

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

const (
	internalServerErrorMessage = "internal server error"
	playerNotFoundMessage      = "player not found"
)

// describeError はエラーに対応する HTTP ステータスと応答の文面を返す。
// 対応づけのないエラーは 500 と固定の文面にし、原因を応答に出さない。
func describeError(err error) (status int, message string) {
	switch {
	case errors.Is(err, port.ErrNotFound):
		return http.StatusNotFound, playerNotFoundMessage
	case errors.Is(err, usecase.ErrPlayerAlreadyRegistered), errors.Is(err, usecase.ErrFactionAlreadySelected):
		return http.StatusConflict, err.Error()
	case errors.Is(err, usecase.ErrBattleLimitExceeded):
		return http.StatusTooManyRequests, err.Error()
	case errors.Is(err, usecase.ErrInvalidFaction), errors.Is(err, domain.ErrInvalidName):
		return http.StatusBadRequest, err.Error()
	case errors.Is(err, domain.ErrLanguageEmpty):
		return http.StatusBadRequest, requiredFieldMessage("language")
	case errors.Is(err, domain.ErrLanguageTooLong):
		return http.StatusBadRequest, invalidFieldMessage("language")
	case errors.Is(err, usecase.ErrInvalidExpGain):
		return http.StatusBadRequest, invalidFieldMessage("exp_gain")
	case errors.Is(err, usecase.ErrInvalidWinnerNumber):
		return http.StatusBadRequest, invalidFieldMessage("winner_num")
	case errors.Is(err, usecase.ErrInvalidMatchType):
		return http.StatusBadRequest, invalidFieldMessage("match_type")
	case errors.Is(err, usecase.ErrPlayer2IDRequired):
		return http.StatusBadRequest, requiredFieldMessage("player2_id")
	default:
		return http.StatusInternalServerError, internalServerErrorMessage
	}
}

// respondError はエラーに対応するエラー応答を返す。5xx のときは原因をログに残す。
func respondError(c *gin.Context, err error) {
	status, message := describeError(err)
	if status >= http.StatusInternalServerError {
		slog.ErrorContext(c.Request.Context(), "handler internal error",
			"method", c.Request.Method,
			"path", c.FullPath(),
			"status", status,
			"error", err,
		)
	}
	c.JSON(status, apiaccount.ErrorResponse{Error: message})
}
