// Copyright 2015-2017, Cyrill @ Schumacher.fm and the CoreStore contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package dbr

import (
	"context"
	"database/sql"

	"github.com/corestoreio/csfw/util/bufferpool"
	"github.com/corestoreio/errors"
	"github.com/corestoreio/log"
)

// Update contains the clauses for an UPDATE statement
type Update struct {
	Log log.Logger
	DB  Execer

	// TODO: add UPDATE JOINS SQLStmtUpdateJoin

	RawFullSQL   string
	RawArguments Arguments // Arguments used by RawFullSQL or BuildCache
	buildCache   []byte

	Table alias
	// SetClauses contains the column/argument association. For each column
	// there must be one argument.
	SetClauses     UpdatedColumns
	WhereFragments WhereFragments
	OrderBys       aliases
	LimitCount     uint64
	OffsetCount    uint64
	LimitValid     bool
	OffsetValid    bool
	// PropagationStopped set to true if you would like to interrupt the
	// listener chain. Once set to true all sub sequent calls of the next
	// listeners will be suppressed.
	PropagationStopped bool
	IsInterpolate      bool // See Interpolate()
	// UseBuildCache if set to true the final build query will be stored in
	// field private field `buildCache` and the arguments in field `Arguments`
	UseBuildCache bool
	// Listeners allows to dispatch certain functions in different
	// situations.
	Listeners UpdateListeners
	// propagationStoppedAt position in the slice where the stopped propagation
	// has been requested. for every new iteration the propagation must stop at
	// this position.
	propagationStoppedAt int
	// previousError any error occurred during construction the SQL statement
	previousError error
}

// NewUpdate creates a new object with a black hole logger.
func NewUpdate(table ...string) *Update {
	return &Update{
		Table: MakeAlias(table...),
	}
}

// Update creates a new Update for the given table
func (c *Connection) Update(table ...string) *Update {
	u := &Update{
		Log:   c.Log,
		Table: MakeAlias(table...),
	}
	u.DB = c.DB
	return u
}

// UpdateBySQL creates a new Update for the given SQL string and arguments
func (c *Connection) UpdateBySQL(sql string, args ...Argument) *Update {
	u := &Update{
		Log:          c.Log,
		RawFullSQL:   sql,
		RawArguments: args,
	}
	u.DB = c.DB
	return u
}

// Update creates a new Update for the given table bound to a transaction
func (tx *Tx) Update(table ...string) *Update {
	u := &Update{
		Log:   tx.Logger,
		Table: MakeAlias(table...),
	}
	u.DB = tx.Tx
	return u
}

// UpdateBySQL creates a new Update for the given SQL string and arguments bound
// to a transaction
func (tx *Tx) UpdateBySQL(sql string, args ...Argument) *Update {
	u := &Update{
		Log:          tx.Logger,
		RawFullSQL:   sql,
		RawArguments: args,
	}
	u.DB = tx.Tx
	return u
}

// WithDB sets the database query object.
func (b *Update) WithDB(db Execer) *Update {
	b.DB = db
	return b
}

// Set appends a column/value pair for the statement.
func (b *Update) Set(column string, arg Argument) *Update {
	if b.previousError != nil {
		return b
	}
	b.SetClauses.Columns = append(b.SetClauses.Columns, column)
	b.SetClauses.Arguments = append(b.SetClauses.Arguments, arg)
	return b
}

// AddColumns adds columns which values gets later derived from an
// ArgumentAssembler. Those columns will get passed to the ArgumentAssembler
// implementation. Mostly used with the type UpdateMulti.
func (b *Update) AddColumns(columnNames ...string) *Update {
	if b.previousError != nil {
		return b
	}
	b.SetClauses.Columns = append(b.SetClauses.Columns, columnNames...)
	return b
}

// SetRecord sets a new argument generator type. See the example for more
// details.
func (b *Update) SetRecord(columns []string, rec ArgumentAssembler) *Update {
	if b.previousError != nil {
		return b
	}
	b.SetClauses.Columns = append(b.SetClauses.Columns, columns...)
	b.SetClauses.Record = rec
	return b
}

// SetMap appends the elements of the map at column/value pairs for the
// statement. Calls internally the `Set` function.
func (b *Update) SetMap(clauses map[string]Argument) *Update {
	if b.previousError != nil {
		return b
	}
	for col, val := range clauses {
		b.Set(col, val)
	}
	return b
}

