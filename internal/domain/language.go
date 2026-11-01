package domain

import (
	"errors"
	"fmt"
	"unicode/utf8"
)

// MaxLanguageLength は言語の最大文字数である。
const MaxLanguageLength = 10

var (
	// ErrLanguageEmpty は言語が空であることを示す。
	ErrLanguageEmpty = errors.New("language is empty")
	// ErrLanguageTooLong は言語が最大文字数を超えていることを示す。
	ErrLanguageTooLong = errors.New("language exceeds the maximum length")
)

// ValidateLanguage は言語が業務ルールを満たすかを検証する。
func ValidateLanguage(language string) error {
	if language == "" {
		return ErrLanguageEmpty
	}
	if length := utf8.RuneCountInString(language); length > MaxLanguageLength {
		return fmt.Errorf("%w: %d characters exceeds max %d", ErrLanguageTooLong, length, MaxLanguageLength)
	}
	return nil
}
