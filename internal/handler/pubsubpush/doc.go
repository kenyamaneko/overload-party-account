// Package pubsubpush は Cloud Pub/Sub push subscription 向けの HTTP 受け口を提供する。
// push envelope の decode とイベント処理結果の応答コードへの変換を共通化し、
// イベントごとの処理は port.MessageHandler の差し替えで表現する。
package pubsubpush
