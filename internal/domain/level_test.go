package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

func TestComputeLevel(t *testing.T) {
	t.Run("レベル進捗計算", func(t *testing.T) {
		t.Run("現在レベルが1未満のとき、エラーを返す", func(t *testing.T) {
			_, err := domain.ComputeLevel(0, 0, 100)

			require.Error(t, err)
		})

		t.Run("累計経験値が次レベル必要経験値未満のとき、レベルは現在レベルのまま変わらない", func(t *testing.T) {
			// coeff=100, currentLevel=1 の次レベル必要経験値は 100*(1+1)^2=400
			level, err := domain.ComputeLevel(399, 1, 100)

			require.NoError(t, err)
			assert.Equal(t, int64(1), level)
		})

		t.Run("累計経験値が次レベル必要経験値ちょうどのとき、レベルが1つ上がる", func(t *testing.T) {
			level, err := domain.ComputeLevel(400, 1, 100)

			require.NoError(t, err)
			assert.Equal(t, int64(2), level)
		})

		t.Run("累計経験値が2〜3レベル分の必要経験値をまとめて超えているとき、超えた分だけ一度に複数レベル上げた値を返す", func(t *testing.T) {
			// coeff=100, currentLevel=1: レベル3必要経験値は100*(2+1)^2=900、レベル4必要経験値は100*(3+1)^2=1600
			level, err := domain.ComputeLevel(1000, 1, 100)

			require.NoError(t, err)
			assert.Equal(t, int64(3), level)
		})
	})
}

func TestComputeExpProgress(t *testing.T) {
	t.Run("レベル進捗計算", func(t *testing.T) {
		t.Run("レベルが1未満のとき、エラーを返す", func(t *testing.T) {
			_, err := domain.ComputeExpProgress(0, 0, 100)

			require.Error(t, err)
		})

		t.Run("累計経験値が現在レベルの開始閾値未満のとき、エラーを返す", func(t *testing.T) {
			// coeff=100, level=2 の開始閾値は 100*2^2=400
			_, err := domain.ComputeExpProgress(2, 399, 100)

			require.Error(t, err)
		})

		t.Run("累計経験値が開始閾値ちょうどのとき、現在レベル内の経験値進捗は0になる", func(t *testing.T) {
			progress, err := domain.ComputeExpProgress(2, 400, 100)

			require.NoError(t, err)
			assert.Equal(t, int64(0), progress.LevelExpCurrent)
		})

		t.Run("累計経験値が開始閾値を超えているとき、現在レベル内の経験値進捗は累計経験値から開始閾値を引いた値になる", func(t *testing.T) {
			// coeff=100, level=2 の開始閾値は 400、累計経験値 450 との差は 50
			progress, err := domain.ComputeExpProgress(2, 450, 100)

			require.NoError(t, err)
			assert.Equal(t, int64(50), progress.LevelExpCurrent)
		})

		t.Run("次レベルまでに必要な経験値の幅は、次レベル必要経験値から現在レベルの開始閾値を引いた値になる", func(t *testing.T) {
			// coeff=100, level=2: 次レベル(3)必要経験値は100*(2+1)^2=900、開始閾値は400、差は500
			progress, err := domain.ComputeExpProgress(2, 450, 100)

			require.NoError(t, err)
			assert.Equal(t, int64(500), progress.LevelExpRequired)
		})
	})
}
