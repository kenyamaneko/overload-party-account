package rest

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	apiaccount "github.com/kenyamaneko/overload-party-account/packages/api-account"
)

const (
	invalidRequestBodyMessage = "invalid request body"
	invalidPlayerIDMessage    = "invalid player id"
)

// requiredFieldMessage は必須項目が欠けているときの応答の文面を返す。
func requiredFieldMessage(field string) string {
	return field + " is required"
}

// invalidFieldMessage は項目の値が許される範囲や形式の外のときの応答の文面を返す。
func invalidFieldMessage(field string) string {
	return field + " is invalid"
}

// respondBadRequest は message を原因とする 400 の応答を返す。
func respondBadRequest(c *gin.Context, message string) {
	c.JSON(http.StatusBadRequest, apiaccount.ErrorResponse{Error: message})
}

// bindRequestBody はリクエストボディを req に読み込み、読み込めたかを返す。
// 読み込めないとき、または requiredKeys の項目がボディに無いか null のときは、400 の応答を返す。
func bindRequestBody(c *gin.Context, req any, requiredKeys ...string) bool {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		respondBadRequest(c, invalidRequestBodyMessage)
		return false
	}
	if err := json.Unmarshal(body, req); err != nil {
		respondBadRequest(c, invalidRequestBodyMessage)
		return false
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		respondBadRequest(c, invalidRequestBodyMessage)
		return false
	}
	for _, key := range requiredKeys {
		value, isPresent := fields[key]
		// 0 や false が有効な値の項目は、生成された型では欠けているときと値が 0 や false のときを区別できないため、キーの有無で確かめる。
		if !isPresent || string(value) == "null" {
			respondBadRequest(c, requiredFieldMessage(key))
			return false
		}
	}
	return true
}

// respondIfEmpty は value が空文字のとき、field が欠けていることを示す 400 の応答を返し、true を返す。
func respondIfEmpty(c *gin.Context, field, value string) bool {
	if value != "" {
		return false
	}
	respondBadRequest(c, requiredFieldMessage(field))
	return true
}

// parsePlayerID はプレイヤーIDを UUID として読み、正規の表記で返す。UUID でないときは false を返す。
func parsePlayerID(value string) (string, bool) {
	parsed, err := uuid.Parse(value)
	if err != nil {
		return "", false
	}
	return parsed.String(), true
}

// parsePlayerIDField はリクエストボディの field のプレイヤーIDを UUID として読む。
// UUID でないときは field が不正であることを示す 400 の応答を返し、false を返す。
func parsePlayerIDField(c *gin.Context, field, value string) (string, bool) {
	playerID, isValid := parsePlayerID(value)
	if !isValid {
		respondBadRequest(c, invalidFieldMessage(field))
	}
	return playerID, isValid
}
