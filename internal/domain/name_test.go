package domain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

func TestValidateName(t *testing.T) {
	t.Run("[表示名]表示名の検証", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			tests := []struct {
				name  string
				input string
			}{
				{name: "表示名が「あ」の1文字のとき、検証に成功する", input: "あ"},
				{name: "表示名が「あいうえおかきくけこさしすせそたちつてと」(20文字)のとき、検証に成功する", input: "あいうえおかきくけこさしすせそたちつてと"},
				{name: "表示名が「abcdefghijklmnopqrst」(20文字)のとき、検証に成功する", input: "abcdefghijklmnopqrst"},
				{name: "表示名が「山田」と「太郎」の間に半角スペースを1つ含むとき、検証に成功する", input: "山田 太郎"},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					err := domain.ValidateName(tt.input)

					require.NoError(t, err)
				})
			}
		})

		t.Run("異常系", func(t *testing.T) {
			tests := []struct {
				name         string
				input        string
				wantFragment string
			}{
				{name: "表示名が空文字のとき、表示名が空または空白のみであることを示すエラーになる", input: "", wantFragment: "empty or whitespace only"},
				{name: "表示名が半角スペース1つのとき、表示名が空または空白のみであることを示すエラーになる", input: " ", wantFragment: "empty or whitespace only"},
				{name: "表示名が全角スペース1つのとき、表示名が空または空白のみであることを示すエラーになる", input: "　", wantFragment: "empty or whitespace only"},
				{name: "表示名が「あいうえおかきくけこさしすせそたちつてとな」(21文字)のとき、表示名が20文字を超えていることを示すエラーになる", input: "あいうえおかきくけこさしすせそたちつてとな", wantFragment: "21 runes exceeds max 20"},
				{name: "表示名が「abcdefghijklmnopqrstu」(21文字)のとき、表示名が20文字を超えていることを示すエラーになる", input: "abcdefghijklmnopqrstu", wantFragment: "21 runes exceeds max 20"},
				{name: "表示名が「あ」と「い」の間に改行を含むとき、表示名が制御文字を含むことを示すエラーになる", input: "あ\nい", wantFragment: "contains control character"},
				{name: "表示名が「あ」と「い」の間にタブ文字を含むとき、表示名が制御文字を含むことを示すエラーになる", input: "あ\tい", wantFragment: "contains control character"},
				{name: "表示名がNUL文字を含むとき、表示名が制御文字を含むことを示すエラーになる", input: "\x00", wantFragment: "contains control character"},
				{name: "表示名がDEL文字(U+007F)を含むとき、表示名が制御文字を含むことを示すエラーになる", input: "\x7f", wantFragment: "contains control character"},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					err := domain.ValidateName(tt.input)

					require.ErrorIs(t, err, domain.ErrInvalidName)
					require.ErrorContains(t, err, tt.wantFragment)
				})
			}
		})
	})
}
