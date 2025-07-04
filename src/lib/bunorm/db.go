// Copyright Project Harbor Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bunorm

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/driver/pgdriver"
	"github.com/uptrace/bun/extra/bundebug"

	"github.com/goharbor/harbor/src/lib/errors"
	"github.com/goharbor/harbor/src/lib/log"
)

var (
	// GlobalDB is the global database instance for Bun ORM
	GlobalDB *bun.DB
)

// Config holds the database configuration
type Config struct {
	Host         string
	Port         int
	Username     string
	Password     string
	Database     string
	SSLMode      string
	MaxIdleConns int
	MaxOpenConns int
	MaxLifetime  time.Duration
	ConnTimeout  time.Duration
}

// InitDB initializes the Bun database connection
func InitDB(config *Config) error {
	// Build PostgreSQL connection string
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=%s",
		config.Username,
		config.Password,
		config.Host,
		config.Port,
		config.Database,
		config.SSLMode,
	)

	// Create SQL database connection
	sqldb := sql.OpenDB(pgdriver.NewConnector(pgdriver.WithDSN(dsn)))

	// Configure connection pool
	sqldb.SetMaxIdleConns(config.MaxIdleConns)
	sqldb.SetMaxOpenConns(config.MaxOpenConns)
	sqldb.SetConnMaxLifetime(config.MaxLifetime)

	// Create Bun database instance
	GlobalDB = bun.NewDB(sqldb, pgdialect.New())

	// Add debugging if needed
	if os.Getenv("BUN_DEBUG") == "true" {
		GlobalDB.AddQueryHook(bundebug.NewQueryHook(
			bundebug.WithVerbose(true),
			bundebug.FromEnv("BUN_DEBUG"),
		))
	}

	// Test the connection
	ctx, cancel := context.WithTimeout(context.Background(), config.ConnTimeout)
	defer cancel()

	if err := GlobalDB.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("Bun database connection initialized successfully")
	return nil
}

// GetDB returns the global database instance
func GetDB() *bun.DB {
	if GlobalDB == nil {
		log.Fatal("Database not initialized. Call InitDB first.")
	}
	return GlobalDB
}

// Close closes the database connection
func Close() error {
	if GlobalDB != nil {
		return GlobalDB.Close()
	}
	return nil
}

// WithTx runs a function within a database transaction
func WithTx(ctx context.Context, fn func(tx bun.Tx) error) error {
	return GlobalDB.RunInTx(ctx, nil, func(ctx context.Context, tx bun.Tx) error {
		return fn(tx)
	})
}

// Transaction interface for compatibility
type Transaction interface {
	NewSelect() *bun.SelectQuery
	NewInsert() *bun.InsertQuery
	NewUpdate() *bun.UpdateQuery
	NewDelete() *bun.DeleteQuery
	NewRaw(query string, args ...interface{}) *bun.RawQuery
}

// DB wrapper that provides both transaction and non-transaction operations
type DB struct {
	*bun.DB
}

// NewDB creates a new DB wrapper
func NewDB() *DB {
	return &DB{DB: GetDB()}
}

// TxDB wraps a Bun transaction to provide the same interface as DB
type TxDB struct {
	tx bun.Tx
}

// NewSelect implements Transaction interface
func (db *DB) NewSelect() *bun.SelectQuery {
	return db.DB.NewSelect()
}

// NewInsert implements Transaction interface
func (db *DB) NewInsert() *bun.InsertQuery {
	return db.DB.NewInsert()
}

// NewUpdate implements Transaction interface
func (db *DB) NewUpdate() *bun.UpdateQuery {
	return db.DB.NewUpdate()
}

// NewDelete implements Transaction interface
func (db *DB) NewDelete() *bun.DeleteQuery {
	return db.DB.NewDelete()
}

// NewRaw implements Transaction interface
func (db *DB) NewRaw(query string, args ...interface{}) *bun.RawQuery {
	return db.DB.NewRaw(query, args...)
}

// BeginTx starts a transaction and returns TxDB
func (db *DB) BeginTx(ctx context.Context) (*TxDB, error) {
	tx, err := db.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	return &TxDB{tx: tx}, nil
}

// NewSelect implements Transaction interface for TxDB
func (tx *TxDB) NewSelect() *bun.SelectQuery {
	return tx.tx.NewSelect()
}

// NewInsert implements Transaction interface for TxDB
func (tx *TxDB) NewInsert() *bun.InsertQuery {
	return tx.tx.NewInsert()
}

// NewUpdate implements Transaction interface for TxDB
func (tx *TxDB) NewUpdate() *bun.UpdateQuery {
	return tx.tx.NewUpdate()
}

// NewDelete implements Transaction interface for TxDB
func (tx *TxDB) NewDelete() *bun.DeleteQuery {
	return tx.tx.NewDelete()
}

// NewRaw implements Transaction interface for TxDB
func (tx *TxDB) NewRaw(query string, args ...interface{}) *bun.RawQuery {
	return tx.tx.NewRaw(query, args...)
}

// Commit commits the transaction
func (tx *TxDB) Commit() error {
	return tx.tx.Commit()
}

// Rollback rolls back the transaction
func (tx *TxDB) Rollback() error {
	return tx.tx.Rollback()
}

// CompatDB provides a compatibility layer between Beego ORM and Bun
type CompatDB struct {
	db Transaction
}

// NewCompatDB creates a new compatibility database wrapper
func NewCompatDB(tx Transaction) *CompatDB {
	return &CompatDB{db: tx}
}

// Insert inserts a record using Bun syntax compatible with Beego patterns
func (c *CompatDB) Insert(ctx context.Context, model interface{}) (int64, error) {
	result, err := c.db.NewInsert().Model(model).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Update updates a record using Bun syntax
func (c *CompatDB) Update(ctx context.Context, model interface{}) (int64, error) {
	result, err := c.db.NewUpdate().Model(model).WherePK().Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// Delete deletes a record using Bun syntax
func (c *CompatDB) Delete(ctx context.Context, model interface{}) (int64, error) {
	result, err := c.db.NewDelete().Model(model).WherePK().Exec(ctx)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// SelectOne selects a single record
func (c *CompatDB) SelectOne(ctx context.Context, model interface{}) error {
	return c.db.NewSelect().Model(model).WherePK().Scan(ctx)
}

// SelectMany selects multiple records
func (c *CompatDB) SelectMany(ctx context.Context, models interface{}) error {
	return c.db.NewSelect().Model(models).Scan(ctx)
}

// contextKey for storing Bun DB in context
type contextKey struct{}

// FromContext returns Bun DB from context
func FromContext(ctx context.Context) (Transaction, error) {
	db, ok := ctx.Value(contextKey{}).(Transaction)
	if !ok {
		return nil, errors.New("cannot get the Bun DB from context")
	}
	return db, nil
}

// NewContext returns new context with Bun DB
func NewContext(ctx context.Context, db Transaction) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, contextKey{}, db)
}

// Context returns a context with Bun DB
func Context() context.Context {
	return NewContext(context.Background(), NewDB())
}