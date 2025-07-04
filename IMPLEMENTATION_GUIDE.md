# Harbor Beego 到 Bun ORM 迁移实施指南

## 开始之前

### 1. 安装依赖

首先确保新的 Bun 依赖已添加到 `go.mod`：

```bash
cd src
go mod tidy
```

### 2. 使用自动化迁移工具

使用我们创建的迁移工具来转换现有模型：

```bash
# 转换单个模型文件
go run tools/migration/beego_to_bun.go src/pkg/project/models/project.go

# 批量转换多个文件
find src -name "*.go" -path "*/models/*" -exec grep -l "orm:\"" {} \; | while read file; do
    echo "Converting $file"
    go run tools/migration/beego_to_bun.go "$file"
done
```

## 分阶段迁移策略

### 第一阶段：核心基础设施

#### 1. 初始化 Bun 数据库连接

在 `src/core/main.go` 中添加 Bun 初始化：

```go
import (
    "github.com/goharbor/harbor/src/lib/bunorm"
)

func initBunDB() {
    config := &bunorm.Config{
        Host:         os.Getenv("POSTGRESQL_HOST"),
        Port:         5432,
        Username:     os.Getenv("POSTGRESQL_USERNAME"),
        Password:     os.Getenv("POSTGRESQL_PASSWORD"),
        Database:     os.Getenv("POSTGRESQL_DATABASE"),
        SSLMode:      os.Getenv("POSTGRESQL_SSLMODE"),
        MaxIdleConns: 50,
        MaxOpenConns: 100,
        MaxLifetime:  time.Hour,
        ConnTimeout:  time.Second * 30,
    }
    
    if err := bunorm.InitDB(config); err != nil {
        log.Fatalf("Failed to initialize Bun database: %v", err)
    }
}

func main() {
    // ... existing code ...
    
    // Initialize Bun DB alongside existing Beego ORM
    initBunDB()
    
    // ... rest of main function ...
}
```

#### 2. 更新中间件链

在路由设置中添加 Bun 中间件：

```go
import (
    "github.com/goharbor/harbor/src/server/middleware/bunorm"
)

// 在需要数据库访问的路由上添加 Bun 中间件
router.Use(bunorm.Middleware())
```

### 第二阶段：模型迁移

#### 1. 创建 Bun 版本的核心模型

优先迁移以下核心模型：
- `ConfigEntry` (已完成)
- `Project` (已完成) 
- `User`
- `Repository`
- `Artifact`

#### 2. 示例：User 模型迁移

```go
// Bun 版本的 User 模型
type BunUser struct {
    bun.BaseModel `bun:"table:harbor_user"`

    UserID       int64     `bun:",pk,autoincrement" json:"user_id"`
    Username     string    `bun:"username,unique,notnull" json:"username"`
    Email        string    `bun:"email,unique,notnull" json:"email"`
    Password     string    `bun:"password,notnull" json:"-"`
    Realname     string    `bun:"realname" json:"realname"`
    Comment      string    `bun:"comment" json:"comment"`
    Deleted      bool      `bun:"deleted,notnull,default:false" json:"deleted"`
    ResetUUID    string    `bun:"reset_uuid" json:"reset_uuid"`
    Salt         string    `bun:"salt" json:"-"`
    SysAdminFlag bool      `bun:"sysadmin_flag,notnull,default:false" json:"sysadmin_flag"`
    CreationTime time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"creation_time"`
    UpdateTime   time.Time `bun:",nullzero,notnull,default:current_timestamp" json:"update_time"`
}
```

### 第三阶段：DAO 层迁移

#### 1. 创建 Bun 版本的 DAO 接口

```go
// DAO 接口保持不变，内部实现切换到 Bun
type ProjectDAO interface {
    Create(ctx context.Context, project *models.Project) (int64, error)
    Get(ctx context.Context, id int64) (*models.Project, error)
    List(ctx context.Context, query *models.ProjectQuery) ([]*models.Project, error)
    Update(ctx context.Context, project *models.Project) error
    Delete(ctx context.Context, id int64) error
}

