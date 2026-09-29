package nutanix

import (
	"context"
	"time"
)

// cleanupTimeout bounds each step cleanup operation. Cleanup is detached from
// build cancellation so its delete requests are still sent after a cancel, so
// without this a stuck delete task would block packer from exiting. It applies
// on every path, not only after a cancel; image and VM delete tasks normally
// complete in seconds.
const cleanupTimeout = 10 * time.Minute

// sleepCtx waits for d, returning ctx.Err() early if ctx is cancelled first.
// Poll loops use it so a cancelled build stops promptly: with
// prism-go-client v0.8 every SDK call fails once ctx is cancelled, so a plain
// sleep would only burn the remaining retries.
func sleepCtx(ctx context.Context, d time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// withCleanupTimeout runs one cleanup operation under its own cleanupTimeout,
// so a slow operation cannot use up the budget of the ones after it.
func withCleanupTimeout(ctx context.Context, op func(context.Context) error) error {
	opCtx, cancel := context.WithTimeout(ctx, cleanupTimeout)
	defer cancel()
	return op(opCtx)
}