// Where appends a WHERE clause to the statement
func (b *Update) Where(args ...ConditionArg) *Update {
	if b.previousError != nil {
		return b
	}
	b.WhereFragments = b.WhereFragments.append(args...)
	return b
}

// OrderBy appends columns to the ORDER BY statement for ascending sorting.
// Columns are getting quoted. When you use ORDER BY or GROUP BY to sort a
// column in a UPDATE, the server sorts values using only the initial number of
// bytes indicated by the max_sort_length system variable.
func (b *Update) OrderBy(columns ...string) *Update {
	b.OrderBys = b.OrderBys.appendColumns(columns, false)
	return b
}

// OrderByDesc appends columns to the ORDER BY statement for descending sorting.
// Columns are getting quoted. When you use ORDER BY or GROUP BY to sort a
// column in a UPDATE, the server sorts values using only the initial number of
// bytes indicated by the max_sort_length system variable.
func (b *Update) OrderByDesc(columns ...string) *Update {
	b.OrderBys = b.OrderBys.appendColumns(columns, false).applySort(len(columns), sortDescending)
	return b
}

// OrderByExpr adds a custom SQL expression to the ORDER BY clause. Does not
// quote the strings.
func (b *Update) OrderByExpr(columns ...string) *Update {
	b.OrderBys = b.OrderBys.appendColumns(columns, true)
	return b
}

// Limit sets a limit for the statement; overrides any existing LIMIT
func (b *Update) Limit(limit uint64) *Update {
	b.LimitCount = limit
	b.LimitValid = true
	return b
}

// Offset sets an offset for the statement; overrides any existing OFFSET
func (b *Update) Offset(offset uint64) *Update {
	b.OffsetCount = offset
	b.OffsetValid = true
	return b
}

// Interpolate if set stringyfies the arguments into the SQL string and returns
// pre-processed SQL command when calling the function ToSQL. Not suitable for
// prepared statements. ToSQLs second argument `Arguments` will then be nil.
func (b *Update) Interpolate() *Update {
	b.IsInterpolate = true
	return b
}

// ToSQL converts the select statement into a string and returns its arguments.
func (b *Update) ToSQL() (string, Arguments, error) {
	return toSQL(b, b.IsInterpolate)
}

func (b *Update) writeBuildCache(sql []byte, arguments Arguments) {
	b.buildCache = sql
	b.RawArguments = arguments
}

func (b *Update) readBuildCache() (sql []byte, arguments Arguments) {
	return b.buildCache, b.RawArguments
}

func (b *Update) hasBuildCache() bool {
	return b.UseBuildCache
}

// ToSQL serialized the Update to a SQL string
// It returns the string with placeholders and a slice of query arguments
func (b *Update) toSQL(buf queryWriter) error {
	if b.previousError != nil {
		return errors.Wrap(b.previousError, "[dbr] Update.ToSQL")
	}

	if err := b.Listeners.dispatch(OnBeforeToSQL, b); err != nil {
		return errors.Wrap(err, "[dbr] Update.Listeners.dispatch")
	}

	if b.RawFullSQL != "" {
		buf.WriteString(b.RawFullSQL)
		return nil
	}

	if len(b.Table.Name) == 0 {
		return errors.NewEmptyf("[dbr] Update: Table at empty")
	}
	if len(b.SetClauses.Columns) == 0 {
		return errors.NewEmptyf("[dbr] Update: SetClauses are empty")
	}

	buf.WriteString("UPDATE ")
	b.Table.FquoteAs(buf)
	buf.WriteString(" SET ")

	// Build SET clause SQL with placeholders and add values to args
	for i, c := range b.SetClauses.Columns {
		if i > 0 {
			buf.WriteString(", ")
		}
		Quoter.FquoteAs(buf, c)
		buf.WriteByte('=')
		if i < len(b.SetClauses.Arguments) {
			arg := b.SetClauses.Arguments[i]
			if e, ok := arg.(*expr); ok {
				e.writeTo(buf, 0)
			} else {
				buf.WriteByte('?')
			}
		} else {
			buf.WriteByte('?')
		}
	}

	// Write WHERE clause if we have any fragments
	if err := b.WhereFragments.write(buf, 'w'); err != nil {
		return errors.Wrap(err, "[dbr] Update.ToSQL.write")
	}

	sqlWriteOrderBy(buf, b.OrderBys, false)
	sqlWriteLimitOffset(buf, b.LimitValid, b.LimitCount, b.OffsetValid, b.OffsetCount)
	return nil
}

