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
			{name: "ASCII の通常名のとき、エラーにならない", input: "Alice"},
			{name: "多バイト文字の通常名のとき、エラーにならない", input: "あいう"},
			{name: "ちょうど MaxNameRunes (20 文字) のとき、エラーにならない", input: strings.Repeat("a", MaxNameRunes)},
			{name: "多バイト文字でちょうど MaxNameRunes のとき、エラーにならない", input: strings.Repeat("あ", MaxNameRunes)},
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
			{name: "MaxNameRunes + 1 (21 文字) のとき、ErrInvalidName になる", input: strings.Repeat("a", MaxNameRunes+1)},
			{name: "空文字のとき、ErrInvalidName になる", input: ""},
			{name: "全角スペースを含む空白のみのとき、ErrInvalidName になる", input: "  　 "},
			{name: "改行 (制御文字) を含むとき、ErrInvalidName になる", input: "ab\ncd"},
			{name: "タブ (制御文字) を含むとき、ErrInvalidName になる", input: "ab\tcd"},
			{name: "NULL バイト (制御文字) を含むとき、ErrInvalidName になる", input: "ab\x00cd"},
		}
		for _, tc := range invalidCases {
			t.Run(tc.name, func(t *testing.T) {
				require.ErrorIs(t, ValidateName(tc.input), ErrInvalidName)
			})
		}
	})
}