// Bun 实现
type bunProjectDAO struct{}

func (d *bunProjectDAO) Create(ctx context.Context, project *models.Project) (int64, error) {
    db, err := bunorm.FromContext(ctx)
    if err != nil {
        return 0, err
    }
    
    bunProject := convertToBunProject(project)
    _, err = db.NewInsert().Model(bunProject).Exec(ctx)
    if err != nil {
        return 0, err
    }
    
    return bunProject.ProjectID, nil
}

func (d *bunProjectDAO) Get(ctx context.Context, id int64) (*models.Project, error) {
    db, err := bunorm.FromContext(ctx)
    if err != nil {
        return nil, err
    }
    
    var bunProject models.BunProject
    err = db.NewSelect().Model(&bunProject).Where("project_id = ?", id).Scan(ctx)
    if err != nil {
        return nil, err
    }
    
    return convertFromBunProject(&bunProject), nil
}
```

#### 2. 模型转换函数

创建 Beego 和 Bun 模型之间的转换函数：

```go
func convertToBunProject(project *models.Project) *models.BunProject {
    return &models.BunProject{
        ProjectID:    project.ProjectID,
        OwnerID:      project.OwnerID,
        Name:         project.Name,
        CreationTime: project.CreationTime,
        UpdateTime:   project.UpdateTime,
        Deleted:      project.Deleted,
        RegistryID:   project.RegistryID,
        // 复制其他字段...
    }
}

func convertFromBunProject(bunProject *models.BunProject) *models.Project {
    return &models.Project{
        ProjectID:    bunProject.ProjectID,
        OwnerID:      bunProject.OwnerID,
        Name:         bunProject.Name,
        CreationTime: bunProject.CreationTime,
        UpdateTime:   bunProject.UpdateTime,
        Deleted:      bunProject.Deleted,
        RegistryID:   bunProject.RegistryID,
        // 复制其他字段...
    }
}
```

### 第四阶段：渐进式切换

#### 1. 功能标志切换

使用环境变量控制是否使用 Bun：

```go
func getProjectDAO(ctx context.Context) ProjectDAO {
    if os.Getenv("USE_BUN_ORM") == "true" {
        return &bunProjectDAO{}
    }
    return &beegoProjectDAO{} // 现有的 Beego 实现
}
```

#### 2. 双写验证

在关键操作中同时写入两个 ORM，用于验证数据一致性：

```go
func (d *dualProjectDAO) Create(ctx context.Context, project *models.Project) (int64, error) {
    // 主要写入 Beego
    id, err := d.beegoDAO.Create(ctx, project)
    if err != nil {
        return 0, err
    }
    
    // 验证写入 Bun（异步）
    go func() {
        bunID, bunErr := d.bunDAO.Create(ctx, project)
        if bunErr != nil || bunID != id {
            log.Errorf("Bun write verification failed: id=%d, bunID=%d, err=%v", id, bunID, bunErr)
        }
    }()
    
    return id, nil
}
```

## 测试策略

### 1. 单元测试

```go
func TestBunProjectDAO_Create(t *testing.T) {
    // 设置测试数据库
    config := &bunorm.Config{
        Host:     "localhost",
        Port:     5432,
        Username: "test",
        Password: "test",
        Database: "test_harbor",
        SSLMode:  "disable",
    }
    
    err := bunorm.InitDB(config)
    require.NoError(t, err)
    
    // 清理测试数据
    defer func() {
        db := bunorm.GetDB()
        _, err := db.NewDelete().Model((*models.BunProject)(nil)).Where("name LIKE 'test_%'").Exec(context.Background())
        require.NoError(t, err)
    }()
    
    dao := &bunProjectDAO{}
    project := &models.Project{
        Name:    "test_project",
        OwnerID: 1,
    }
    
    id, err := dao.Create(context.Background(), project)
    require.NoError(t, err)
    require.Greater(t, id, int64(0))
    
    // 验证创建的项目
    retrieved, err := dao.Get(context.Background(), id)
    require.NoError(t, err)
    assert.Equal(t, project.Name, retrieved.Name)
    assert.Equal(t, project.OwnerID, retrieved.OwnerID)
}
```

### 2. 集成测试

```go
func TestProjectWorkflow_WithBun(t *testing.T) {
    // 使用真实的 HTTP 请求测试完整工作流
    os.Setenv("USE_BUN_ORM", "true")
    defer os.Unsetenv("USE_BUN_ORM")
    
    // 创建项目
    resp := testCreateProject(t, "test_project")
    projectID := extractProjectID(resp)
    
    // 获取项目
    project := testGetProject(t, projectID)
    assert.Equal(t, "test_project", project.Name)
    
    // 更新项目
    project.Name = "updated_project"
    testUpdateProject(t, project)
    
    // 删除项目
    testDeleteProject(t, projectID)
}
```

### 3. 性能基准测试

```go
func BenchmarkProjectDAO_List(b *testing.B) {
    setupBenchmarkData(b)
    
    b.Run("Beego", func(b *testing.B) {
        dao := &beegoProjectDAO{}
        for i := 0; i < b.N; i++ {
            _, err := dao.List(context.Background(), &models.ProjectQuery{})
            if err != nil {
                b.Fatal(err)
            }
        }
    })
    
    b.Run("Bun", func(b *testing.B) {
        dao := &bunProjectDAO{}
        for i := 0; i < b.N; i++ {
            _, err := dao.List(context.Background(), &models.ProjectQuery{})
            if err != nil {
                b.Fatal(err)
            }
        }
    })
}
```

## 数据迁移脚本

### 1. 验证数据一致性

```sql
-- 验证项目表数据一致性
SELECT 
    COUNT(*) as total_projects,
    COUNT(CASE WHEN deleted = true THEN 1 END) as deleted_projects,
    MIN(creation_time) as earliest_project,
    MAX(creation_time) as latest_project
