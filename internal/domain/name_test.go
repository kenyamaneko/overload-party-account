package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateName(t *testing.T) {
	t.Run("表示名のバリデーション", func(t *testing.T) {
		validCases := []struct {
			name  string
			input string
		}{
			{name: "ASCIIの通常名のとき、エラーにならない", input: "Alice"},
			{name: "多バイト文字の通常名のとき、エラーにならない", input: "あいう"},
			{name: "ちょうどMaxNameRunes (20文字)のとき、エラーにならない", input: strings.Repeat("a", MaxNameRunes)},
			{name: "多バイト文字でちょうどMaxNameRunesのとき、エラーにならない", input: strings.Repeat("あ", MaxNameRunes)},
		}
		for _, tc := range validCases {
			t.Run(tc.name, func(t *testing.T) {
				require.NoError(t, ValidateName(tc.input))
			})
		}

		invalidCases := []struct {
			name  string
			input string
		}{
			{name: "MaxNameRunes + 1 (21文字)のとき、ErrInvalidNameになる", input: strings.Repeat("a", MaxNameRunes+1)},
			{name: "空文字のとき、ErrInvalidNameになる", input: ""},
			{name: "全角スペースを含む空白のみのとき、ErrInvalidNameになる", input: "  　 "},
			{name: "改行 (制御文字)を含むとき、ErrInvalidNameになる", input: "ab\ncd"},
			{name: "タブ (制御文字)を含むとき、ErrInvalidNameになる", input: "ab\tcd"},
			{name: "NULLバイト (制御文字)を含むとき、ErrInvalidNameになる", input: "ab\x00cd"},
		}
		for _, tc := range invalidCases {
			t.Run(tc.name, func(t *testing.T) {
				require.ErrorIs(t, ValidateName(tc.input), ErrInvalidName)
			})
		}
	})
}
