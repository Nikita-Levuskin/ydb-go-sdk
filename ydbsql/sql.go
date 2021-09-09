package ydbsql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"

	"github.com/ydb-platform/ydb-go-sdk/v3"
	"github.com/ydb-platform/ydb-go-sdk/v3/table"
)

var (
	ErrDeprecated          = errors.New("ydbsql: deprecated")
	ErrUnsupported         = errors.New("ydbsql: not supported")
	ErrActiveTransaction   = errors.New("ydbsql: can not begin tx within active tx")
	ErrNoActiveTransaction = errors.New("ydbsql: no active tx to work with")
)

type ydbWrapper struct {
	dst *driver.Value
}

func (d *ydbWrapper) UnmarshalYDB(res ydb.RawScanner) error {
	if !res.NextItem() {
		err := res.Err()
		if err == nil {
			err = io.ErrUnexpectedEOF
		}
		return err
	}
	if res.IsOptional() {
		res.Unwrap()
	}
	if res.IsDecimal() {
		b, p, s := res.UnwrapDecimal()
		*d.dst = Decimal{
			Bytes:     b,
			Precision: p,
			Scale:     s,
		}
	} else {
		*d.dst = res.Any()
	}
	return res.Err()
}

// conn is a connection to the ydb.
type conn struct {
	connector *connector     // Immutable and r/o usage.
	session   *table.Session // Immutable and r/o usage.

	idle bool

	tx  *table.Transaction
	txc *table.TransactionControl
}

func (c *conn) takeSession(ctx context.Context) bool {
	if !c.idle {
		return true
	}
	if has, _ := c.pool().Take(ctx, c.session); !has {
		return false
	}
	c.idle = false
	return true
}

func (c *conn) ResetSession(ctx context.Context) error {
	if c.idle {
		return nil
	}
	err := c.pool().Put(ctx, c.session)
	if err != nil {
		return mapBadSessionError(err)
	}
	c.idle = true
	return nil
}

func (c *conn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	if !c.takeSession(ctx) {
		return nil, driver.ErrBadConn
	}
	s, err := c.session.Prepare(ctx, query)
	if err != nil {
		return nil, mapBadSessionError(err)
	}
	return &stmt{
		conn: c,
		stmt: s,
	}, nil
}

// txIsolationOrControl maps driver transaction options to ydb transaction option or query transaction control.
// This caused by ydb logic that prevents start actual transaction with OnlineReadOnly mode and ReadCommitted
// and ReadUncommitted isolation levels should use tx_control in every query request.
// It returns error on unsupported options.
func txIsolationOrControl(opts driver.TxOptions) (isolation table.TxOption, control []table.TxControlOption, err error) {
	level := sql.IsolationLevel(opts.Isolation)
	switch level {
	case sql.LevelDefault,
		sql.LevelSerializable,
		sql.LevelLinearizable:

		isolation = table.WithSerializableReadWrite()
		return

	case sql.LevelReadUncommitted:
		if opts.ReadOnly {
			control = []table.TxControlOption{
				table.BeginTx(
					table.WithOnlineReadOnly(
						table.WithInconsistentReads(),
					),
				),
				table.CommitTx(),
			}
			return
		}

	case sql.LevelReadCommitted:
		if opts.ReadOnly {
			control = []table.TxControlOption{
				table.BeginTx(
					table.WithOnlineReadOnly(),
				),
				table.CommitTx(),
			}
			return
		}
	}
	return nil, nil, fmt.Errorf(
		"ydbsql: unsupported transaction options: isolation=%s read_only=%t",
		nameIsolationLevel(level), opts.ReadOnly,
	)
}

func (c *conn) BeginTx(ctx context.Context, opts driver.TxOptions) (tx driver.Tx, err error) {
	if !c.takeSession(ctx) {
		return nil, driver.ErrBadConn
	}
	if c.tx != nil || c.txc != nil {
		return nil, ErrActiveTransaction
	}
	isolation, control, err := txIsolationOrControl(opts)
	if err != nil {
		return nil, err
	}
	if isolation != nil {
		c.tx, err = c.session.BeginTransaction(ctx, table.TxSettings(isolation))
		if err != nil {
			return nil, mapBadSessionError(err)
		}
		c.txc = table.TxControl(table.WithTx(c.tx))
	} else {
		c.txc = table.TxControl(control...)
	}
	return c, nil
}

