// Package pubsub は account サービスの Pub/Sub subscriber を管理する。
// 各 subscriber は push 受け口 (internal/handler/pubsubpush) から 1 件ずつ渡される
// イベントを、event_id をキーとした冪等トランザクション内で account スキーマに書き込む。
//
// ペイロードの event_type が想定と異なる場合、各 subscriber はエラーではなく成功
// として応答する (warn ログのみ)。publisher 側が将来イベント種別を追加したときに、
// この subscriber を止めないための挙動。
package pubsub