FROM project;
```

### 2. 性能优化建议

```sql
-- 为 Bun 查询添加必要的索引
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_project_owner_id ON project(owner_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_project_name ON project(name);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_project_registry_id ON project(registry_id);
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_project_deleted ON project(deleted);
```

## 回滚计划

### 1. 快速回滚

```go
// 通过环境变量快速切回 Beego
os.Setenv("USE_BUN_ORM", "false")
```

### 2. 数据恢复

```bash
# 如果数据出现问题，从备份恢复
pg_restore -h localhost -U harbor -d harbor_db /backup/harbor_backup.sql
```

## 监控和告警

### 1. 性能监控

```go
// 添加 Bun 查询的性能监控
func (d *bunProjectDAO) List(ctx context.Context, query *models.ProjectQuery) ([]*models.Project, error) {
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        metrics.DBQueryDuration.WithLabelValues("bun", "project", "list").Observe(duration.Seconds())
    }()
    
    // ... 实际查询逻辑
}
```

### 2. 错误监控

```go
// 监控 Bun 查询错误
if err != nil {
    metrics.DBQueryErrors.WithLabelValues("bun", "project", "list").Inc()
    log.Errorf("Bun query failed: %v", err)
    return nil, err
}
```

## 最终切换检查清单

- [ ] 所有核心模型已迁移到 Bun
- [ ] 所有 DAO 实现已更新
- [ ] 单元测试全部通过
- [ ] 集成测试全部通过
- [ ] 性能基准测试满足要求
- [ ] 生产环境双写验证通过
- [ ] 监控和告警配置完成
- [ ] 回滚计划已验证
- [ ] 团队培训已完成
- [ ] 文档已更新

## 预期收益验证

### 1. 性能提升测试

```bash
# 运行性能基准测试
go test -bench=. ./src/pkg/project/dao/...

# 监控生产环境性能指标
# - 查询响应时间
# - 数据库连接池使用率
# - 内存使用情况
```

### 2. 开发体验改善

- 类型安全的查询构建
- 更好的错误处理
- 更直观的 API
- 支持 Go 泛型

这个迁移将显著提升 Harbor 项目的性能和开发体验，同时保持向后兼容性和数据完整性。