// Rollback implements driver.Tx interface.
// Note that it is called by driver even if a user did not called it.
func (c *conn) Rollback() error {
	if c.tx == nil && c.txc == nil {
		return ErrNoActiveTransaction
	}

	tx := c.tx
	c.tx = nil
	c.txc = nil

	if tx != nil {
		return tx.Rollback(context.Background())
	}
	return nil
}

func (c *conn) Commit() (err error) {
	if c.tx == nil && c.txc == nil {
		return ErrNoActiveTransaction
	}

	tx := c.tx
	c.tx = nil
	c.txc = nil

	if tx != nil {
		_, err = tx.CommitTx(context.Background())
	}
	return
}

func (c *conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	_, err := c.exec(ctx, &reqQuery{text: query}, params(args))
	if err != nil {
		return nil, err
	}
	return result{}, nil
}

func (c *conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if ContextScanQueryMode(ctx) {
		// Allow to use scanQuery only through QueryContext API.
		return c.scanQueryContext(ctx, query, args)
	}
	return c.queryContext(ctx, query, args)
}

func (c *conn) queryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	res, err := c.exec(ctx, &reqQuery{text: query}, params(args))
	if err != nil {
		return nil, err
	}
	res.NextResultSet(ctx)
	return &rows{res: res}, nil
}

func (c *conn) scanQueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	res, err := c.exec(ctx, &reqScanQuery{text: query}, params(args))
	if err != nil {
		return nil, err
	}
	res.NextResultSet(ctx)
	return &stream{ctx: ctx, res: res}, res.Err()
}

func (c *conn) CheckNamedValue(v *driver.NamedValue) error {
	return checkNamedValue(v)
}

func (c *conn) Ping(ctx context.Context) error {
	if !c.takeSession(ctx) {
		return driver.ErrBadConn
	}
	_, err := c.session.KeepAlive(ctx)
	return mapBadSessionError(err)
}

func (c *conn) Close() error {
	ctx := context.Background()
	if !c.takeSession(ctx) {
		return driver.ErrBadConn
	}
	err := c.session.Close(ctx)
	return mapBadSessionError(err)
}

func (c *conn) Prepare(string) (driver.Stmt, error) {
	return nil, ErrDeprecated
}

func (c *conn) Begin() (driver.Tx, error) {
	return nil, ErrDeprecated
}

func (c *conn) exec(ctx context.Context, req processor, params *table.QueryParameters) (res *table.Result, err error) {
	if !c.takeSession(ctx) {
		return nil, driver.ErrBadConn
	}
	rc := c.retryConfig()
	retryNoIdempotent := ydb.ContextRetryNoIdempotent(ctx)
	maxRetries := rc.MaxRetries
	if c.tx != nil {
		// NOTE: when under transaction, no retries must be done.
		maxRetries = 0
	}
	var m ydb.RetryMode
	for i := 0; i <= maxRetries; i++ {
		if e := backoff(ctx, m, rc, i-1); e != nil {
			break
		}
		res, err = req.process(ctx, c, params)
		if err == nil {
			return res, nil
		}

		m = rc.RetryChecker.Check(err)

		if m.MustDeleteSession() {
			return nil, driver.ErrBadConn
		}
		if !m.MustRetry(retryNoIdempotent) {
			break
		}
	}
	return nil, err
}

func (c *conn) txControl() *table.TransactionControl {
	if c.txc == nil {
		return c.connector.defaultTxControl
	}
	return c.txc
}

func (c *conn) dataOpts() []table.ExecuteDataQueryOption {
	return c.connector.dataOpts
}

func (c *conn) scanOpts() []table.ExecuteScanQueryOption {
	return c.connector.scanOpts
}

func (c *conn) pool() *table.SessionPool {
	return &c.connector.pool
}

func (c *conn) retryConfig() *RetryConfig {
	return &c.connector.retryConfig
}

type processor interface {
	process(context.Context, *conn, *table.QueryParameters) (*table.Result, error)
}

type reqStmt struct {
	stmt *table.Statement
}

func (o *reqStmt) process(ctx context.Context, c *conn, params *table.QueryParameters) (*table.Result, error) {
	_, res, err := o.stmt.Execute(ctx, c.txControl(), params, c.dataOpts()...)
	return res, err
}

type reqQuery struct {
	text string
}

func (o *reqQuery) process(ctx context.Context, c *conn, params *table.QueryParameters) (*table.Result, error) {
	_, res, err := c.session.Execute(ctx, c.txControl(), o.text, params, c.dataOpts()...)
	return res, err
}

type reqScanQuery struct {
	text string
}

