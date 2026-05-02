package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxNameRunes は表示名の許容最大長 (rune 単位)。
// repo / DB のサイズ制約 (VARCHAR 50) ではなく、ここで定義した業務上限が真。
const MaxNameRunes = 20

// ErrInvalidName は表示名が業務ルールに違反したことを示す。
// handler 層で 400 にマップする (handler/rest/errors.go)。
var ErrInvalidName = errors.New("invalid name")

// ValidateName は表示名の業務ルールを検証する。
func ValidateName(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("%w: empty or whitespace only", ErrInvalidName)
	}
	if n := utf8.RuneCountInString(s); n > MaxNameRunes {
		return fmt.Errorf("%w: %d runes exceeds max %d", ErrInvalidName, n, MaxNameRunes)
	}
	for _, r := range s {
		if unicode.IsControl(r) {
			return fmt.Errorf("%w: contains control character", ErrInvalidName)
		}
	}
	return nil
}
