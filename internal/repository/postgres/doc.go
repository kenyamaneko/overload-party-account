// Package postgres は PostgreSQL による account サービスのデータアクセス実装を提供する。
//
// トランザクション方針: 全 repository メソッドは context 経由のトランザクション
// (TxManager.RunInTx が設定) に参加する。connFrom(ctx, pool) で透過的に Tx か Pool を選ぶ。
package postgres
