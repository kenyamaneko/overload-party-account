package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/kenyamaneko/overload-party-scenario/packages/api-scenario/apiscenariofake"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// apiscenariofakeStream は apiscenariofake.Subscriber を port.MessageStream として
// 露出するテスト用 adapter。apishopfakeStream と同じ設計 (eager subscribe +
// handled channel) を scenario 側 fake 向けに再掲している。
//
// handler の戻り値 (ack/nack 相当) は handled channel に転送され、テストは
// ExpectHandled で同期的に観測する。nack 後の再配信はせず、in-memory の
// at-most-once 配信として振る舞う (retry 挙動を確かめたいテストは別 stream 実装
// を書く)。
type apiscenariofakeStream struct {
	ch      <-chan []byte
	topic   string
	handled chan error
}

func newApiscenariofakeStream(sub *apiscenariofake.Subscriber, topic string) *apiscenariofakeStream {
	return &apiscenariofakeStream{
		ch:      sub.Messages(topic),
		topic:   topic,
		handled: make(chan error, 16),
	}
}

// Consume は ctx がキャンセルされるまで subscriber のメッセージを handler に渡し、
// handler の戻り値を handled channel に流す。
func (s *apiscenariofakeStream) Consume(ctx context.Context, handler port.MessageHandler) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case data, ok := <-s.ch:
			if !ok {
				return nil
			}
			s.handled <- handler(ctx, data)
		}
	}
}

// ExpectHandled は 1 メッセージ分の handler 戻り値を timeout 付きで取り出す。
func (s *apiscenariofakeStream) ExpectHandled(t *testing.T, timeout time.Duration) error {
	t.Helper()
	select {
	case err := <-s.handled:
		return err
	case <-time.After(timeout):
		t.Fatalf("apiscenariofakeStream[%s]: timeout waiting for handler completion (%s)", s.topic, timeout)
		return nil
	}
}
