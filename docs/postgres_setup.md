# PostgreSQL 数据库配置

## 配置信息

数据库已成功配置并连接到PostgreSQL：

```yaml
database:
  type: "postgres"
  host: "localhost"
  port: 5432
  user: "wuliang"
  password: "lingting"
  dbname: "wolink_core"
  sslmode: "disable"
```

## 数据库状态

✅ **数据库已创建**: `wolink_core`
✅ **连接测试**: 成功
✅ **表自动迁移**: 已完成
✅ **应用启动**: 成功

## 已创建的表

应用启动时自动创建了以下表：
- `departments` - 部门表
- `api_keys` - API密钥表
- `model_registries` - 模型注册表
- `api_key_model_mappings` - API密钥模型映射表
- `conversations` - 对话记录表
- `usage_logs` - 使用日志表

## 连接测试

测试数据库连接：
```bash
PGPASSWORD=lingting psql -h localhost -p 5432 -U wuliang -d wolink_core -c "SELECT 1;"
```

查看已创建的表：
```bash
PGPASSWORD=lingting psql -h localhost -p 5432 -U wuliang -d wolink_core -c "\dt"
```

## 启动服务

使用PostgreSQL数据库启动服务：
```bash
go run cmd/main.go
```

服务将在 `http://localhost:8080` 启动。

## 数据库管理

### 连接数据库
```bash
PGPASSWORD=lingting psql -h localhost -p 5432 -U wuliang -d wolink_core
```

### 查看表结构
```sql
\dt                    # 列出所有表
\d conversations       # 查看conversations表结构
\d api_keys           # 查看api_keys表结构
```

### 查看数据
```sql
SELECT * FROM model_registries;
SELECT * FROM api_keys;
SELECT COUNT(*) FROM conversations;
```

### 备份数据库
```bash
PGPASSWORD=lingting pg_dump -h localhost -p 5432 -U wuliang wolink_core > backup.sql
```

### 恢复数据库
```bash
PGPASSWORD=lingting psql -h localhost -p 5432 -U wuliang wolink_core < backup.sql
```

## 分布式部署

多节点部署时，所有节点共享同一个PostgreSQL数据库：

```yaml
# 所有节点使用相同的数据库配置
database:
  type: "postgres"
  host: "localhost"  # 或数据库集群地址
  port: 5432
  user: "wuliang"
  password: "lingting"
  dbname: "wolink_core"
  sslmode: "disable"
```

### 连接池配置

对于多节点部署，建议调整连接池大小：

```yaml
infrastructure:
  database_pool:
    max_open_conns: 50      # 每节点最大连接数
    max_idle_conns: 20      # 每节点空闲连接数
    conn_max_lifetime: "5m" # 连接最大存活时间
    conn_max_idle_time: "2m" # 空闲连接最大存活时间
```

**总连接数计算**：
```
总连接数 = 节点数 × max_open_conns

例如：
- 3节点：3 × 50 = 150 连接
- 5节点：5 × 50 = 250 连接
```

## 性能优化

### PostgreSQL 配置建议

在 `postgresql.conf` 中优化：

```conf
# 连接数
max_connections = 500

# 内存
shared_buffers = 256MB
effective_cache_size = 1GB
work_mem = 4MB

# WAL
wal_level = replica
max_wal_size = 1GB
min_wal_size = 80MB
```

### 索引优化

对于大表，可以考虑添加额外索引：

```sql
-- 对话表索引
CREATE INDEX idx_conversations_created_at ON conversations(created_at DESC);
CREATE INDEX idx_conversations_api_key ON conversations(api_key_id);
CREATE INDEX idx_conversations_department ON conversations(department_id);

-- 使用日志索引
CREATE INDEX idx_usage_logs_request_time ON usage_logs(request_time DESC);
CREATE INDEX idx_usage_logs_api_key ON usage_logs(api_key_id);
```

## 监控

### 连接数监控
```sql
SELECT count(*) FROM pg_stat_activity WHERE datname = 'wolink_core';
```

### 表大小
```sql
SELECT 
    tablename,
    pg_size_pretty(pg_total_relation_size(schemaname||'.'||tablename)) AS size
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY pg_total_relation_size(schemaname||'.'||tablename) DESC;
```

### 慢查询
```sql
SELECT query, mean_time, calls
FROM pg_stat_statements
WHERE dbid = (SELECT oid FROM pg_database WHERE datname = 'wolink_core')
ORDER BY mean_time DESC
LIMIT 10;
```

## 故障排查

### 连接失败

1. 检查PostgreSQL服务是否运行：
```bash
brew services list | grep postgresql
```

2. 检查端口是否监听：
```bash
lsof -i :5432
```

3. 测试连接：
```bash
PGPASSWORD=lingting psql -h localhost -p 5432 -U wuliang -d postgres -c "SELECT 1;"
```

### 权限问题

确保用户有足够权限：
```sql
GRANT ALL PRIVILEGES ON DATABASE wolink_core TO wuliang;
\c wolink_core
GRANT ALL ON ALL TABLES IN SCHEMA public TO wuliang;
```

## 迁移指南

### 从 SQLite 迁移到 PostgreSQL

如果之前使用SQLite，可以按以下步骤迁移：

1. 导出SQLite数据：
```bash
sqlite3 wolink.db .dump > sqlite_export.sql
```

2. 转换SQL语法（需要手动调整）

3. 导入到PostgreSQL：
```bash
PGPASSWORD=lingting psql -h localhost -p 5432 -U wuliang wolink_core < converted.sql
```

或使用专业工具如 `pgloader`。

## 安全建议

1. **生产环境**：
   - 更改默认密码
   - 启用SSL连接（修改sslmode）
   - 限制数据库访问IP

2. **备份策略**：
   - 定期自动备份
   - 保留至少7天备份
   - 测试备份恢复流程

3. **监控告警**：
   - 监控连接数
   - 监控磁盘空间
   - 监控慢查询

## 相关信息

- PostgreSQL版本：14+
- 连接驱动：gorm.io/driver/postgres
- ORM：GORM v1.30+
- 连接池：Go database/sql

## 测试验证

✅ 数据库连接测试通过
✅ 表自动迁移测试通过
✅ 应用启动测试通过
✅ 模型注册测试通过

数据库已就绪，可以正常使用！