// ToSQL serialized the Update to a SQL string
// It returns the string with placeholders and a slice of query arguments
func (b *Update) appendArgs(args Arguments) (Arguments, error) {
	if b.previousError != nil {
		return nil, errors.Wrap(b.previousError, "[dbr] Update.appendArgs")
	}

	if b.RawFullSQL != "" {
		return b.RawArguments, nil
	}

	if args == nil {
		args = make(Arguments, 0, len(b.SetClauses.Columns)+len(b.WhereFragments))
	}
	if b.SetClauses.Record != nil {
		var err error
		args, err = b.SetClauses.Record.AssembleArguments(SQLStmtUpdate|SQLPartSet, args, b.SetClauses.Columns)
		if err != nil {
			return nil, errors.Wrap(err, "[dbr] Update.ToSQL Record.AssembleArguments")
		}
	}

	// Build SET clause SQL with placeholders and add values to args
	for _, arg := range b.SetClauses.Arguments {
		if e, ok := arg.(*expr); ok {
			args = append(args, e.Arguments...)
		} else {
			args = append(args, arg)
		}
	}

	// Write WHERE clause if we have any fragments
	args, pap, err := b.WhereFragments.appendArgs(args, 'w')
	if err != nil {
		return nil, errors.Wrap(err, "[dbr] Update.ToSQL.write")
	}
	if args, err = appendAssembledArgs(pap, b.SetClauses.Record, args, SQLStmtUpdate|SQLPartWhere, b.WhereFragments.Conditions()); err != nil {
		return nil, errors.Wrap(err, "[dbr] Update.toSQL.appendAssembledArgs")
	}

	return args, nil
}

// Exec interpolates and executes the statement represented by the Update
// object. It returns the raw database/sql Result and an error if there was one.
func (b *Update) Exec(ctx context.Context) (sql.Result, error) {
	sqlStr, args, err := b.ToSQL()
	if err != nil {
		return nil, errors.Wrap(err, "[dbr] Update.Exec.ToSQL")
	}

	if b.Log != nil && b.Log.IsInfo() {
		defer log.WhenDone(b.Log).Info("dbr.Update.Exec.Timing", log.String("sql", sqlStr))
	}

	result, err := b.DB.ExecContext(ctx, sqlStr, args.Interfaces()...)
	if err != nil {
		return nil, errors.Wrap(err, "[dbr] Update.Exec.Exec")
	}

	return result, nil
}

// Prepare creates a new prepared statement represented by the Update object. It
// returns the raw database/sql Stmt and an error if there was one.
func (b *Update) Prepare(ctx context.Context) (*sql.Stmt, error) {
	// TODO(CyS) implement build cache like the toSQL function
	buf := bufferpool.Get()
	defer bufferpool.Put(buf)

	if err := b.toSQL(buf); err != nil {
		return nil, errors.Wrap(err, "[dbr] Update.Prepare.ToSQL")
	}

	if b.Log != nil && b.Log.IsInfo() {
		defer log.WhenDone(b.Log).Info("dbr.Update.Prepare.Timing", log.String("sql", buf.String()))
	}

	stmt, err := b.DB.PrepareContext(ctx, buf.String())
	return stmt, errors.Wrap(err, "[dbr] Update.Prepare.Prepare")
}

// UpdatedColumns contains the column/argument association for either the SET
// clause in an UPDATE statement or to be used in an INSERT ... ON DUPLICATE KEY
// statement. For each column there must be one argument which can either be nil
// or has an actual value.
//
// When using the ON DUPLICATE KEY feature in the Insert builder:
//
// The function dbr.ArgExpr is supported and allows SQL
// constructs like (ib == InsertBuilder builds INSERT statements):
// 		`columnA`=VALUES(`columnB`)+2
// by writing the Go code:
//		ib.AddOnDuplicateKey("columnA", ArgExpr("VALUES(`columnB`)+?", ArgInt(2)))
// Omitting the argument and using the keyword nil will turn this Go code:
//		ib.AddOnDuplicateKey("columnA", nil)
// into that SQL:
// 		`columnA`=VALUES(`columnA`)
// Same applies as when the columns gets only assigned without any arguments:
//		ib.OnDuplicateKey.Columns = []string{"name","sku"}
// will turn into:
// 		`name`=VALUES(`name`), `sku`=VALUES(`sku`)
// Type `UpdatedColumns` gets used in type `Update` with field
// `SetClauses` and in type `Insert` with field OnDuplicateKey.
type UpdatedColumns struct {
	Columns   []string
	Arguments Arguments
	Record    ArgumentAssembler
}

