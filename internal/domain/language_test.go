package domain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

func TestValidateLanguage(t *testing.T) {
	t.Run("[プレイヤー設定]言語の検証", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			tests := []struct {
				name  string
				input string
			}{
				{name: "言語が「あ」の1文字のとき、検証に成功する", input: "あ"},
				{name: "言語が「あいうえおかきくけこ」(10文字)のとき、検証に成功する", input: "あいうえおかきくけこ"},
				{name: "言語が「abcdefghij」(10文字)のとき、検証に成功する", input: "abcdefghij"},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					err := domain.ValidateLanguage(tt.input)

					require.NoError(t, err)
				})
			}
		})

		t.Run("異常系", func(t *testing.T) {
			tests := []struct {
				name    string
				input   string
				wantErr error
			}{
				{name: "言語が空文字のとき、言語が空であることを示すエラーになる", input: "", wantErr: domain.ErrLanguageEmpty},
				{name: "言語が「あいうえおかきくけこさ」(11文字)のとき、言語が10文字を超えていることを示すエラーになる", input: "あいうえおかきくけこさ", wantErr: domain.ErrLanguageTooLong},
				{name: "言語が「abcdefghijk」(11文字)のとき、言語が10文字を超えていることを示すエラーになる", input: "abcdefghijk", wantErr: domain.ErrLanguageTooLong},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					err := domain.ValidateLanguage(tt.input)

					require.ErrorIs(t, err, tt.wantErr)
				})
			}
		})
	})
}
