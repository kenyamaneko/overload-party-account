package rest

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

// errorStatus はドメインエラーを HTTP ステータスコードに変換する。
// 該当なしは 500 にフォールバックしクライアントのリトライを促す。
func errorStatus(err error) int {
	switch {
	case isNotFound(err):
		return http.StatusNotFound
	case isConflict(err):
		return http.StatusConflict
	case isTooManyRequests(err):
		return http.StatusTooManyRequests
	case isValidation(err):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// respondError は Gin context に統一フォーマットの JSON エラーを書き込む。
// 5xx はクライアントに実体を伝搬できないため ops 可視性のために構造化ログに記録する。
func respondError(c *gin.Context, err error) {
	status := errorStatus(err)
	if status >= http.StatusInternalServerError {
		slog.ErrorContext(c.Request.Context(), "handler internal error",
			"method", c.Request.Method,
			"path", c.FullPath(),
			"status", status,
			"error", err,
		)
	}
	c.JSON(status, gin.H{"error": err.Error()})
}

func isNotFound(err error) bool {
	return errors.Is(err, port.ErrNotFound) ||
		errors.Is(err, usecase.ErrPlayerNotFound)
}

func isConflict(err error) bool {
	return errors.Is(err, usecase.ErrPlayerAlreadyRegistered) ||
		errors.Is(err, usecase.ErrFactionAlreadySelected)
}

func isTooManyRequests(err error) bool {
	return errors.Is(err, usecase.ErrBattleLimitExceeded)
}

func isValidation(err error) bool {
	return errors.Is(err, usecase.ErrInvalidFaction) ||
		errors.Is(err, domain.ErrInvalidName)
}