// writeOnDuplicateKey writes the columns to `w` and appends the arguments to
// `args` and returns `args`.
func (uc UpdatedColumns) writeOnDuplicateKey(w queryWriter) error {
	if len(uc.Columns) == 0 {
		return nil
	}

	useArgs := len(uc.Arguments) == len(uc.Columns)

	w.WriteString(" ON DUPLICATE KEY UPDATE ")
	for i, c := range uc.Columns {
		if i > 0 {
			w.WriteString(", ")
		}
		Quoter.quote(w, c)
		w.WriteByte('=')
		if useArgs {
			// todo remove continue
			if e, ok := uc.Arguments[i].(*expr); ok {
				_ = e.writeTo(w, 0)
				continue
			}
			if uc.Arguments[i] == nil {
				w.WriteString("VALUES(")
				Quoter.quote(w, c)
				w.WriteByte(')')
				continue
			}
			w.WriteByte('?')
		} else {
			w.WriteString("VALUES(")
			Quoter.quote(w, c)
			w.WriteByte(')')
		}
	}
	return nil
}

func (uc UpdatedColumns) appendArgs(args Arguments) (Arguments, error) {
	if len(uc.Columns) == 0 {
		return args, nil
	}
	if len(uc.Arguments) == len(uc.Columns) {
		for i := range uc.Columns {
			if arg := uc.Arguments[i]; arg != nil { // must get skipped because VALUES(column_name)
				args = append(args, arg)
			}
		}
	}
	return args, nil
}

// UpdateMulti allows to run an UPDATE statement multiple times with different
// values in an optionally transaction. If you enable the interpolate feature on
// the Update object the interpolated SQL string will be send each time to the
// SQL server otherwise a prepared statement will be created. Create a single
// Update object without the SET arguments but with empty WHERE arguments. The
// empty WHERE arguments trigger the placeholder and the correct operator. The
// values itself will be provided either through the Records slice or via
// RecordChan.
type UpdateMulti struct {
	// IsTransaction set to true to enable running the UPDATE queries in a
	// transaction.
	IsTransaction bool
	// IsolationLevel defines the transaction isolation level.
	sql.IsolationLevel
	// Tx knows how to start a transaction. Must be set if transactions hasn't
	// been disabled.
	Tx TxBeginner
	// Update represents the template UPDATE statement.
	Update *Update

	// Alias provides a special feature that instead of the column name, the
	// alias will be passed to the ArgumentGenerater.Record function. If the alias
	// slice is empty the column names get passed. Otherwise the alias slice
	// must have the same length as the columns slice.
	Alias   []string
	Records []ArgumentAssembler
	// RecordChan waits for incoming records to send them to the prepared
	// statement. If the channel gets closed the transaction gets terminated and
	// the UPDATE statement removed.
	RecordChan <-chan ArgumentAssembler
}

// NewUpdateMulti creates new UPDATE statement which runs multiple times for a
// specific Update statement.
func NewUpdateMulti(tpl *Update) *UpdateMulti {
	return &UpdateMulti{
		Update: tpl,
	}
}

// AddRecords pulls in values to match Columns from the record.
func (b *UpdateMulti) AddRecords(recs ...ArgumentAssembler) *UpdateMulti {
	b.Records = append(b.Records, recs...)
	return b
}

// Transaction enables transaction usage and sets an optional isolation level.
// If not set the default database isolation level gets used.
func (b *UpdateMulti) Transaction(level ...sql.IsolationLevel) *UpdateMulti {
	b.IsTransaction = true
	if len(level) == 1 {
		b.IsolationLevel = level[0]
	}
	return b
}

func (b *UpdateMulti) validate() error {
	if len(b.Update.SetClauses.Columns) == 0 {
		return errors.NewEmptyf("[dbr] UpdateMulti: Columns are empty")
	}
	if len(b.Alias) > 0 && len(b.Alias) != len(b.Update.SetClauses.Columns) {
		return errors.NewMismatchf("[dbr] UpdateMulti: Alias slice and Columns slice must have the same length")
	}
	if len(b.Records) == 0 && b.RecordChan == nil {
		return errors.NewEmptyf("[dbr] UpdateMulti: Records empty or RecordChan is nil")
	}
	return nil
}

