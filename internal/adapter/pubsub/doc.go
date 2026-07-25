// Package pubsub は account サービスの Pub/Sub subscriber を管理する。
// 各 subscriber は push 受け口 (internal/handler/pubsubpush) から 1 件ずつ渡される
// イベントを、event_id をキーとした冪等トランザクション内で account スキーマに書き込む。
package pubsub
