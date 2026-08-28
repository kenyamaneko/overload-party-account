package domain_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

func TestValidateName(t *testing.T) {
	t.Run("[表示名バリデーション]表示名の妥当性検証", func(t *testing.T) {
		t.Run("空文字のとき、エラーを返す", func(t *testing.T) {
			err := domain.ValidateName("")

			assert.ErrorIs(t, err, domain.ErrInvalidName)
		})

		t.Run("空白文字のみで構成されているとき、エラーを返す", func(t *testing.T) {
			err := domain.ValidateName("   ")

			assert.ErrorIs(t, err, domain.ErrInvalidName)
		})

		t.Run("文字数が上限(20文字)を超えるとき、エラーを返す", func(t *testing.T) {
			err := domain.ValidateName(strings.Repeat("あ", domain.MaxNameRunes+1))

			assert.ErrorIs(t, err, domain.ErrInvalidName)
		})

		t.Run("文字数がちょうど上限(20文字)のとき、エラーにならない", func(t *testing.T) {
			err := domain.ValidateName(strings.Repeat("あ", domain.MaxNameRunes))

			require.NoError(t, err)
		})

		t.Run("制御文字が含まれるとき、エラーを返す", func(t *testing.T) {
			err := domain.ValidateName("abc\x07def")

			assert.ErrorIs(t, err, domain.ErrInvalidName)
		})

		t.Run("空白以外の文字を含み、上限以下で、制御文字を含まないとき、エラーにならない", func(t *testing.T) {
			err := domain.ValidateName("プレイヤー1号")

			require.NoError(t, err)
		})
	})
}
