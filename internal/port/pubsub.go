package port

import "context"

// MessageHandler は 1 メッセージの処理結果を成功/失敗の形で返す。
// nil = 成功 (正常処理 or 責務外として無視できるケース)、非 nil = 失敗
// (ペイロード不正や副作用失敗で再配信を要するケース)。
//
// 戻り値の意味論を「error か否か」に一本化することで、呼び出し側
// (push HTTP handler) は単一戻り値を見て応答コードを決められる。
type MessageHandler = func(ctx context.Context, data []byte) error
