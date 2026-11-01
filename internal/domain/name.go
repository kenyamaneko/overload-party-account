package domain

import (
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// MaxNameRunes は表示名の最大文字数である。
const MaxNameRunes = 20

// ErrInvalidName は表示名が業務ルールに違反していることを示す。
var ErrInvalidName = errors.New("invalid name")

// ValidateName は表示名が業務ルールを満たすかを検証する。
// 空または空白のみ、最大文字数の超過、制御文字を含む表示名は ErrInvalidName を返す。
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