func txUpdateMultiRollback(tx Txer, previousErr error, msg string, args ...interface{}) ([]sql.Result, error) {
	if err := tx.Rollback(); err != nil {
		eArg := []interface{}{previousErr}
		return nil, errors.Wrapf(err, "[dbr] UpdateMulti.Tx.Rollback. Previous Error: %s. "+msg, append(eArg, args...)...)
	}
	return nil, errors.Wrapf(previousErr, msg, args...)
}

// Exec runs multiple UPDATE queries for different records in serial order. The
// returned result slice indexes are same index as for the Records slice.
func (b *UpdateMulti) Exec(ctx context.Context) ([]sql.Result, error) {
	if err := b.validate(); err != nil {
		return nil, errors.Wrap(err, "[dbr] UpdateMulti.Exec")
	}

	if b.Update.Log != nil && b.Update.Log.IsInfo() {
		defer log.WhenDone(b.Update.Log).Info("dbr.UpdateMulti.Exec.Timing",
			log.Stringer("sql", b.Update),
			log.Int("records", len(b.Records)))
	}

	isInterpolate := b.Update.IsInterpolate
	b.Update.IsInterpolate = false

	sqlBuf := bufferpool.Get()
	defer bufferpool.Put(sqlBuf)

	err := b.Update.toSQL(sqlBuf)
	if err != nil {
		return nil, errors.Wrap(err, "[dbr] UpdateMulti.Exec.Update.toSQL")
	}

	exec := b.Update.DB
	var tx Txer = txMock{}
	if b.IsTransaction {
		var err error
		tx, err = b.Tx.BeginTx(ctx, &sql.TxOptions{
			Isolation: b.IsolationLevel,
		})
		if err != nil {
			return nil, errors.Wrapf(err, "[dbr] UpdateMulti.Exec.Tx.BeginTx. with Query: %q", b.Update)
		}
		exec = tx
	}

	var stmt *sql.Stmt
	if !isInterpolate {
		stmt, err = exec.PrepareContext(ctx, sqlBuf.String())
		if err != nil {
			return txUpdateMultiRollback(tx, err, "[dbr] UpdateMulti.Exec.Prepare. with Query: %q", b.Update)
		}
		defer stmt.Close() // todo check for error
	}

	args := make(Arguments, 0, len(b.Records)+len(b.Update.WhereFragments))
	results := make([]sql.Result, len(b.Records))
	for i, rec := range b.Records {
		// For parallel execution see ExecChan
		b.Update.SetClauses.Record = rec
		args, err = b.Update.appendArgs(args)
		if err != nil {
			return txUpdateMultiRollback(tx, err, "[dbr] UpdateMulti.Exec.Interpolate. Index %d with Query: %q", i, sqlBuf)
		}
		if isInterpolate {
			fullSQL, err := interpolate(sqlBuf.Bytes(), args...)
			if err != nil {
				return txUpdateMultiRollback(tx, err, "[dbr] UpdateMulti.Exec.Interpolate. Index %d with Query: %q", i, sqlBuf)
			}

			results[i], err = exec.ExecContext(ctx, fullSQL)
			if err != nil {
				return txUpdateMultiRollback(tx, err, "[dbr] UpdateMulti.Exec.Exec. Index %d with Query: %q", i, sqlBuf)
			}
		} else {
			results[i], err = stmt.ExecContext(ctx, args.Interfaces()...)
			if err != nil {
				return txUpdateMultiRollback(tx, err, "[dbr] UpdateMulti.Exec.Stmt.Exec. Index %d with Query: %q", i, sqlBuf)
			}
		}
		args = args[:0] // reset for re-usage
	}

	if err := tx.Commit(); err != nil {
		return nil, errors.Wrapf(err, "[dbr] UpdateMulti.Tx.Commit. Query: %q", sqlBuf)
	}

	return results, nil
}

// ExecChan executes incoming Records and writes the output into the provided
// channels. It closes the channels once the queries have been sent.
// All queries will run parallel, except when using a transaction.
func (b *UpdateMulti) ExecChan(resChan chan<- sql.Result, errChan chan<- error) {
	defer close(resChan)
	defer close(errChan)
	if err := b.validate(); err != nil {
		errChan <- errors.Wrap(err, "[dbr] UpdateMulti.Exec")
		return
	}
	// This could run in parallel but it depends if each exec gets a
	// different connection. In a transaction only serial processing is
	// possible because a Go transaction gets bound to one connection.

	panic("TODO(CyS) implement")

}
