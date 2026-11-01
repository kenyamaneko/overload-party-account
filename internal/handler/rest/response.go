package rest

import (
	"github.com/gin-gonic/gin"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
	"github.com/kenyamaneko/overload-party-account/internal/presenter"
)

// respondPlayer はプレイヤー情報を status の応答として返す。応答の形に変換できないときは 500 の応答を返す。
func respondPlayer(c *gin.Context, status int, info *domain.PlayerInfo) {
	response, err := presenter.ToPlayerResponse(info)
	if err != nil {
		respondError(c, err)
		return
	}
	c.JSON(status, response)
}