func (o *reqScanQuery) process(ctx context.Context, c *conn, params *table.QueryParameters) (*table.Result, error) {
	return c.session.StreamExecuteScanQuery(ctx, o.text, params, c.scanOpts()...)
}

type TxOperationFunc func(context.Context, *sql.Tx) error

// TxDoer contains options for retrying transactions.
type TxDoer struct {
	DB      *sql.DB
	Options *sql.TxOptions

	// RetryConfig allows to override retry parameters from DB.
	RetryConfig *RetryConfig
}

// Do starts a transaction and calls f with it. If f() call returns a retryable
// error, it repeats it accordingly to retry configuration that TxDoer's DB
// driver holds.
//
// Note that callers should mutate state outside of f carefully and keeping in
// mind that f could be called again even if no error returned – transaction
// commitment can be failed:
//
//   var results []int
//   ydbsql.DoTx(ctx, db, TxOperationFunc(func(ctx context.Context, tx *sql.Tx) error {
//       // Reset resulting slice to prevent duplicates when retry occurred.
//       results = results[:0]
//
//       rows, err := tx.QueryContext(...)
//       if err != nil {
//           // handle error
//       }
//       for rows.Next() {
//           results = append(results, ...)
//       }
//       return rows.Err()
//   }))
func (d TxDoer) Do(ctx context.Context, f TxOperationFunc) (err error) {
	rc := d.RetryConfig
	retryNoIdempotent := ydb.ContextRetryNoIdempotent(ctx)
	if rc == nil {
		rc = &d.DB.Driver().(*Driver).c.retryConfig
	}
	for i := 0; i <= rc.MaxRetries; i++ {
		if err = d.do(ctx, f); err == nil {
			return
		}
		if err == driver.ErrBadConn {
			// ErrBadConn returned by us to indicate that conn's underlying
			// session must be closed. Thus we could retry whole transaction.
			continue
		}
		m := rc.RetryChecker.Check(err)
		if !m.MustRetry(retryNoIdempotent) {
			return err
		}
		if e := backoff(ctx, m, rc, i); e != nil {
			break
		}
	}
	return
}

func (d TxDoer) do(ctx context.Context, f TxOperationFunc) error {
	tx, err := d.DB.BeginTx(ctx, d.Options)
	if err != nil {
		return err
	}
	defer func() {
		_ = tx.Rollback()
	}()
	if err := f(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

// DoTx is a shortcut for calling Do(ctx, f) on initialized TxDoer with DB
// field set to given db.
func DoTx(ctx context.Context, db *sql.DB, f TxOperationFunc) error {
	return (TxDoer{DB: db}).Do(ctx, f)
}

var isolationLevelName = [...]string{
	sql.LevelDefault:         "default",
	sql.LevelReadUncommitted: "read_uncommitted",
	sql.LevelReadCommitted:   "read_committed",
	sql.LevelWriteCommitted:  "write_committed",
	sql.LevelRepeatableRead:  "repeatable_read",
	sql.LevelSnapshot:        "snapshot",
	sql.LevelSerializable:    "serializable",
	sql.LevelLinearizable:    "linearizable",
}

func nameIsolationLevel(x sql.IsolationLevel) string {
	if int(x) < len(isolationLevelName) {
		return isolationLevelName[x]
	}
	return "unknown_isolation"
}

type stmt struct {
	conn *conn
	stmt *table.Statement
}

func (s *stmt) NumInput() int {
	return s.stmt.NumInput()
}

func (s *stmt) Close() error {
	return nil
}

func (s stmt) Exec([]driver.Value) (driver.Result, error) {
	return nil, ErrDeprecated
}

func (s stmt) Query([]driver.Value) (driver.Rows, error) {
	return nil, ErrDeprecated
}

func (s *stmt) CheckNamedValue(v *driver.NamedValue) error {
	return checkNamedValue(v)
}

func (s *stmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	_, err := s.conn.exec(ctx, &reqStmt{stmt: s.stmt}, params(args))
	if err != nil {
		return nil, err
	}
	return result{}, nil
}

func (s *stmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	if ContextScanQueryMode(ctx) {
		// Allow to use scanQuery only through QueryContext API.
		return s.scanQueryContext(ctx, args)
	}
	return s.queryContext(ctx, args)
}

func (s *stmt) queryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	res, err := s.conn.exec(ctx, &reqStmt{stmt: s.stmt}, params(args))
	if err != nil {
		return nil, err
	}
	res.NextResultSet(ctx)
	return &rows{res: res}, nil
}

func (s *stmt) scanQueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	res, err := s.conn.exec(ctx, &reqScanQuery{text: s.stmt.Text()}, params(args))
	if err != nil {
		return nil, err
	}
	res.NextResultSet(ctx)
	return &stream{ctx: ctx, res: res}, res.Err()
}

