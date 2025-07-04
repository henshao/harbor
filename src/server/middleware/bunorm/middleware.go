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
	"net/http"

	"github.com/goharbor/harbor/src/lib/bunorm"
	"github.com/goharbor/harbor/src/server/middleware"
)

// Config defines the config for Bun ORM middleware.
type Config struct {
	// Creator defines a function to create Bun transaction
	Creator func() bunorm.Transaction
}

var (
	// DefaultConfig default Bun ORM config
	DefaultConfig = Config{
		Creator: func() bunorm.Transaction {
			return bunorm.NewDB()
		},
	}
)

// Middleware middleware which adds Bun DB to the http request context with default config
func Middleware(skippers ...middleware.Skipper) func(http.Handler) http.Handler {
	return MiddlewareWithConfig(DefaultConfig, skippers...)
}

// MiddlewareWithConfig middleware which adds Bun DB to the http request context with config
func MiddlewareWithConfig(config Config, skippers ...middleware.Skipper) func(http.Handler) http.Handler {
	if config.Creator == nil {
		config.Creator = DefaultConfig.Creator
	}

	return middleware.New(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		ctx := bunorm.NewContext(r.Context(), config.Creator())
		next.ServeHTTP(w, r.WithContext(ctx))
	}, skippers...)
}

// TransactionMiddleware provides transaction-aware middleware
// This creates a transaction for each request and commits/rollbacks automatically
func TransactionMiddleware(skippers ...middleware.Skipper) func(http.Handler) http.Handler {
	return middleware.New(func(w http.ResponseWriter, r *http.Request, next http.Handler) {
		db := bunorm.NewDB()
		
		tx, err := db.BeginTx(r.Context())
		if err != nil {
			http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
			return
		}

		ctx := bunorm.NewContext(r.Context(), tx)
		
		// Custom response writer to track if we should commit or rollback
		rw := &responseWriter{
			ResponseWriter: w,
			tx:            tx,
			committed:     false,
		}

		defer func() {
			if !rw.committed {
				// If we haven't committed yet and there's an error, rollback
				if rw.statusCode >= 400 {
					tx.Rollback()
				} else {
					tx.Commit()
				}
			}
		}()

		next.ServeHTTP(rw, r.WithContext(ctx))
	}, skippers...)
}

// responseWriter wraps http.ResponseWriter to track response status
type responseWriter struct {
	http.ResponseWriter
	tx         *bunorm.TxDB
	statusCode int
	committed  bool
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if rw.statusCode == 0 {
		rw.statusCode = 200
	}
	return rw.ResponseWriter.Write(b)
}

// CommitTransaction commits the transaction manually
func (rw *responseWriter) CommitTransaction() error {
	if !rw.committed {
		rw.committed = true
		return rw.tx.Commit()
	}
	return nil
}

// RollbackTransaction rolls back the transaction manually
func (rw *responseWriter) RollbackTransaction() error {
	if !rw.committed {
		rw.committed = true
		return rw.tx.Rollback()
	}
	return nil
}