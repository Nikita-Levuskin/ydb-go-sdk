package table

import (
	"context"

	"github.com/ydb-platform/ydb-go-sdk/v3/table/options"
)

// Operation is the interface that holds an operation for retry.
type Operation func(context.Context, Session) error

// TxOperation is the interface that holds an operation for retry.
type TxOperation func(context.Context, Transaction) error

type Option func(o *Options)

type Options struct {
	Idempotent      bool
	TxSettings      *TransactionSettings
	TxCommitOptions []options.CommitTransactionOption
}

func WithIdempotent() Option {
	return func(o *Options) {
		o.Idempotent = true
	}
}

func WithTxSettings(tx *TransactionSettings) Option {
	return func(o *Options) {
		o.TxSettings = tx
	}
}

func WithTxCommitOptions(opts ...options.CommitTransactionOption) Option {
	return func(o *Options) {
		o.TxCommitOptions = append(o.TxCommitOptions, opts...)
	}
}

type ctxIdempotentOperationKey struct{}

func WithIdempotentOperation(ctx context.Context) context.Context {
	return context.WithValue(ctx, ctxIdempotentOperationKey{}, struct{}{})
}

func ContextIdempotentOperation(ctx context.Context) bool {
	return ctx.Value(ctxIdempotentOperationKey{}) != nil
}

type ctxTransactionSettingsKey struct{}

func WithTransactionSettings(ctx context.Context, tx *TransactionSettings) context.Context {
	return context.WithValue(ctx, ctxTransactionSettingsKey{}, tx)
}

func ContextTransactionSettings(ctx context.Context) *TransactionSettings {
	if tx, ok := ctx.Value(ctxTransactionSettingsKey{}).(*TransactionSettings); ok {
		return tx
	}
	return nil
}

type ClosableSession interface {
	Session

	Close(ctx context.Context) (err error)
}

type Client interface {
	// Close closes table client
	Close(ctx context.Context) error

	// CreateSession returns session or error for manually control of session lifecycle
	// CreateSession do not provide retry loop for failed create session requests.
	// Best effort policy may be implements with outer retry loop includes CreateSession call
	CreateSession(ctx context.Context) (s ClosableSession, err error)

	// Do provide the best effort for execute operation
	// Do implements internal busy loop until one of the following conditions is met:
	// - deadline was canceled or deadlined
	// - retry operation returned nil as error
	// Warning: if deadline without deadline or cancellation func Retry will be worked infinite
	Do(ctx context.Context, op Operation, opts ...Option) error

	// DoTx provide the best effort for execute operation
	// DoTx implements internal busy loop until one of the following conditions is met:
	// - deadline was canceled or deadlined
	// - retry operation returned nil as error
	// Warning: if deadline without deadline or cancellation func Retry will be worked infinite
	DoTx(ctx context.Context, op TxOperation, opts ...Option) error
}