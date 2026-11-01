package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

func assertComputedLevel(t *testing.T, exp, coefficient, want int64) {
	t.Helper()

	got, err := domain.ComputeLevel(exp, coefficient)

	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func assertExpCurrent(t *testing.T, exp, coefficient, want int64) {
	t.Helper()

	got, err := domain.ComputeExpProgress(exp, coefficient)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want, got.LevelExpCurrent)
}

func assertExpRequired(t *testing.T, exp, coefficient, want int64) {
	t.Helper()

	got, err := domain.ComputeExpProgress(exp, coefficient)

	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, want, got.LevelExpRequired)
}

func TestComputeLevel(t *testing.T) {
	t.Run("[レベル]累計経験値からのレベル算出", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			tests := []struct {
				name        string
				coefficient int64
				exp         int64
				want        int64
			}{
				{name: "係数が100で累計経験値が0のとき、レベルは1になる", coefficient: 100, exp: 0, want: 1},
				{name: "係数が100で累計経験値が399のとき、レベルは1になる", coefficient: 100, exp: 399, want: 1},
				{name: "係数が100で累計経験値が400のとき、レベルは2になる", coefficient: 100, exp: 400, want: 2},
				{name: "係数が100で累計経験値が899のとき、レベルは2になる", coefficient: 100, exp: 899, want: 2},
				{name: "係数が100で累計経験値が900のとき、レベルは3になる", coefficient: 100, exp: 900, want: 3},
				{name: "係数が100で累計経験値が1599のとき、レベルは3になる", coefficient: 100, exp: 1599, want: 3},
				{name: "係数が100で累計経験値が1600のとき、レベルは4になる", coefficient: 100, exp: 1600, want: 4},
				{name: "係数が100で累計経験値が12099のとき、レベルは10になる", coefficient: 100, exp: 12099, want: 10},
				{name: "係数が100で累計経験値が12100のとき、レベルは11になる", coefficient: 100, exp: 12100, want: 11},
				{name: "係数が60で累計経験値が239のとき、レベルは1になる", coefficient: 60, exp: 239, want: 1},
				{name: "係数が60で累計経験値が240のとき、レベルは2になる", coefficient: 60, exp: 240, want: 2},
				{name: "係数が1で累計経験値が3のとき、レベルは1になる", coefficient: 1, exp: 3, want: 1},
				{name: "係数が1で累計経験値が4のとき、レベルは2になる", coefficient: 1, exp: 4, want: 2},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					assertComputedLevel(t, tt.exp, tt.coefficient, tt.want)
				})
			}
		})

		t.Run("異常系", func(t *testing.T) {
			tests := []struct {
				name        string
				exp         int64
				coefficient int64
				wantErr     error
			}{
				{name: "累計経験値が-1のとき、累計経験値が負であることを示すエラーになる", exp: -1, coefficient: 100, wantErr: domain.ErrExpBelowLevelThreshold},
				{name: "係数が0のとき、係数が1未満であることを示すエラーになる", exp: 0, coefficient: 0, wantErr: domain.ErrInvalidExpCoefficient},
			}
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					_, err := domain.ComputeLevel(tt.exp, tt.coefficient)

					require.ErrorIs(t, err, tt.wantErr)
				})
			}
		})
	})
}

func TestComputeExpProgress(t *testing.T) {
	t.Run("[レベル]累計経験値からのレベル内の経験値進捗の算出", func(t *testing.T) {
		t.Run("正常系", func(t *testing.T) {
			t.Run("レベル内の経験値を取得すると", func(t *testing.T) {
				tests := []struct {
					name        string
					coefficient int64
					exp         int64
					want        int64
				}{
					{name: "係数が100で累計経験値が0のとき、レベル内の経験値は0になる", coefficient: 100, exp: 0, want: 0},
					{name: "係数が100で累計経験値が399のとき、レベル内の経験値は399になる", coefficient: 100, exp: 399, want: 399},
					{name: "係数が100で累計経験値が400のとき、レベル内の経験値は0になる", coefficient: 100, exp: 400, want: 0},
					{name: "係数が100で累計経験値が899のとき、レベル内の経験値は499になる", coefficient: 100, exp: 899, want: 499},
					{name: "係数が100で累計経験値が900のとき、レベル内の経験値は0になる", coefficient: 100, exp: 900, want: 0},
					{name: "係数が60で累計経験値が239のとき、レベル内の経験値は239になる", coefficient: 60, exp: 239, want: 239},
					{name: "係数が60で累計経験値が240のとき、レベル内の経験値は0になる", coefficient: 60, exp: 240, want: 0},
					{name: "係数が60で累計経験値が539のとき、レベル内の経験値は299になる", coefficient: 60, exp: 539, want: 299},
					{name: "係数が60で累計経験値が540のとき、レベル内の経験値は0になる", coefficient: 60, exp: 540, want: 0},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						assertExpCurrent(t, tt.exp, tt.coefficient, tt.want)
					})
				}
			})

			t.Run("現在のレベルの開始から次のレベルの開始までに必要な経験値の幅を取得すると", func(t *testing.T) {
				tests := []struct {
					name        string
					coefficient int64
					exp         int64
					want        int64
				}{
					{name: "係数が100で累計経験値が399のとき、現在のレベルの開始から次のレベルの開始までに必要な経験値の幅は400になる", coefficient: 100, exp: 399, want: 400},
					{name: "係数が100で累計経験値が400のとき、現在のレベルの開始から次のレベルの開始までに必要な経験値の幅は500になる", coefficient: 100, exp: 400, want: 500},
					{name: "係数が100で累計経験値が900のとき、現在のレベルの開始から次のレベルの開始までに必要な経験値の幅は700になる", coefficient: 100, exp: 900, want: 700},
					{name: "係数が100で累計経験値が12099のとき、現在のレベルの開始から次のレベルの開始までに必要な経験値の幅は2100になる", coefficient: 100, exp: 12099, want: 2100},
					{name: "係数が100で累計経験値が12100のとき、現在のレベルの開始から次のレベルの開始までに必要な経験値の幅は2300になる", coefficient: 100, exp: 12100, want: 2300},
					{name: "係数が60で累計経験値が239のとき、現在のレベルの開始から次のレベルの開始までに必要な経験値の幅は240になる", coefficient: 60, exp: 239, want: 240},
					{name: "係数が60で累計経験値が240のとき、現在のレベルの開始から次のレベルの開始までに必要な経験値の幅は300になる", coefficient: 60, exp: 240, want: 300},
					{name: "係数が1で累計経験値が0のとき、現在のレベルの開始から次のレベルの開始までに必要な経験値の幅は4になる", coefficient: 1, exp: 0, want: 4},
				}
				for _, tt := range tests {
					t.Run(tt.name, func(t *testing.T) {
						assertExpRequired(t, tt.exp, tt.coefficient, tt.want)
					})
				}
			})
		})

		t.Run("異常系", func(t *testing.T) {
			t.Run("累計経験値が-1のとき、累計経験値が負であることを示すエラーになる", func(t *testing.T) {
				_, err := domain.ComputeExpProgress(-1, 100)

				require.ErrorIs(t, err, domain.ErrExpBelowLevelThreshold)
			})
		})
	})
}
