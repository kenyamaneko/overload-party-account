package usecase_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-account/internal/port"
	"github.com/kenyamaneko/overload-party-account/internal/usecase"
)

func TestIsPublisherBug(t *testing.T) {
	t.Run("[エラー分類]発行側不整合の判定", func(t *testing.T) {
		t.Run("エラーが見つからないことを示すエラー(port.ErrNotFound)を包んでいるとき、trueを返す", func(t *testing.T) {
			err := fmt.Errorf("wrapped: %w", port.ErrNotFound)

			assert.True(t, usecase.IsPublisherBug(err))
		})

		t.Run("エラーが初期ファクションの競合(ErrFactionConflict)を包んでいるとき、trueを返す", func(t *testing.T) {
			err := fmt.Errorf("wrapped: %w", usecase.ErrFactionConflict)

			assert.True(t, usecase.IsPublisherBug(err))
		})

		t.Run("port.ErrNotFoundもErrFactionConflictも包んでいないエラーのとき、falseを返す", func(t *testing.T) {
			err := errors.New("some other error")

			assert.False(t, usecase.IsPublisherBug(err))
		})
	})
}
