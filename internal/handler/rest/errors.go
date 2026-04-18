package rest

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/service"
)

// errorStatus はドメインエラーを HTTP ステータスコードに変換する。
// service 層が返す sentinel をここで「not found / conflict / validation」のいずれに
// 翻訳するかを決定する責務は transport (handler) 側に閉じる。
//
// 分類対象に該当しないエラー (DB 一時障害等) は default の 500 にフォールバックし、
// クライアントのリトライを促す。
func errorStatus(err error) int {
	switch {
	case isNotFound(err):
		return http.StatusNotFound
	case isConflict(err):
		return http.StatusConflict
	case isValidation(err):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// respondError は Gin context に統一フォーマットの JSON エラーを書き込む。
func respondError(c *gin.Context, err error) {
	c.JSON(errorStatus(err), gin.H{"error": err.Error()})
}

// isNotFound は対象リソースが見つからない類のエラーか判定する。
// port.ErrNotFound は repo から bubble され、service.ErrPlayerNotFound は service 層の
// ドメイン別名である。
func isNotFound(err error) bool {
	return errors.Is(err, port.ErrNotFound) ||
		errors.Is(err, service.ErrPlayerNotFound)
}

// isConflict は既存リソースとの衝突 (重複登録・冪等な再選択) によるエラーか判定する。
func isConflict(err error) bool {
	return errors.Is(err, service.ErrPlayerAlreadyRegistered) ||
		errors.Is(err, service.ErrFactionAlreadySelected)
}

// isValidation はクライアント入力の妥当性違反によるエラーか判定する。
func isValidation(err error) bool {
	return errors.Is(err, service.ErrInvalidFaction)
}