func checkNamedValue(v *driver.NamedValue) (err error) {
	if v.Name == "" {
		return fmt.Errorf("ydbsql: only named parameters are supported")
	}

	if valuer, ok := v.Value.(driver.Valuer); ok {
		v.Value, err = valuer.Value()
		if err != nil {
			return fmt.Errorf("ydbsql: driver.Valuer error: %w", err)
		}
	}

	switch x := v.Value.(type) {
	case ydb.Value:
		// OK.

	case valuer:
		// Some ydbsql level types implement valuer interface.
		// Currently it is a date/time types.
		v.Value = x.Value()

	case bool:
		v.Value = ydb.BoolValue(x)
	case int8:
		v.Value = ydb.Int8Value(x)
	case uint8:
		v.Value = ydb.Uint8Value(x)
	case int16:
		v.Value = ydb.Int16Value(x)
	case uint16:
		v.Value = ydb.Uint16Value(x)
	case int32:
		v.Value = ydb.Int32Value(x)
	case uint32:
		v.Value = ydb.Uint32Value(x)
	case int64:
		v.Value = ydb.Int64Value(x)
	case uint64:
		v.Value = ydb.Uint64Value(x)
	case float32:
		v.Value = ydb.FloatValue(x)
	case float64:
		v.Value = ydb.DoubleValue(x)
	case []byte:
		v.Value = ydb.StringValue(x)
	case string:
		v.Value = ydb.UTF8Value(x)
	case [16]byte:
		v.Value = ydb.UUIDValue(x)

	default:
		return fmt.Errorf("ydbsql: unsupported type: %T", x)
	}

	v.Name = "$" + v.Name

	return nil
}

func params(args []driver.NamedValue) *table.QueryParameters {
	if len(args) == 0 {
		return nil
	}
	opts := make([]table.ParameterOption, len(args))
	for i, arg := range args {
		opts[i] = table.ValueParam(
			arg.Name,
			arg.Value.(ydb.Value),
		)
	}
	return table.NewQueryParameters(opts...)
}

type rows struct {
	res *table.Result
}

func (r *rows) Columns() []string {
	var i int
	cs := make([]string, r.res.ColumnCount())
	r.res.Columns(func(m table.Column) {
		cs[i] = m.Name
		i++
	})
	return cs
}

func (r *rows) NextResultSet() error {
	if !r.res.NextResultSet(context.Background()) {
		return io.EOF
	}
	return nil
}

func (r *rows) HasNextResultSet() bool {
	return r.res.HasNextResultSet()
}

func (r *rows) Next(dst []driver.Value) error {
	if !r.res.NextRow() {
		return io.EOF
	}
	for i := range dst {
		// NOTE: for queries like "SELECT * FROM xxx" order of columns is
		// undefined.
		_ = r.res.Scan(&ydbWrapper{&dst[i]})
	}
	return r.res.Err()
}

func (r *rows) Close() error {
	return r.res.Close()
}

type stream struct {
	res *table.Result
	ctx context.Context
}

func (r *stream) Columns() []string {
	var i int
	cs := make([]string, r.res.ColumnCount())
	r.res.Columns(func(m table.Column) {
		cs[i] = m.Name
		i++
	})
	return cs
}

func (r *stream) Next(dst []driver.Value) error {
	if !r.res.HasNextRow() {
		if !r.res.NextResultSet(r.ctx) {
			err := r.res.Err()
			if err != nil {
				return err
			}
			return io.EOF
		}
	}
	if !r.res.NextRow() {
		return io.EOF
	}
	for i := range dst {
		// NOTE: for queries like "SELECT * FROM xxx" order of columns is
		// undefined.
		_ = r.res.Scan(&ydbWrapper{&dst[i]})
	}
	return r.res.Err()
}

func (r *stream) Close() error {
	return r.res.Close()
}

type result struct{}

func (r result) LastInsertId() (int64, error) { return 0, ErrUnsupported }
func (r result) RowsAffected() (int64, error) { return 0, ErrUnsupported }

func mapBadSessionError(err error) error {
	if ydb.IsOpError(err, ydb.StatusBadSession) {
		return driver.ErrBadConn
	}
	return err
}
