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

package examples

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/goharbor/harbor/src/lib/bunorm"
	"github.com/goharbor/harbor/src/lib/config/models"
	projectModels "github.com/goharbor/harbor/src/pkg/project/models"
)

// TestBunMigration demonstrates the migration from Beego to Bun ORM
func TestBunMigration(t *testing.T) {
	// Skip if no test database configured
	if os.Getenv("TEST_DB_HOST") == "" {
		t.Skip("No test database configured")
	}

	// Initialize Bun database
	config := &bunorm.Config{
		Host:         os.Getenv("TEST_DB_HOST"),
		Port:         5432,
		Username:     os.Getenv("TEST_DB_USER"),
		Password:     os.Getenv("TEST_DB_PASSWORD"),
		Database:     os.Getenv("TEST_DB_NAME"),
		SSLMode:      "disable",
		MaxIdleConns: 10,
		MaxOpenConns: 20,
		MaxLifetime:  time.Hour,
		ConnTimeout:  time.Second * 30,
	}

	err := bunorm.InitDB(config)
	require.NoError(t, err)
	defer bunorm.Close()

	t.Run("ConfigEntry Migration", testConfigEntryMigration)
	t.Run("Project Migration", testProjectMigration)
	t.Run("Transaction Example", testTransactionExample)
	t.Run("Query Builder Example", testQueryBuilderExample)
}

