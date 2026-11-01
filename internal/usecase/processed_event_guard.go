package usecase

import (
	"context"
	"fmt"

	"github.com/kenyamaneko/overload-party-account/internal/port"
)

// processedEventGuard は、同じイベントIDのイベントの反映を1回だけ行う。
type processedEventGuard struct {
	txRunner  port.TxRunner
	eventRepo port.ProcessedEventRepo
}

// applyOnce は、イベントIDが未処理のときだけ apply を呼ぶ。反映済みのイベントIDでは何もせず nil を返す。
// イベントIDの記録と apply の書き込みは1つのトランザクションで行い、apply が失敗したときはイベントIDも記録しない。
func (g *processedEventGuard) applyOnce(
	ctx context.Context,
	eventID, eventType string,
	apply func(txCtx context.Context) error,
) error {
	return g.txRunner.RunInTx(ctx, func(txCtx context.Context) error {
		isInserted, err := g.eventRepo.InsertProcessedEvent(txCtx, eventID, eventType)
		if err != nil {
			return fmt.Errorf("insert processed_events: %w", err)
		}
		if !isInserted {
			return nil
		}
		return apply(txCtx)
	})
}
