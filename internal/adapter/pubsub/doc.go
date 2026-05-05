// Package pubsub は account サービスの Pub/Sub subscriber を管理する。
// 各 subscriber は exactly-once subscription からイベントを取得し、
// event_id をキーとした冪等トランザクション内で account スキーマに書き込む。
package pubsub
