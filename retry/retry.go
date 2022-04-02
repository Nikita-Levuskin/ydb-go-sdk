package retry

import (
	"context"
	"math"
	"time"

	"github.com/ydb-platform/ydb-go-sdk/v3/internal/errors"
	"github.com/ydb-platform/ydb-go-sdk/v3/internal/rand"
	"github.com/ydb-platform/ydb-go-sdk/v3/trace"
)

// Default parameters used by Retry() functions within different sub packages.
const (
	fastSlot = 5 * time.Millisecond
	slowSlot = 1 * time.Second
)

// Default parameters used by Retry() functions within different sub packages.
var (
	FastBackoff = newBackoff(
		withSlotDuration(fastSlot),
		withCeiling(6),
	)
	SlowBackoff = newBackoff(
		withSlotDuration(slowSlot),
		withCeiling(6),
	)
)

// retryOperation is the interface that holds an operation for retry.
// if retryOperation returns not nil - operation will retry
// if retryOperation returns nil - retry loop will break
type retryOperation func(context.Context) (err error)

type retryableErrorOption errors.RetryableErrorOption

const (
	BackoffTypeNoBackoff   = errors.BackoffTypeNoBackoff
	BackoffTypeFastBackoff = errors.BackoffTypeFastBackoff
	BackoffTypeSlowBackoff = errors.BackoffTypeSlowBackoff
)

func WithBackoff(t errors.BackoffType) retryableErrorOption {
	return retryableErrorOption(errors.WithBackoff(t))
}

func WithDeleteSession() retryableErrorOption {
	return retryableErrorOption(errors.WithDeleteSession())
}

func RetryableError(err error, opts ...retryableErrorOption) error {
	return errors.RetryableError(
		err,
		func() (retryableErrorOptions []errors.RetryableErrorOption) {
			for _, o := range opts {
				retryableErrorOptions = append(retryableErrorOptions, errors.RetryableErrorOption(o))
			}
			return retryableErrorOptions
		}()...,
	)
}

type retryOptionsHolder struct {
	id          string
	trace       trace.Retry
	idempotent  bool
	fastBackoff Backoff
	slowBackoff Backoff
}

type retryOption func(h *retryOptionsHolder)

// WithID returns id option
func WithID(id string) retryOption {
	return func(h *retryOptionsHolder) {
		h.id = id
	}
}

// WithTrace returns trace option
func WithTrace(trace trace.Retry) retryOption {
	return func(h *retryOptionsHolder) {
		h.trace = trace
	}
}

// WithIdempotent returns idempotent trace option
func WithIdempotent() retryOption {
	return func(h *retryOptionsHolder) {
		h.idempotent = true
	}
}

// WithFastBackoff returns fast backoff trace option
func WithFastBackoff(b Backoff) retryOption {
	return func(h *retryOptionsHolder) {
		h.fastBackoff = b
	}
}

// WithSlowBackoff returns fast backoff trace option
func WithSlowBackoff(b Backoff) retryOption {
	return func(h *retryOptionsHolder) {
		h.slowBackoff = b
	}
}

// Retry provide the best effort fo retrying operation
// Retry implements internal busy loop until one of the following conditions is met:
// - deadline was canceled or deadlined
// - retry operation returned nil as error
// Warning: if deadline without deadline or cancellation func Retry will be worked infinite
// If you need to retry your op func on some logic errors - you must return RetryableError() from retryOperation
func Retry(ctx context.Context, op retryOperation, opts ...retryOption) (err error) {
	h := &retryOptionsHolder{
		fastBackoff: FastBackoff,
		slowBackoff: SlowBackoff,
	}
	for _, o := range opts {
		o(h)
	}
	var (
		i        int
		attempts int

		code           = int64(0)
		onIntermediate = trace.RetryOnRetry(h.trace, &ctx, h.id, h.idempotent)
	)
	defer func() {
		onIntermediate(err)(attempts, err)
	}()
	for {
		i++
		attempts++
		select {
		case <-ctx.Done():
			return errors.WithStackTrace(ctx.Err())

		default:
			err = op(ctx)

			if err == nil {
				return
			}

			m := Check(err)

			if m.StatusCode() != code {
				i = 0
			}

			if !m.MustRetry(h.idempotent) {
				return errors.WithStackTrace(err)
			}

			if e := Wait(ctx, h.fastBackoff, h.slowBackoff, m, i); e != nil {
				return errors.WithStackTrace(err)
			}

			code = m.StatusCode()

			onIntermediate(err)
		}
	}
}

