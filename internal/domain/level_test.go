package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

func TestComputeLevel(t *testing.T) {
	t.Run("[レベル進捗計算]現在レベルの算出", func(t *testing.T) {
		t.Run("現在レベルが1未満のとき、エラーを返す", func(t *testing.T) {
			_, err := domain.ComputeLevel(0, 0, 100)

			require.Error(t, err)
		})

		t.Run("係数を100とすると、現在レベルが1、累計経験値が399のとき、レベルは1のまま変わらない", func(t *testing.T) {
			level, err := domain.ComputeLevel(399, 1, 100)

			require.NoError(t, err)
			assert.Equal(t, int64(1), level)
		})

		t.Run("係数を100とすると、現在レベルが1、累計経験値が400のとき、レベルは2になる", func(t *testing.T) {
			level, err := domain.ComputeLevel(400, 1, 100)

			require.NoError(t, err)
			assert.Equal(t, int64(2), level)
		})

		t.Run("係数を100とすると、現在レベルが1、累計経験値が1000のとき、複数レベル分をまとめて上げた3になる", func(t *testing.T) {
			level, err := domain.ComputeLevel(1000, 1, 100)

			require.NoError(t, err)
			assert.Equal(t, int64(3), level)
		})
	})
}

func TestComputeExpProgress(t *testing.T) {
	t.Run("[レベル進捗計算]レベル内経験値進捗の算出", func(t *testing.T) {
		t.Run("レベルが1未満のとき、エラーを返す", func(t *testing.T) {
			_, err := domain.ComputeExpProgress(0, 0, 100)

			require.Error(t, err)
		})

		t.Run("係数を100とすると、レベルが2、累計経験値が399のとき、エラーを返す", func(t *testing.T) {
			_, err := domain.ComputeExpProgress(2, 399, 100)

			require.Error(t, err)
		})

		t.Run("係数を100とすると、レベルが2、累計経験値が400のとき、現在レベル内の経験値進捗は0になる", func(t *testing.T) {
			progress, err := domain.ComputeExpProgress(2, 400, 100)

			require.NoError(t, err)
			assert.Equal(t, int64(0), progress.LevelExpCurrent)
		})

		t.Run("係数を100とすると、レベルが2、累計経験値が450のとき、現在レベル内の経験値進捗は50になる", func(t *testing.T) {
			progress, err := domain.ComputeExpProgress(2, 450, 100)

			require.NoError(t, err)
			assert.Equal(t, int64(50), progress.LevelExpCurrent)
		})

		t.Run("係数を100とすると、レベルが2、累計経験値が450のとき、次レベルまでに必要な経験値の幅は500になる", func(t *testing.T) {
			progress, err := domain.ComputeExpProgress(2, 450, 100)

			require.NoError(t, err)
			assert.Equal(t, int64(500), progress.LevelExpRequired)
		})
	})
}
