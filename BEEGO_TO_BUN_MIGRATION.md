# Harbor ORM Migration: Beego to Bun

## 概述

本文档详细说明了将 Harbor 项目从 Beego ORM v2 迁移到 Bun ORM 的完整计划和实施步骤。

## 迁移范围

### 当前 Beego ORM 使用情况
- **依赖包**: `github.com/beego/beego/v2 v2.3.6`
- **主要包**: 
  - `github.com/beego/beego/v2/client/orm` - ORM 核心
  - `github.com/beego/beego/v2/server/web` - Web 框架
- **影响文件**: 50+ 个模型文件，100+ 个 DAO 文件
- **数据库**: PostgreSQL (主要)

### 目标 Bun ORM
- **包**: `github.com/uptrace/bun`
- **数据库驱动**: `github.com/uptrace/bun/driver/pgdriver`
- **额外包**: `github.com/uptrace/bun/dialect/pgdialect`

## 迁移计划

### Phase 1: 依赖和初始化迁移

#### 1.1 更新 go.mod
```diff
- github.com/beego/beego/v2 v2.3.6
+ github.com/uptrace/bun v1.1.16
+ github.com/uptrace/bun/driver/pgdriver v1.1.16
+ github.com/uptrace/bun/dialect/pgdialect v1.1.16
+ github.com/uptrace/bun/extra/bundebug v1.1.16
```

#### 1.2 数据库连接初始化
- 替换 `orm.RegisterDataBase()` 为 Bun 连接
- 更新连接池配置
- 迁移事务管理

### Phase 2: 模型定义迁移

#### 2.1 结构体标签迁移
**Beego 标签 → Bun 标签映射:**

| Beego 标签 | Bun 标签 | 说明 |
|------------|----------|------|
| `orm:"pk;auto;column(id)"` | `bun:",pk,autoincrement"` | 主键自增 |
| `orm:"column(name)"` | `bun:"name"` | 列名 |
| `orm:"auto_now_add"` | `bun:",nullzero,notnull,default:current_timestamp"` | 创建时间 |
| `orm:"auto_now"` | `bun:",nullzero,notnull,default:current_timestamp"` | 更新时间 |
| `orm:"-"` | `bun:"-"` | 忽略字段 |
| `orm:"size(255)"` | `bun:",type:varchar(255)"` | 字段类型 |

#### 2.2 模型注册迁移
- 移除 `orm.RegisterModel()` 调用
- 使用 Bun 的表创建机制

### Phase 3: ORM 操作迁移

#### 3.1 查询操作
**Beego → Bun:**
```go
// Before (Beego)
o := orm.NewOrm()
var projects []Project
_, err := o.QueryTable("project").Filter("deleted", false).All(&projects)

// After (Bun)
err := db.NewSelect().Model(&projects).Where("deleted = ?", false).Scan(ctx)
```

#### 3.2 插入操作
```go
// Before (Beego)
id, err := o.Insert(&project)

// After (Bun)
_, err := db.NewInsert().Model(&project).Exec(ctx)
```

#### 3.3 更新操作
```go
// Before (Beego)
_, err := o.Update(&project)

// After (Bun)
_, err := db.NewUpdate().Model(&project).WherePK().Exec(ctx)
```

### Phase 4: 中间件和上下文迁移

#### 4.1 ORM 中间件重构
- 更新 `src/server/middleware/orm/orm.go`
- 重构上下文传递机制
- 更新 `src/lib/orm/orm.go`

#### 4.2 事务处理迁移
- 重构 `WithTransaction` 函数
- 更新事务上下文管理

### Phase 5: 测试迁移

#### 5.1 Mock 对象更新
- 重构 `FakeOrmer` 类型
- 更新测试用例中的 ORM 调用
- 重构数据库测试工具

## 实施步骤

### 步骤 1: 核心依赖迁移
1. 更新 `go.mod` 文件
2. 创建新的数据库连接管理器
3. 保持兼容性的适配层

### 步骤 2: 模型逐步迁移
1. 从最小的模型开始 (如 `ConfigEntry`)
2. 逐个更新模型定义
3. 验证数据库操作

### 步骤 3: DAO 层迁移
1. 更新每个包的 DAO 实现
2. 保持接口不变，只修改内部实现
3. 逐步测试验证

### 步骤 4: 中间件层迁移
1. 重构 ORM 中间件
2. 更新上下文处理
3. 验证整体流程

### 步骤 5: 测试和验证
1. 运行完整测试套件
2. 性能基准测试
3. 集成测试验证

## 注意事项

### 兼容性考虑
- Bun 使用不同的查询构建语法
- 需要重写复杂查询
- 事务处理模型不同

### 性能优化
- Bun 提供更好的查询性能
- 支持查询缓存
- 更好的连接池管理

### 风险缓解
- 创建完整的备份
- 分阶段实施，保持可回滚性
- 完整的测试覆盖

## 预期收益

1. **性能提升**: Bun 提供更好的查询性能和内存使用
2. **现代化**: 支持 Go 泛型，类型安全的查询
3. **开发体验**: 更直观的 API 和更好的调试支持
4. **维护性**: 更清晰的代码结构和更好的错误处理

## 时间线

- **准备阶段**: 1-2 天
- **核心迁移**: 5-7 天
- **测试验证**: 2-3 天
- **文档更新**: 1 天

**总计**: 约 2 周时间