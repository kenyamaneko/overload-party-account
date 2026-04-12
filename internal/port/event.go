package port

import "context"

// EventSubscriber はバックグラウンドでメッセージを取得・処理するイベント駆動アダプタのインターフェースです。
type EventSubscriber interface {
	// Start は ctx がキャンセルされるまでメッセージを受信し続けます。
	Start(ctx context.Context) error
}
