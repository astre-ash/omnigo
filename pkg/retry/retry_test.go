package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultDelays(t *testing.T) {
	delays := DefaultDelays()
	assert.Equal(t, []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}, delays)

	delays[0] = 99 * time.Second
	assert.Equal(t, []time.Duration{1 * time.Second, 3 * time.Second, 5 * time.Second}, DefaultDelays())
}

func TestDo_NilOperation(t *testing.T) {
	err := Do(context.Background(), DefaultDelays(), nil, nil)
	assert.ErrorIs(t, err, ErrNilOperation)
}

func TestDo_SuccessOnFirstAttempt(t *testing.T) {
	ctx := context.Background()
	opCalls := 0
	isRetryableCalls := 0

	err := Do(
		ctx,
		[]time.Duration{10 * time.Millisecond, 20 * time.Millisecond},
		func(err error) bool {
			isRetryableCalls++
			return true
		},
		func() error {
			opCalls++
			return nil
		},
	)

	require.NoError(t, err)
	assert.Equal(t, 1, opCalls)
	assert.Equal(t, 0, isRetryableCalls, "isRetryable must not be called when op succeeds")
}

func TestDo_RetriesUntilSuccess(t *testing.T) {
	ctx := context.Background()
	errTemp := errors.New("temporary error")

	opCalls := 0
	isRetryableCalls := 0

	err := Do(
		ctx,
		[]time.Duration{5 * time.Millisecond, 5 * time.Millisecond, 5 * time.Millisecond},
		func(err error) bool {
			isRetryableCalls++
			return errors.Is(err, errTemp)
		},
		func() error {
			opCalls++
			if opCalls < 3 {
				return errTemp
			}
			return nil
		},
	)

	require.NoError(t, err)
	assert.Equal(t, 3, opCalls)
	assert.Equal(t, 2, isRetryableCalls)
}

func TestDo_ExhaustsAllDelaysAndReturnsLastError(t *testing.T) {
	ctx := context.Background()
	delays := []time.Duration{10 * time.Millisecond, 15 * time.Millisecond, 20 * time.Millisecond}

	opCalls := 0
	isRetryableCalls := 0

	start := time.Now()
	err := Do(
		ctx,
		delays,
		func(err error) bool {
			isRetryableCalls++
			return true
		},
		func() error {
			opCalls++
			return fmt.Errorf("attempt %d failed", opCalls)
		},
	)
	elapsed := time.Since(start)

	require.Error(t, err)
	assert.Equal(t, "attempt 4 failed", err.Error())
	assert.Equal(t, len(delays)+1, opCalls)
	assert.Equal(t, len(delays), isRetryableCalls)
	assert.GreaterOrEqual(t, elapsed, 45*time.Millisecond)
}

func TestDo_NonRetryableAndNilPredicate(t *testing.T) {
	ctx := context.Background()

	t.Run("stops on first attempt without waiting for delay", func(t *testing.T) {
		errFatal := errors.New("fatal error")
		opCalls := 0
		isRetryableCalls := 0

		start := time.Now()
		err := Do(
			ctx,
			[]time.Duration{5 * time.Second},
			func(err error) bool {
				isRetryableCalls++
				return false
			},
			func() error {
				opCalls++
				return errFatal
			},
		)
		elapsed := time.Since(start)

		require.ErrorIs(t, err, errFatal)
		assert.Equal(t, 1, opCalls)
		assert.Equal(t, 1, isRetryableCalls)
		assert.Less(t, elapsed, 200*time.Millisecond)
	})

	t.Run("stops on subsequent attempt when error becomes non-retryable", func(t *testing.T) {
		errRetryable := errors.New("retryable")
		errFatal := errors.New("fatal")
		opCalls := 0

		err := Do(
			ctx,
			[]time.Duration{5 * time.Millisecond, 5 * time.Second},
			func(err error) bool {
				return errors.Is(err, errRetryable)
			},
			func() error {
				opCalls++
				if opCalls == 1 {
					return errRetryable
				}
				return errFatal
			},
		)

		require.ErrorIs(t, err, errFatal)
		assert.Equal(t, 2, opCalls)
	})

	t.Run("nil isRetryable with non-empty delays does not panic and does not retry", func(t *testing.T) {
		errFail := errors.New("failed")
		opCalls := 0

		err := Do(ctx, []time.Duration{5 * time.Second}, nil, func() error {
			opCalls++
			return errFail
		})

		require.ErrorIs(t, err, errFail)
		assert.Equal(t, 1, opCalls)
	})
}

func TestDo_EmptyZeroAndNegativeDelays(t *testing.T) {
	ctx := context.Background()
	errFail := errors.New("failed")

	for _, tc := range []struct {
		name          string
		delays        []time.Duration
		expectedCalls int
	}{
		{name: "nil delays", delays: nil, expectedCalls: 1},
		{name: "empty delays", delays: []time.Duration{}, expectedCalls: 1},
		{name: "zero delays", delays: []time.Duration{0, 0}, expectedCalls: 3},
		{name: "negative delays", delays: []time.Duration{-time.Second, -time.Millisecond}, expectedCalls: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opCalls := 0

			err := Do(ctx, tc.delays, func(err error) bool { return true }, func() error {
				opCalls++
				return errFail
			})

			require.ErrorIs(t, err, errFail)
			assert.Equal(t, tc.expectedCalls, opCalls)
		})
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	t.Run("already canceled context does not execute op", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		opCalls := 0
		err := Do(
			ctx,
			[]time.Duration{5 * time.Second},
			func(err error) bool { return true },
			func() error {
				opCalls++
				return nil
			},
		)

		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Contains(t, err.Error(), "operation cancelled before attempt 1")
		assert.Equal(t, 0, opCalls)
	})

	t.Run("context canceled during op stops before next attempt even with zero delay", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		opCalls := 0

		err := Do(
			ctx,
			[]time.Duration{0, 0},
			func(err error) bool { return true },
			func() error {
				opCalls++
				cancel()
				return errors.New("temporary error")
			},
		)

		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Contains(t, err.Error(), "operation cancelled before attempt 2")
		assert.Equal(t, 1, opCalls)
	})

	t.Run("context canceled explicitly during backoff", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		opCalls := 0

		go func() {
			time.Sleep(15 * time.Millisecond)
			cancel()
		}()

		start := time.Now()
		err := Do(
			ctx,
			[]time.Duration{5 * time.Second},
			func(err error) bool { return true },
			func() error {
				opCalls++
				return errors.New("temporary error")
			},
		)
		elapsed := time.Since(start)

		require.Error(t, err)
		assert.ErrorIs(t, err, context.Canceled)
		assert.Contains(t, err.Error(), "operation cancelled during retry backoff")
		assert.Equal(t, 1, opCalls)
		assert.Less(t, elapsed, 500*time.Millisecond)
	})

	t.Run("context deadline exceeded during backoff", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		opCalls := 0
		start := time.Now()
		err := Do(
			ctx,
			[]time.Duration{5 * time.Second},
			func(err error) bool { return true },
			func() error {
				opCalls++
				return errors.New("temporary error")
			},
		)
		elapsed := time.Since(start)

		require.Error(t, err)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Contains(t, err.Error(), "operation cancelled during retry backoff")
		assert.Equal(t, 1, opCalls)
		assert.Less(t, elapsed, 500*time.Millisecond)
	})
}
