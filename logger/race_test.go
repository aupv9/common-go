package logger_test

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"

	"github.com/aupv9/common-go/logger"
)

// TestRegisterExtractorConcurrent đảm bảo đăng ký extractor đồng thời với logging
// không gây data race (chạy với -race để phát hiện).
func TestRegisterExtractorConcurrent(t *testing.T) {
	log := logger.New(logger.Config{Level: "debug", Format: "json", Writer: io.Discard})
	ctx := context.Background()

	var wg sync.WaitGroup
	stop := make(chan struct{})

	// Goroutine ghi log liên tục.
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					log.InfoContext(ctx, "tick")
				}
			}
		}()
	}

	// Đồng thời đăng ký extractor.
	for i := 0; i < 50; i++ {
		logger.RegisterContextExtractor(func(context.Context) []slog.Attr {
			return []slog.Attr{slog.String("x", "y")}
		})
	}

	close(stop)
	wg.Wait()
}
