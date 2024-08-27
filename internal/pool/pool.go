package pool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"

	"github.com/ydb-platform/ydb-go-sdk/v3/internal/stack"
	"github.com/ydb-platform/ydb-go-sdk/v3/internal/xcontext"
	"github.com/ydb-platform/ydb-go-sdk/v3/internal/xerrors"
	"github.com/ydb-platform/ydb-go-sdk/v3/internal/xlist"
	"github.com/ydb-platform/ydb-go-sdk/v3/internal/xsync"
	"github.com/ydb-platform/ydb-go-sdk/v3/retry"
)

type (
	Item[T any] interface {
		*T
		IsAlive() bool
		Close(ctx context.Context) error
	}
	Config[PT Item[T], T any] struct {
		trace         *Trace
		clock         clockwork.Clock
		limit         int
		createTimeout time.Duration
		createItem    func(ctx context.Context) (PT, error)
		closeTimeout  time.Duration
		closeItem     func(ctx context.Context, item PT)
		idleThreshold time.Duration
	}
	itemInfo[PT Item[T], T any] struct {
		idle    *xlist.Element[PT]
		touched time.Time
	}
	Pool[PT Item[T], T any] struct {
		config Config[PT, T]

		createItem func(ctx context.Context) (PT, error)
		closeItem  func(ctx context.Context, item PT)

		mu               xsync.RWMutex
		createInProgress int // KIKIMR-9163: in-create-process counter
		index            map[PT]itemInfo[PT, T]
		idle             xlist.List[PT]
		waitQ            xlist.List[*chan PT]
		waitChPool       xsync.Pool[chan PT]

		done chan struct{}
	}
	option[PT Item[T], T any] func(c *Config[PT, T])
)

func WithCreateItemFunc[PT Item[T], T any](f func(ctx context.Context) (PT, error)) option[PT, T] {
	return func(c *Config[PT, T]) {
		c.createItem = f
	}
}

func withCloseItemFunc[PT Item[T], T any](f func(ctx context.Context, item PT)) option[PT, T] {
	return func(c *Config[PT, T]) {
		c.closeItem = f
	}
}

func WithCreateItemTimeout[PT Item[T], T any](t time.Duration) option[PT, T] {
	return func(c *Config[PT, T]) {
		c.createTimeout = t
	}
}

func WithCloseItemTimeout[PT Item[T], T any](t time.Duration) option[PT, T] {
	return func(c *Config[PT, T]) {
		c.closeTimeout = t
	}
}

func WithLimit[PT Item[T], T any](size int) option[PT, T] {
	return func(c *Config[PT, T]) {
		c.limit = size
	}
}

func WithTrace[PT Item[T], T any](t *Trace) option[PT, T] {
	return func(c *Config[PT, T]) {
		c.trace = t
	}
}

func WithIdleThreshold[PT Item[T], T any](idleThreshold time.Duration) option[PT, T] {
	return func(c *Config[PT, T]) {
		c.idleThreshold = idleThreshold
	}
}

func WithClock[PT Item[T], T any](clock clockwork.Clock) option[PT, T] {
	return func(c *Config[PT, T]) {
		c.clock = clock
	}
}

func New[PT Item[T], T any](
	ctx context.Context,
	opts ...option[PT, T],
) *Pool[PT, T] {
	p := &Pool[PT, T]{
		config: Config[PT, T]{
			trace:         defaultTrace,
			clock:         clockwork.NewRealClock(),
			limit:         DefaultLimit,
			createItem:    defaultCreateItem[T, PT],
			createTimeout: defaultCreateTimeout,
			closeTimeout:  defaultCloseTimeout,
		},
		index: make(map[PT]itemInfo[PT, T]),
		idle:  xlist.New[PT](),
		waitQ: xlist.New[*chan PT](),
		waitChPool: xsync.Pool[chan PT]{
			New: func() *chan PT {
				ch := make(chan PT)

				return &ch
			},
		},
		done: make(chan struct{}),
	}

	for _, opt := range opts {
		if opt != nil {
			opt(&p.config)
		}
	}

	onDone := p.config.trace.OnNew(&NewStartInfo{
		Context: &ctx,
		Call:    stack.FunctionID("github.com/ydb-platform/ydb-go-sdk/v3/internal/pool.New"),
	})

	defer func() {
		onDone(&NewDoneInfo{
			Limit: p.config.limit,
		})
	}()

	p.createItem = makeAsyncCreateItemFunc(p)
	if p.config.closeItem != nil {
		p.closeItem = p.config.closeItem
	} else {
		p.closeItem = makeAsyncCloseItemFunc[PT, T](p)
	}

	return p
}

// defaultCreateItem returns a new item
func defaultCreateItem[T any, PT Item[T]](context.Context) (PT, error) {
	var item T

	return &item, nil
}

