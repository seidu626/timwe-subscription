package graceful

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Operation func(ctx context.Context) error

// GracefulShutdown performs a graceful shutdown of a service
func GracefulShutdown(ctx context.Context, logger *zap.Logger, timeout time.Duration, operations map[string]Operation) {
	if len(operations) == 0 {
		return
	}

	wait := make(chan struct{})
	go func() {
		signalchan := make(chan os.Signal, 1)
		signal.Notify(signalchan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
		oscall := <-signalchan

		timeAfterExecuted := time.AfterFunc(timeout, func() {
			logger.Warn("Force shutdown")
			os.Exit(0)
		})
		defer timeAfterExecuted.Stop()

		wg := sync.WaitGroup{}
		wg.Add(len(operations))
		for k, op := range operations {
			go func(k string, op Operation) {
				defer wg.Done()
				logger.Warn(fmt.Sprintf("Shutdown %s", k))
				err := op(ctx)
				if err != nil {
					logger.Error("Error when stop server", zap.Error(err))
				}
			}(k, op)
		}
		wg.Wait()

		logger.Warn(fmt.Sprintf("system call:%+v", oscall))
		close(wait)
	}()
}
