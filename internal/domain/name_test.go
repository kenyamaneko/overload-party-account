package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// ValidateName の業務契約をテーブル駆動で固定する。
// 上限値・空文字・空白のみ・制御文字の各カテゴリで境界値を 1 ケースずつ覆う。
func TestValidateName(t *testing.T) {
	noErr := func(t *testing.T, err error) { t.Helper(); require.NoError(t, err) }
	errIs := func(target error) func(*testing.T, error) {
		return func(t *testing.T, err error) { t.Helper(); require.ErrorIs(t, err, target) }
	}

	tests := []struct {
		name      string
		input     string
		assertErr func(*testing.T, error)
	}{
		{name: "ASCII の通常名は OK", input: "Alice", assertErr: noErr},
		{name: "多バイト文字の通常名は OK", input: "あいう", assertErr: noErr},
		{name: "境界: ちょうど MaxNameRunes (20) は OK", input: strings.Repeat("a", MaxNameRunes), assertErr: noErr},
		{name: "境界: 多バイト文字でちょうど MaxNameRunes は OK", input: strings.Repeat("あ", MaxNameRunes), assertErr: noErr},
		{name: "境界: MaxNameRunes + 1 (21) は不正", input: strings.Repeat("a", MaxNameRunes+1), assertErr: errIs(ErrInvalidName)},
		{name: "空文字は不正", input: "", assertErr: errIs(ErrInvalidName)},
		{name: "全角スペースを含む空白のみは不正", input: "  　 ", assertErr: errIs(ErrInvalidName)},
		{name: "改行を含むのは不正 (制御文字)", input: "ab\ncd", assertErr: errIs(ErrInvalidName)},
		{name: "タブを含むのは不正 (制御文字)", input: "ab\tcd", assertErr: errIs(ErrInvalidName)},
		{name: "NULL バイトを含むのは不正 (制御文字)", input: "ab\x00cd", assertErr: errIs(ErrInvalidName)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.assertErr(t, ValidateName(tt.input))
		})
	}
}