// makeAsyncCreateItemFunc wraps the createItem function with timeout handling
func makeAsyncCreateItemFunc[PT Item[T], T any]( //nolint:funlen
	p *Pool[PT, T],
) func(ctx context.Context) (PT, error) {
	return func(ctx context.Context) (PT, error) {
		if !xsync.WithLock(&p.mu, func() bool {
			if len(p.index)+p.createInProgress < p.config.limit {
				p.createInProgress++

				return true
			}

			return false
		}) {
			return nil, xerrors.WithStackTrace(errPoolIsOverflow)
		}
		defer func() {
			p.mu.WithLock(func() {
				p.createInProgress--
			})
		}()

		var (
			ch = make(chan struct {
				item PT
				err  error
			})
			done = make(chan struct{})
		)

		defer close(done)

		go func() {
			defer close(ch)

			createCtx, cancelCreate := xcontext.WithDone(xcontext.ValueOnly(ctx), p.done)
			defer cancelCreate()

			if d := p.config.createTimeout; d > 0 {
				createCtx, cancelCreate = xcontext.WithTimeout(createCtx, d)
				defer cancelCreate()
			}

			newItem, err := p.config.createItem(createCtx)
			if newItem != nil {
				p.mu.WithLock(func() {
					p.index[newItem] = itemInfo[PT, T]{
						touched: p.config.clock.Now(),
					}
				})
			}

			select {
			case ch <- struct {
				item PT
				err  error
			}{
				item: newItem,
				err:  xerrors.WithStackTrace(err),
			}:
			case <-done:
				if newItem == nil {
					return
				}

				_ = p.putItem(createCtx, newItem)
			}
		}()

		select {
		case <-p.done:
			return nil, xerrors.WithStackTrace(errClosedPool)
		case <-ctx.Done():
			return nil, xerrors.WithStackTrace(ctx.Err())
		case result, has := <-ch:
			if !has {
				return nil, xerrors.WithStackTrace(xerrors.Retryable(errNoProgress))
			}

			if result.err != nil {
				if xerrors.IsContextError(result.err) {
					return nil, xerrors.WithStackTrace(xerrors.Retryable(result.err))
				}

				return nil, xerrors.WithStackTrace(result.err)
			}

			return result.item, nil
		}
	}
}

func (p *Pool[PT, T]) onChangeStats() {
	p.mu.RLock()
	info := ChangeInfo{
		Limit: p.config.limit,
		Idle:  p.idle.Len(),
	}
	p.mu.RUnlock()
	p.config.trace.OnChange(info)
}

func (p *Pool[PT, T]) Stats() Stats {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return Stats{
		Limit:            p.config.limit,
		Index:            len(p.index),
		Idle:             p.idle.Len(),
		CreateInProgress: p.createInProgress,
	}
}

func makeAsyncCloseItemFunc[PT Item[T], T any](
	p *Pool[PT, T],
) func(ctx context.Context, item PT) {
	return func(ctx context.Context, item PT) {
		closeItemCtx, closeItemCancel := xcontext.WithDone(xcontext.ValueOnly(ctx), p.done)
		defer closeItemCancel()

		if d := p.config.closeTimeout; d > 0 {
			closeItemCtx, closeItemCancel = xcontext.WithTimeout(ctx, d)
			defer closeItemCancel()
		}

		go func() {
			_ = item.Close(closeItemCtx)
		}()
	}
}

func (p *Pool[PT, T]) try(ctx context.Context, f func(ctx context.Context, item PT) error) (finalErr error) {
	onDone := p.config.trace.OnTry(&TryStartInfo{
		Context: &ctx,
		Call:    stack.FunctionID("github.com/ydb-platform/ydb-go-sdk/v3/internal/pool.(*Pool).try"),
	})
	defer func() {
		onDone(&TryDoneInfo{
			Error: finalErr,
		})
	}()

	select {
	case <-p.done:
		return xerrors.WithStackTrace(errClosedPool)
	case <-ctx.Done():
		return xerrors.WithStackTrace(ctx.Err())
	default:
	}

	item, err := p.getItem(ctx)
	if err != nil {
		if xerrors.IsYdb(err) {
			return xerrors.WithStackTrace(xerrors.Retryable(err))
		}

		return xerrors.WithStackTrace(err)
	}

	defer func() {
		_ = p.putItem(ctx, item)
	}()

	err = f(ctx, item)
	if err != nil {
		return xerrors.WithStackTrace(err)
	}

	return nil
}

