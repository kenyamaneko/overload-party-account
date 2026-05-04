package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/kenyamaneko/overload-party-account/internal/domain"
)

const testExpCoeff = 60

func TestComputeLevel(t *testing.T) {
	threshold := func(level int64) int64 {
		return testExpCoeff * level * level
	}

	tests := []struct {
		name         string
		newExp       int64
		currentLevel int64
		wantLevel    int64
	}{
		{
			name:         "閾値未満なら同じレベルのまま",
			newExp:       threshold(2) - 1,
			currentLevel: 1,
			wantLevel:    1,
		},
		{
			name:         "閾値ちょうどでレベルアップする",
			newExp:       threshold(2),
			currentLevel: 1,
			wantLevel:    2,
		},
		{
			name:         "一度に複数レベル上がる",
			newExp:       threshold(4),
			currentLevel: 1,
			wantLevel:    4,
		},
		{
			name:         "次の閾値未満なら現在レベル据え置き",
			newExp:       threshold(4) - 1,
			currentLevel: 3,
			wantLevel:    3,
		},
		{
			name:         "レベル 0 は 1 に補正される",
			newExp:       0,
			currentLevel: 0,
			wantLevel:    1,
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
			got := domain.ComputeLevel(tt.newExp, tt.currentLevel, testExpCoeff)
			assert.Equal(t, tt.wantLevel, got)
		})
	}
}

func TestComputeLevelProgress(t *testing.T) {
	tests := []struct {
		name             string
		level            int64
		exp              int64
		coeff            int64
		wantLevelCurrent int64
		wantLevelReq     int64
	}{
		{
			name:             "level=1 / exp=0 / coeff=100",
			level:            1, exp: 0, coeff: 100,
			wantLevelCurrent: 0, wantLevelReq: 300,
		},
		{
			name:             "level=2 / exp=500 / coeff=100",
			level:            2, exp: 500, coeff: 100,
			wantLevelCurrent: 100, wantLevelReq: 500,
		},
		{
			name:             "exp が currentLevelExp 未満なら LevelExpCurrent は 0 にクランプ",
			level:            3, exp: 0, coeff: 100,
			wantLevelCurrent: 0, wantLevelReq: 700,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := domain.ComputeLevelProgress(tt.level, tt.exp, tt.coeff)
			assert.Equal(t, tt.wantLevelCurrent, got.LevelExpCurrent)
			assert.Equal(t, tt.wantLevelReq, got.LevelExpRequired)
		})
	}
}
