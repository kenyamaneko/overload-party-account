package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

const testExpCoeff = 60

// levelStartExp は level n の開始時点の累計経験値を返す。
func levelStartExp(level int64) int64 {
	return testExpCoeff * level * level
}

func TestComputeLevel(t *testing.T) {
	tests := []struct {
		name         string
		newExp       int64
		currentLevel int64
		wantLevel    int64
	}{
		{
			name:         "閾値未満なら同じレベルのまま",
			newExp:       levelStartExp(2) - 1,
			currentLevel: 1,
			wantLevel:    1,
		},
		{
			name:         "閾値ちょうどでレベルアップする",
			newExp:       levelStartExp(2),
			currentLevel: 1,
			wantLevel:    2,
		},
		{
			name:         "一度に複数レベル上がる",
			newExp:       levelStartExp(4),
			currentLevel: 1,
			wantLevel:    4,
		},
		{
			name:         "次の閾値未満なら現在レベル据え置き",
			newExp:       levelStartExp(4) - 1,
			currentLevel: 3,
			wantLevel:    3,
		},
		{
			name:         "レベルは下がらない",
			newExp:       0,
			currentLevel: 5,
			wantLevel:    5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.ComputeLevel(tt.newExp, tt.currentLevel, testExpCoeff)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantLevel, got)
		})
	}
}

func TestComputeLevel_Invalid(t *testing.T) {
	tests := []struct {
		name         string
		newExp       int64
		currentLevel int64
	}{
		{
			name:         "currentLevel が 0 ならエラー",
			newExp:       0,
			currentLevel: 0,
		},
		{
			name:         "currentLevel が負ならエラー",
			newExp:       0,
			currentLevel: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.ComputeLevel(tt.newExp, tt.currentLevel, testExpCoeff)
			assert.Error(t, err)
		})
	}
}

func TestComputeExpProgress(t *testing.T) {
	tests := []struct {
		name    string
		level   int64
		exp     int64
		wantExp *domain.LevelProgress
	}{
		{
			name:    "level=1 はプレイヤー初期状態として exp=0 から始まる",
			level:   1,
			exp:     0,
			wantExp: &domain.LevelProgress{LevelExpCurrent: 0, LevelExpRequired: 240},
		},
		{
			name:    "level=2 開始ちょうどなら LevelExpCurrent は 0",
			level:   2,
			exp:     levelStartExp(2),
			wantExp: &domain.LevelProgress{LevelExpCurrent: 0, LevelExpRequired: 300},
		},
		{
			name:    "現レベル内で進捗中",
			level:   2,
			exp:     500,
			wantExp: &domain.LevelProgress{LevelExpCurrent: 260, LevelExpRequired: 300},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := domain.ComputeExpProgress(tt.level, tt.exp, testExpCoeff)
			assert.NoError(t, err)
			assert.Equal(t, tt.wantExp, got)
		})
	}
}

func TestComputeExpProgress_Invalid(t *testing.T) {
	tests := []struct {
		name  string
		level int64
		exp   int64
	}{
		{
			name:  "level が 0 ならエラー",
			level: 0,
			exp:   0,
		},
		{
			name:  "level が負ならエラー",
			level: -1,
			exp:   0,
		},
		{
			name:  "exp が現レベル開始閾値未満なら整合性エラー",
			level: 3,
			exp:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := domain.ComputeExpProgress(tt.level, tt.exp, testExpCoeff)
			assert.Error(t, err)
		})
	}
}