func (p *Pool[PT, T]) With(
	ctx context.Context,
	f func(ctx context.Context, item PT) error,
	opts ...retry.Option,
) (finalErr error) {
	var (
		onDone = p.config.trace.OnWith(&WithStartInfo{
			Context: &ctx,
			Call:    stack.FunctionID("github.com/ydb-platform/ydb-go-sdk/v3/internal/pool.(*Pool).With"),
		})
		attempts int
	)
	defer func() {
		onDone(&WithDoneInfo{
			Error:    finalErr,
			Attempts: attempts,
		})
	}()

	err := retry.Retry(ctx, func(ctx context.Context) error {
		attempts++
		err := p.try(ctx, f)
		if err != nil {
			return xerrors.WithStackTrace(err)
		}

		return nil
	}, opts...)
	if err != nil {
		return xerrors.WithStackTrace(fmt.Errorf("pool.With failed with %d attempts: %w", attempts, err))
	}

	return nil
}

func (p *Pool[PT, T]) Close(ctx context.Context) (finalErr error) {
	onDone := p.config.trace.OnClose(&CloseStartInfo{
		Context: &ctx,
		Call:    stack.FunctionID("github.com/ydb-platform/ydb-go-sdk/v3/internal/pool.(*Pool).Close"),
	})
	defer func() {
		onDone(&CloseDoneInfo{
			Error: finalErr,
		})
	}()

	select {
	case <-p.done:
		return xerrors.WithStackTrace(errClosedPool)

	default:
		close(p.done)

		p.mu.Lock()
		defer p.mu.Unlock()

		p.config.limit = 0

		for el := p.waitQ.Front(); el != nil; el = el.Next() {
			close(*el.Value)
		}

		var wg sync.WaitGroup
		wg.Add(p.idle.Len())

		for el := p.idle.Front(); el != nil; el = el.Next() {
			go func(item PT) {
				defer wg.Done()
				p.closeItem(ctx, item)
			}(el.Value)
			delete(p.index, el.Value)
		}

		wg.Wait()

		p.idle.Clear()

		return nil
	}
}

// getWaitCh returns pointer to a channel of sessions.
//
// Note that returning a pointer reduces allocations on sync.Pool usage –
// sync.Client.Get() returns empty interface, which leads to allocation for
// non-pointer values.
func (p *Pool[PT, T]) getWaitCh() *chan PT { //nolint:gocritic
	return p.waitChPool.GetOrNew()
}

// putWaitCh receives pointer to a channel and makes it available for further
// use.
// Note that ch MUST NOT be owned by any goroutine at the call moment and ch
// MUST NOT contain any value.
func (p *Pool[PT, T]) putWaitCh(ch *chan PT) { //nolint:gocritic
	p.waitChPool.Put(ch)
}

// p.mu must be held.
func (p *Pool[PT, T]) peekFirstIdle() (item PT, touched time.Time) {
	el := p.idle.Front()
	if el == nil {
		return
	}
	item = el.Value
	info, has := p.index[item]
	if !has || el != info.idle {
		panic(fmt.Sprintf("inconsistent index: (%v, %+v, %+v)", has, el, info.idle))
	}

	return item, info.touched
}

// removes first session from idle and resets the keepAliveCount
// to prevent session from dying in the internalPoolGC after it was returned
// to be used only in outgoing functions that make session busy.
// p.mu must be held.
func (p *Pool[PT, T]) removeFirstIdle() PT {
	idle, _ := p.peekFirstIdle()
	if idle != nil {
		info := p.removeIdle(idle)
		p.index[idle] = info
	}

	return idle
}

// p.mu must be held.
func (p *Pool[PT, T]) notifyAboutIdle(idle PT) (notified bool) {
	for el := p.waitQ.Front(); el != nil; el = p.waitQ.Front() {
		// Some goroutine is waiting for a session.
		//
		// It could be in this states:
		//   1) Reached the select code and awaiting for a value in channel.
		//   2) Reached the select code but already in branch of deadline
		//   cancellation. In this case it is locked on p.mu.Lock().
		//   3) Not reached the select code and thus not reading yet from the
		//   channel.
		//
		// For cases (2) and (3) we close the channel to signal that goroutine
		// missed something and may want to retry (especially for case (3)).
		//
		// After that we taking a next waiter and repeat the same.
		ch := p.waitQ.Remove(el)
		select {
		case *ch <- idle:
			// Case (1).
			return true

		case <-p.done:
			// Case (2) or (3).
			close(*ch)

		default:
			// Case (2) or (3).
			close(*ch)
		}
	}

	return false
}

// p.mu must be held.
func (p *Pool[PT, T]) removeIdle(item PT) itemInfo[PT, T] {
	info, has := p.index[item]
	if !has || info.idle == nil {
		panic("inconsistent session client index")
	}

	p.idle.Remove(info.idle)
	info.idle = nil
	p.index[item] = info

	return info
}

