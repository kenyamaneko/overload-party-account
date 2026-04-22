package port

import "context"

// MessageHandler は 1 メッセージの処理結果を ack / nack の形で返す。
// nil = ack (正常処理 or 責務外として ACK できるケース)、非 nil = nack
// (ペイロード不正や副作用失敗で再配信を要するケース)。
//
// 戻り値の意味論を「error か否か」に一本化することで、adapter は
// handler の単一戻り値を見て Ack / Nack のいずれを呼ぶかを決められる。
type MessageHandler func(ctx context.Context, data []byte) error

// MessageStream は Pub/Sub subscription の抽象境界。account は GCP Pub/Sub
// SDK の型に直接依存せず、本 interface を通してメッセージを受け取る
// (Clean Architecture の adapter 層ポート)。
//
// Consume は ctx がキャンセルされるまで handler をメッセージ毎に呼び出し、
// 戻り値で ack / nack 制御する。ctx キャンセルは正常終了扱いで nil を返す
// 契約 (ランナー側で「ctx キャンセル時は subscriber エラーを握りつぶす」
// 分岐を書かなくて済むよう、停止経路を明確化する目的)。
type MessageStream interface {
	Consume(ctx context.Context, handler MessageHandler) error
}