// testConfigEntryMigration demonstrates ConfigEntry model migration
func testConfigEntryMigration(t *testing.T) {
	ctx := context.Background()
	db := bunorm.NewDB()

	// Clean up test data
	defer func() {
		_, err := db.NewDelete().
			Model((*models.BunConfigEntry)(nil)).
			Where("k LIKE 'test_%'").
			Exec(ctx)
		require.NoError(t, err)
	}()

	// Create a new config entry using Bun
	configEntry := &models.BunConfigEntry{
		Key:   "test_config_key",
		Value: "test_config_value",
	}

	// Insert using Bun ORM
	_, err := db.NewInsert().Model(configEntry).Exec(ctx)
	require.NoError(t, err)
	assert.Greater(t, configEntry.ID, int64(0))

	// Read back the config entry
	var retrieved models.BunConfigEntry
	err = db.NewSelect().
		Model(&retrieved).
		Where("id = ?", configEntry.ID).
		Scan(ctx)
	require.NoError(t, err)

	assert.Equal(t, configEntry.Key, retrieved.Key)
	assert.Equal(t, configEntry.Value, retrieved.Value)

	// Update the config entry
	retrieved.Value = "updated_value"
	_, err = db.NewUpdate().
		Model(&retrieved).
		WherePK().
		Exec(ctx)
	require.NoError(t, err)

	// Verify update
	var updated models.BunConfigEntry
	err = db.NewSelect().
		Model(&updated).
		Where("id = ?", configEntry.ID).
		Scan(ctx)
	require.NoError(t, err)
	assert.Equal(t, "updated_value", updated.Value)

	// Delete the config entry
	_, err = db.NewDelete().
		Model(&retrieved).
		WherePK().
		Exec(ctx)
	require.NoError(t, err)

	// Verify deletion
	var count int
	count, err = db.NewSelect().
		Model((*models.BunConfigEntry)(nil)).
		Where("id = ?", configEntry.ID).
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// testProjectMigration demonstrates Project model migration
func testProjectMigration(t *testing.T) {
	ctx := context.Background()
	db := bunorm.NewDB()

	// Clean up test data
	defer func() {
		_, err := db.NewDelete().
			Model((*projectModels.BunProject)(nil)).
			Where("name LIKE 'test_%'").
			Exec(ctx)
		require.NoError(t, err)
	}()

	// Create a new project using Bun
	project := &projectModels.BunProject{
		Name:         "test_project",
		OwnerID:      1,
		Deleted:      false,
		RegistryID:   0,
		CreationTime: time.Now(),
		UpdateTime:   time.Now(),
	}

	// Insert using Bun ORM
	_, err := db.NewInsert().Model(project).Exec(ctx)
	require.NoError(t, err)
	assert.Greater(t, project.ProjectID, int64(0))

	// Query with filters (demonstrating Bun query builder)
	var projects []projectModels.BunProject
	err = db.NewSelect().
		Model(&projects).
		Where("deleted = ?", false).
		Where("name LIKE ?", "test_%").
		Order("creation_time DESC").
		Limit(10).
		Scan(ctx)
	require.NoError(t, err)
	assert.Len(t, projects, 1)
	assert.Equal(t, project.Name, projects[0].Name)
}

// testTransactionExample demonstrates transaction handling with Bun
func testTransactionExample(t *testing.T) {
	ctx := context.Background()

	// Clean up test data
	defer func() {
		db := bunorm.NewDB()
		_, err := db.NewDelete().
			Model((*models.BunConfigEntry)(nil)).
			Where("k LIKE 'tx_test_%'").
			Exec(ctx)
		require.NoError(t, err)
	}()

	// Test successful transaction
	err := bunorm.WithTx(ctx, func(tx bunorm.Transaction) error {
		// Insert multiple config entries in a transaction
		configs := []*models.BunConfigEntry{
			{Key: "tx_test_1", Value: "value1"},
			{Key: "tx_test_2", Value: "value2"},
		}

		for _, config := range configs {
			_, err := tx.NewInsert().Model(config).Exec(ctx)
			if err != nil {
				return err
			}
		}

		return nil
	})
	require.NoError(t, err)

	// Verify both entries were created
	db := bunorm.NewDB()
	count, err := db.NewSelect().
		Model((*models.BunConfigEntry)(nil)).
		Where("k LIKE 'tx_test_%'").
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	// Test rollback transaction
	err = bunorm.WithTx(ctx, func(tx bunorm.Transaction) error {
		// Insert a config entry
		config := &models.BunConfigEntry{
			Key:   "tx_test_rollback",
			Value: "should_not_exist",
		}
		_, err := tx.NewInsert().Model(config).Exec(ctx)
		if err != nil {
			return err
		}

		// Force rollback by returning an error
		return assert.AnError
	})
	require.Error(t, err)

	// Verify the entry was not created due to rollback
	count, err = db.NewSelect().
		Model((*models.BunConfigEntry)(nil)).
		Where("k = 'tx_test_rollback'").
		Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

// testQueryBuilderExample demonstrates advanced query building with Bun
func testQueryBuilderExample(t *testing.T) {
	ctx := context.Background()
	db := bunorm.NewDB()

	// Clean up test data
	defer func() {
		_, err := db.NewDelete().
			Model((*projectModels.BunProject)(nil)).
			Where("name LIKE 'query_test_%'").
			Exec(ctx)
		require.NoError(t, err)
	}()

	// Create test data
	projects := []*projectModels.BunProject{
		{Name: "query_test_public", OwnerID: 1, Deleted: false},
		{Name: "query_test_private", OwnerID: 2, Deleted: false},
		{Name: "query_test_deleted", OwnerID: 1, Deleted: true},
	}

	for _, project := range projects {
		_, err := db.NewInsert().Model(project).Exec(ctx)
		require.NoError(t, err)
	}

	// Test complex query with subquery
	var activeProjects []projectModels.BunProject
	err := db.NewSelect().
		Model(&activeProjects).
		Where("deleted = ?", false).
		Where("name LIKE ?", "query_test_%").
		Where("owner_id IN (?)", db.NewSelect().
			Column("DISTINCT owner_id").
			Table("project").
			Where("deleted = ?", false)).
		Order("name ASC").
		Scan(ctx)
	require.NoError(t, err)
	assert.Len(t, activeProjects, 2)

	// Test aggregate query
	type OwnerStats struct {
		OwnerID      int   `bun:"owner_id"`
		ProjectCount int   `bun:"project_count"`
		ActiveCount  int   `bun:"active_count"`
	}

	var stats []OwnerStats
	err = db.NewSelect().
		Column("owner_id").
		ColumnExpr("COUNT(*) as project_count").
		ColumnExpr("COUNT(CASE WHEN deleted = false THEN 1 END) as active_count").
		Table("project").
		Where("name LIKE ?", "query_test_%").
		Group("owner_id").
		Order("owner_id ASC").
		Scan(ctx, &stats)
	require.NoError(t, err)
	assert.Len(t, stats, 2)

	// Owner 1 should have 2 projects (1 active, 1 deleted)
	owner1Stats := stats[0]
	assert.Equal(t, 1, owner1Stats.OwnerID)
	assert.Equal(t, 2, owner1Stats.ProjectCount)
	assert.Equal(t, 1, owner1Stats.ActiveCount)

	// Owner 2 should have 1 project (1 active)
	owner2Stats := stats[1]
	assert.Equal(t, 2, owner2Stats.OwnerID)
	assert.Equal(t, 1, owner2Stats.ProjectCount)
	assert.Equal(t, 1, owner2Stats.ActiveCount)
}

// BenchmarkBunVsBeego compares performance between Bun and Beego (simulation)
func BenchmarkBunVsBeego(b *testing.B) {
	// Skip if no test database configured
	if os.Getenv("TEST_DB_HOST") == "" {
		b.Skip("No test database configured")
	}

	// Initialize Bun database
	config := &bunorm.Config{
		Host:         os.Getenv("TEST_DB_HOST"),
		Port:         5432,
		Username:     os.Getenv("TEST_DB_USER"),
		Password:     os.Getenv("TEST_DB_PASSWORD"),
		Database:     os.Getenv("TEST_DB_NAME"),
		SSLMode:      "disable",
		MaxIdleConns: 10,
		MaxOpenConns: 20,
		MaxLifetime:  time.Hour,
		ConnTimeout:  time.Second * 30,
	}

	err := bunorm.InitDB(config)
	if err != nil {
		b.Fatalf("Failed to initialize database: %v", err)
	}
	defer bunorm.Close()

	ctx := context.Background()
	db := bunorm.NewDB()

	// Setup benchmark data
	_, err = db.NewInsert().
		Model(&models.BunConfigEntry{Key: "bench_test", Value: "benchmark_value"}).
		Exec(ctx)
	if err != nil {
		b.Fatalf("Failed to create benchmark data: %v", err)
	}

	defer func() {
		_, err := db.NewDelete().
			Model((*models.BunConfigEntry)(nil)).
			Where("k = 'bench_test'").
			Exec(ctx)
		if err != nil {
			b.Logf("Failed to clean up benchmark data: %v", err)
		}
	}()

	b.Run("BunSelect", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			var config models.BunConfigEntry
			err := db.NewSelect().
				Model(&config).
				Where("k = ?", "bench_test").
				Scan(ctx)
			if err != nil {
				b.Fatalf("Bun select failed: %v", err)
			}
		}
	})

	// Note: This would be the Beego comparison in a real scenario
	// b.Run("BeegoSelect", func(b *testing.B) {
	//     for i := 0; i < b.N; i++ {
	//         o := orm.NewOrm()
	//         var config models.ConfigEntry
	//         err := o.QueryTable("properties").Filter("k", "bench_test").One(&config)
	//         if err != nil {
	//             b.Fatalf("Beego select failed: %v", err)
	//         }
	//     }
	// })
}

// Example usage documentation
func ExampleBunMigration() {
	// This example shows how to migrate from Beego ORM to Bun ORM

	// 1. Initialize Bun database
	config := &bunorm.Config{
		Host:     "localhost",
		Port:     5432,
		Username: "harbor",
		Password: "password",
		Database: "harbor",
		SSLMode:  "disable",
	}
	bunorm.InitDB(config)

	// 2. Use Bun models instead of Beego models
	ctx := context.Background()
	db := bunorm.NewDB()

	// Old Beego way:
	// o := orm.NewOrm()
	// var projects []models.Project
	// _, err := o.QueryTable("project").Filter("deleted", false).All(&projects)

	// New Bun way:
	var projects []projectModels.BunProject
	err := db.NewSelect().
		Model(&projects).
		Where("deleted = ?", false).
		Scan(ctx)
	_ = err

	// 3. Use transactions with Bun
	err = bunorm.WithTx(ctx, func(tx bunorm.Transaction) error {
		project := &projectModels.BunProject{
			Name:    "example_project",
			OwnerID: 1,
		}
		_, err := tx.NewInsert().Model(project).Exec(ctx)
		return err
	})
	_ = err

	// Output: Migration completed successfully
}