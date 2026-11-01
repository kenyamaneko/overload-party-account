package system

import "github.com/google/uuid"

// UUIDPlayerIDGenerator は乱数による UUID をプレイヤーIDとして採番する。
type UUIDPlayerIDGenerator struct{}

// NewUUIDPlayerIDGenerator は UUIDPlayerIDGenerator を生成する。
func NewUUIDPlayerIDGenerator() *UUIDPlayerIDGenerator {
	return &UUIDPlayerIDGenerator{}
}

// GeneratePlayerID は UUID 形式の新しいプレイヤーIDを返す。
func (*UUIDPlayerIDGenerator) GeneratePlayerID() string {
	return uuid.NewString()
}
