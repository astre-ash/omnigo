package retry

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrNilOperation = errors.New("operation must not be nil")

type IsRetryableFunc func(err error) bool

func DefaultDelays() []time.Duration {
	return []time.Duration{
		1 * time.Second,
		3 * time.Second,
		5 * time.Second,
	}
}

func Do(ctx context.Context, delays []time.Duration, isRetryable IsRetryableFunc, op func() error) error {
	if op == nil {
		return ErrNilOperation
	}

	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("operation cancelled before attempt %d: %w", attempt+1, err)
		}

		err := op()
		if err == nil {
			return nil
		}

		if attempt >= len(delays) || isRetryable == nil || !isRetryable(err) {
			return err
		}

		if delay := delays[attempt]; delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return fmt.Errorf("operation cancelled during retry backoff: %w", ctx.Err())
			case <-timer.C:
			}
		}
	}
}
