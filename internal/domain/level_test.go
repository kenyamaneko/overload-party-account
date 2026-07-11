package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

const testExpCoeff = 60

// levelStartExp は level n の開始時点の累計経験値を返す。
func levelStartExp(level int64) int64 {
	return testExpCoeff * level * level
}

func TestComputeLevel(t *testing.T) {
	t.Run("レベル算出", func(t *testing.T) {
		validCases := []struct {
			name         string
			newExp       int64
			currentLevel int64
			wantLevel    int64
		}{
			{
				name:         "新経験値が次レベル閾値未満のとき、レベルは据え置かれる",
				newExp:       levelStartExp(2) - 1,
				currentLevel: 1,
				wantLevel:    1,
			},
			{
				name:         "新経験値が次レベル閾値ちょうどのとき、レベルが 1 上がる",
				newExp:       levelStartExp(2),
				currentLevel: 1,
				wantLevel:    2,
			},
			{
				name:         "新経験値が複数レベル分あるとき、一度に複数レベル上がる",
				newExp:       levelStartExp(4),
				currentLevel: 1,
				wantLevel:    4,
			},
			{
				name:         "現在レベル 3 で次レベル閾値未満のとき、レベルは 3 のまま",
				newExp:       levelStartExp(4) - 1,
				currentLevel: 3,
				wantLevel:    3,
			},
			{
				name:         "新経験値が 0 でも、現在レベルより下がらない",
				newExp:       0,
				currentLevel: 5,
				wantLevel:    5,
			},
		}
		for _, tc := range validCases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := domain.ComputeLevel(tc.newExp, tc.currentLevel, testExpCoeff)
				require.NoError(t, err)
				assert.Equal(t, tc.wantLevel, got)
			})
		}

		invalidCases := []struct {
			name         string
			newExp       int64
			currentLevel int64
		}{
			{name: "currentLevel が 0 のとき、エラーになる", newExp: 0, currentLevel: 0},
			{name: "currentLevel が -1 のとき、エラーになる", newExp: 0, currentLevel: -1},
		}
		for _, tc := range invalidCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := domain.ComputeLevel(tc.newExp, tc.currentLevel, testExpCoeff)
				assert.Error(t, err)
			})
		}
	})
}

func TestComputeExpProgress(t *testing.T) {
	t.Run("経験値進捗の算出", func(t *testing.T) {
		validCases := []struct {
			name    string
			level   int64
			exp     int64
			wantExp *domain.LevelProgress
		}{
			{
				name:    "level=1 / exp=0 (初期状態) のとき、LevelExpCurrent=0・LevelExpRequired=240 になる",
				level:   1,
				exp:     0,
				wantExp: &domain.LevelProgress{LevelExpCurrent: 0, LevelExpRequired: 240},
			},
			{
				name:    "level=2 開始ちょうどのとき、LevelExpCurrent=0 になる",
				level:   2,
				exp:     levelStartExp(2),
				wantExp: &domain.LevelProgress{LevelExpCurrent: 0, LevelExpRequired: 300},
			},
			{
				name:    "現レベル内で進捗中 (level=2 / exp=500) のとき、LevelExpCurrent=260 になる",
				level:   2,
				exp:     500,
				wantExp: &domain.LevelProgress{LevelExpCurrent: 260, LevelExpRequired: 300},
			},
		}
		for _, tc := range validCases {
			t.Run(tc.name, func(t *testing.T) {
				got, err := domain.ComputeExpProgress(tc.level, tc.exp, testExpCoeff)
				require.NoError(t, err)
				assert.Equal(t, tc.wantExp, got)
			})
		}

		invalidCases := []struct {
			name  string
			level int64
			exp   int64
		}{
			{name: "level が 0 のとき、エラーになる", level: 0, exp: 0},
			{name: "level が -1 のとき、エラーになる", level: -1, exp: 0},
			{name: "exp が現レベル開始閾値未満 (level=3 / exp=0) のとき、整合性エラーになる", level: 3, exp: 0},
		}
		for _, tc := range invalidCases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := domain.ComputeExpProgress(tc.level, tc.exp, testExpCoeff)
				assert.Error(t, err)
			})
		}
	})
}
