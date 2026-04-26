package model

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ValidateName の業務契約をテーブル駆動で固定する。
// 上限値・空文字・空白のみ・制御文字の各カテゴリで境界値を 1 ケースずつ覆う。
func TestValidateName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "ASCII の通常名は OK", input: "Alice", wantErr: false},
		{name: "多バイト文字の通常名は OK", input: "あいう", wantErr: false},
		{name: "境界: ちょうど MaxNameRunes (20) は OK", input: strings.Repeat("a", MaxNameRunes), wantErr: false},
		{name: "境界: 多バイト文字でちょうど MaxNameRunes は OK", input: strings.Repeat("あ", MaxNameRunes), wantErr: false},
		{name: "境界: MaxNameRunes + 1 (21) は不正", input: strings.Repeat("a", MaxNameRunes+1), wantErr: true},
		{name: "空文字は不正", input: "", wantErr: true},
		{name: "全角スペースを含む空白のみは不正", input: "  　 ", wantErr: true},
		{name: "改行を含むのは不正 (制御文字)", input: "ab\ncd", wantErr: true},
		{name: "タブを含むのは不正 (制御文字)", input: "ab\tcd", wantErr: true},
		{name: "NULL バイトを含むのは不正 (制御文字)", input: "ab\x00cd", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, errors.Is(err, ErrInvalidName),
					"want ErrInvalidName, got %v", err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