// p.mu must be held.
func (p *Pool[PT, T]) pushIdle(item PT, now time.Time) {
	info, has := p.index[item]
	if !has {
		panic("trying to store item created outside of the client")
	}
	if info.idle != nil {
		panic("inconsistent item client index")
	}

	info.touched = now
	info.idle = p.idle.PushBack(item)
	p.index[item] = info
}

const maxAttempts = 100

func (p *Pool[PT, T]) getItem(ctx context.Context) (PT, error) { //nolint:funlen
	var (
		start   = p.config.clock.Now()
		i       int
		lastErr error
	)

	for ; i < maxAttempts; i++ {
		select {
		case <-p.done:
			return nil, xerrors.WithStackTrace(errClosedPool)
		default:
		}

		if item := xsync.WithLock(&p.mu, func() PT {
			return p.removeFirstIdle()
		}); item != nil {
			if item.IsAlive() {
				info := xsync.WithLock(&p.mu, func() itemInfo[PT, T] {
					info, has := p.index[item]
					if !has {
						panic("no index for item")
					}

					return info
				})

				if p.config.idleThreshold > 0 && p.config.clock.Since(info.touched) > p.config.idleThreshold {
					p.closeItem(ctx, item)
					p.mu.WithLock(func() {
						delete(p.index, item)
					})

					continue
				}

				return item, nil
			}
		}

		item, createItemErr := p.createItem(ctx)
		if item != nil {
			return item, nil
		}

		if !isRetriable(createItemErr) {
			return nil, xerrors.WithStackTrace(createItemErr)
		}

		item, waitFromChErr := p.waitFromCh(ctx)
		if item != nil {
			return item, nil
		}

		if waitFromChErr != nil && !isRetriable(waitFromChErr) {
			return nil, xerrors.WithStackTrace(waitFromChErr)
		}

		lastErr = xerrors.WithStackTrace(xerrors.Join(createItemErr, waitFromChErr))
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	return nil, xerrors.WithStackTrace(
		fmt.Errorf("failed to get item from pool ("+
			"attempts: %d, latency: %v, pool has %d items (%d busy, %d idle, %d create_in_progress): %w",
			i, p.config.clock.Since(start), len(p.index), len(p.index)-p.idle.Len(), p.idle.Len(), p.createInProgress, lastErr,
		),
	)
}

//nolint:funlen
func (p *Pool[PT, T]) waitFromCh(ctx context.Context) (s PT, err error) {
	p.mu.Lock()
	ch := p.getWaitCh()
	el := p.waitQ.PushBack(ch)
	p.mu.Unlock()

	var deadliine <-chan time.Time
	if timeout := p.config.createTimeout; timeout > 0 {
		t := p.config.clock.NewTimer(timeout)
		defer t.Stop()

		deadliine = t.Chan()
	}

	select {
	case <-p.done:
		p.mu.WithLock(func() {
			p.waitQ.Remove(el)
		})

		return nil, xerrors.WithStackTrace(errClosedPool)

	case item, ok := <-*ch:
		// Note that race may occur and some goroutine may try to write
		// session into channel after it was enqueued but before it being
		// read here. In that case we will receive nil here and will retry.
		//
		// The same way will work when some session become deleted - the
		// nil value will be sent into the channel.
		if ok {
			// Put only filled and not closed channel back to the Client.
			// That is, we need to avoid races on filling reused channel
			// for the next waiter – session could be lost for a long time.
			p.putWaitCh(ch)
		}

		return item, nil

	case <-deadliine:
		p.mu.WithLock(func() {
			p.waitQ.Remove(el)
		})

		return nil, nil //nolint:nilnil

	case <-ctx.Done():
		p.mu.WithLock(func() {
			p.waitQ.Remove(el)
		})

		return nil, xerrors.WithStackTrace(ctx.Err())
	}
}

// p.mu must be free.
func (p *Pool[PT, T]) putItem(ctx context.Context, item PT) (err error) {
	select {
	case <-p.done:
		p.closeItem(ctx, item)
		p.mu.WithLock(func() {
			delete(p.index, item)
		})

		return xerrors.WithStackTrace(errClosedPool)
	default:
		p.mu.Lock()
		defer p.mu.Unlock()

		if !item.IsAlive() {
			p.closeItem(ctx, item)
			delete(p.index, item)

			return xerrors.WithStackTrace(errItemIsNotAlive)
		}

		if p.idle.Len() >= p.config.limit {
			p.closeItem(ctx, item)
			delete(p.index, item)

			return xerrors.WithStackTrace(errPoolIsOverflow)
		}

		if !p.notifyAboutIdle(item) {
			p.pushIdle(item, p.config.clock.Now())
		}

		return nil
	}
}
