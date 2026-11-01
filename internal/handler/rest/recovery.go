package rest

import (
	"fmt"
	"io"

	"github.com/gin-gonic/gin"
)

// NewRecovery はリクエストの処理中に異常終了したとき、原因を構造化ログに残して 500 の応答を返すミドルウェアを返す。
func NewRecovery() gin.HandlerFunc {
	// gin は異常終了の内容とスタックを構造化されていない形で出力するため、出力先を捨て、ログは respondError に任せる。
	return gin.CustomRecoveryWithWriter(io.Discard, func(c *gin.Context, recovered any) {
		respondError(c, fmt.Errorf("panic: %v", recovered))
		c.Abort()
	})
}
