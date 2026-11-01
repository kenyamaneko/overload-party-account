package port

import "time"

// Clock は現在時刻を返す。
type Clock interface {
	// Now は現在時刻を返す。
	Now() time.Time
}

// PlayerIDGenerator は新しいプレイヤーIDを採番する。
type PlayerIDGenerator interface {
	// GeneratePlayerID は UUID 形式の新しいプレイヤーIDを返す。
	GeneratePlayerID() string
}
