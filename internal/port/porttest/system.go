package porttest

import (
	"fmt"
	"time"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

const sequentialPlayerIDFormat = "00000000-0000-4000-8000-%012d"

var (
	_ port.Clock             = FixedClock{}
	_ port.PlayerIDGenerator = FixedPlayerIDGenerator{}
	_ port.PlayerIDGenerator = (*SequentialPlayerIDGenerator)(nil)
)

// FixedClock は常に同じ時刻を返す port.Clock である。
type FixedClock struct {
	now time.Time
}

// NewFixedClock は常に now を返す FixedClock を返す。
func NewFixedClock(now time.Time) FixedClock {
	return FixedClock{now: now}
}

// Now は固定した時刻を返す。
func (c FixedClock) Now() time.Time {
	return c.now
}

// FixedPlayerIDGenerator は常に同じプレイヤーIDを返す port.PlayerIDGenerator である。
type FixedPlayerIDGenerator struct {
	playerID string
}

// NewFixedPlayerIDGenerator は常に playerID を返す FixedPlayerIDGenerator を返す。
func NewFixedPlayerIDGenerator(playerID string) FixedPlayerIDGenerator {
	return FixedPlayerIDGenerator{playerID: playerID}
}

// GeneratePlayerID は固定したプレイヤーIDを返す。
func (g FixedPlayerIDGenerator) GeneratePlayerID() string {
	return g.playerID
}

// SequentialPlayerIDGenerator は呼ばれるたびに、連番を含む UUID 形式のプレイヤーIDを返す port.PlayerIDGenerator である。
type SequentialPlayerIDGenerator struct {
	issued int
}

// NewSequentialPlayerIDGenerator は、まだ何も採番していない SequentialPlayerIDGenerator を返す。
func NewSequentialPlayerIDGenerator() *SequentialPlayerIDGenerator {
	return &SequentialPlayerIDGenerator{}
}

// GeneratePlayerID は 1 から始まる連番を含むプレイヤーIDを返す。
func (g *SequentialPlayerIDGenerator) GeneratePlayerID() string {
	g.issued++
	return fmt.Sprintf(sequentialPlayerIDFormat, g.issued)
}