// Check returns retry mode for err.
func Check(err error) (m retryMode) {
	statusCode, operationStatus, backoff, deleteSession := errors.Check(err)
	return retryMode{
		statusCode:      statusCode,
		operationStatus: operationStatus,
		backoff:         backoff,
		deleteSession:   deleteSession,
	}
}

func Wait(ctx context.Context, fastBackoff Backoff, slowBackoff Backoff, m retryMode, i int) error {
	var b Backoff
	switch m.BackoffType() {
	case errors.BackoffTypeNoBackoff:
		return nil
	case errors.BackoffTypeFastBackoff:
		b = fastBackoff
	case errors.BackoffTypeSlowBackoff:
		b = slowBackoff
	}
	return waitBackoff(ctx, b, i)
}

// logBackoff contains logarithmic Backoff policy.
type logBackoff struct {
	// SlotDuration is a size of a single time slot used in Backoff delay
	// calculation.
	// If SlotDuration is less or equal to zero, then the time.Second value is
	// used.
	SlotDuration time.Duration

	// Ceiling is a maximum degree of Backoff delay growth.
	// If Ceiling is less or equal to zero, then the default ceiling of 1 is
	// used.
	Ceiling uint

	// JitterLimit controls fixed and random portions of Backoff delay.
	// Its value can be in range [0, 1].
	// If JitterLimit is non zero, then the Backoff delay will be equal to (F + R),
	// where F is a result of multiplication of this value and calculated delay
	// duration D; and R is a random sized part from [0,(D - F)].
	JitterLimit float64

	// generator of jitter
	r rand.Rand
}

type option func(b *logBackoff)

func withSlotDuration(slotDuration time.Duration) option {
	return func(b *logBackoff) {
		b.SlotDuration = slotDuration
	}
}

func withCeiling(ceiling uint) option {
	return func(b *logBackoff) {
		b.Ceiling = ceiling
	}
}

func withJitterLimit(jitterLimit float64) option {
	return func(b *logBackoff) {
		b.JitterLimit = jitterLimit
	}
}

func newBackoff(opts ...option) logBackoff {
	b := logBackoff{
		r: rand.New(rand.WithLock()),
	}
	for _, o := range opts {
		o(&b)
	}
	return b
}

// Wait implements Backoff interface.
func (b logBackoff) Wait(n int) <-chan time.Time {
	return time.After(b.delay(n))
}

// delay returns mapping of i to delay.
func (b logBackoff) delay(i int) time.Duration {
	s := b.SlotDuration
	if s <= 0 {
		s = time.Second
	}
	n := 1 << min(uint(i), max(1, b.Ceiling))
	d := s * time.Duration(n)
	f := time.Duration(math.Min(1, math.Abs(b.JitterLimit)) * float64(d))
	if f == d {
		return f
	}
	return f + time.Duration(b.r.Int64(int64(d-f)+1))
}

func min(a, b uint) uint {
	if a < b {
		return a
	}
	return b
}

func max(a, b uint) uint {
	if a > b {
		return a
	}
	return b
}

// retryMode reports whether operation is able retried and with which properties.
type retryMode struct {
	statusCode      int64
	operationStatus errors.OperationStatus
	backoff         errors.BackoffType
	deleteSession   bool
}

func (m retryMode) MustRetry(isOperationIdempotent bool) bool {
	switch m.operationStatus {
	case errors.OperationFinished:
		return false
	case errors.OperationStatusUndefined:
		return isOperationIdempotent
	default:
		return true
	}
}

func (m retryMode) StatusCode() int64 { return m.statusCode }

func (m retryMode) MustBackoff() bool { return m.backoff&errors.BackoffTypeBackoffAny != 0 }

func (m retryMode) BackoffType() errors.BackoffType { return m.backoff }

func (m retryMode) MustDeleteSession() bool { return m.deleteSession }

// Backoff is the interface that contains logic of delaying operation retry.
type Backoff interface {
	// Wait maps index of the retry to a channel which fulfillment means that
	// delay is over.
	//
	// Note that retry index begins from 0 and 0-th index means that it is the
	// first retry attempt after an initial error.
	Wait(n int) <-chan time.Time
}

// waitBackoff is a helper function that waits for i-th Backoff b or ctx
// expiration.
// It returns non-nil error if and only if deadline expiration branch wins.
func waitBackoff(ctx context.Context, b Backoff, i int) error {
	if b == nil {
		if err := ctx.Err(); err != nil {
			return errors.WithStackTrace(err)
		}
		return nil
	}
	select {
	case <-b.Wait(i):
		return nil
	case <-ctx.Done():
		if err := ctx.Err(); err != nil {
			return errors.WithStackTrace(err)
		}
		return nil
	}
}
