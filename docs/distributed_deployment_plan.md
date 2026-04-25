# 分布式部署改进方案

## 问题分析

### 当前架构状态

**✅ 已支持分布式的组件：**
1. 数据库（MySQL/PostgreSQL）- 共享数据源
2. Redis缓存 - 共享缓存层
3. Redis队列（QueueService）- 多节点共同消费
4. API Key验证 - 基于Redis缓存
5. 速率限制 - 基于Redis计数器

**❌ 需要改进的组件：**
1. CommunicationLogger - 本地文件存储，不支持多节点
2. ModelConfigService - 本地文件加载，可能不一致
3. 缺少节点标识和健康检查
4. 缺少分布式配置管理

## 改进方案

### 1. CommunicationLogger 分布式改造

**当前问题：**
- 使用本地文件存储，每个节点写入自己的文件
- 无法集中管理和分析

**解决方案：双写模式**

```yaml
communication_log:
  enabled: true
  # 模式：local（本地）/ shared（共享NFS）/ disabled
  mode: "local"
  storage_path: "./logs/communications"
  # 是否同时写入Redis队列（用于集中收集）
  enable_remote_sync: true
  redis_queue_key: "wolink:comm_logs"
  max_file_size_mb: 100
  flush_interval_ms: 1000
```

**实现策略：**
- 节点继续写入本地文件（保证性能和可靠性）
- 同时异步发送到Redis队列
- 独立的日志收集器从Redis队列消费并集中存储
- 或者使用共享存储（NFS/对象存储）

### 2. 节点标识和管理

**新增配置：**

```yaml
node:
  id: "node-01"  # 唯一节点ID，可以用hostname或环境变量
  region: "cn-east-1"  # 可选：区域标识
```

**新增服务：NodeService**
- 节点注册和心跳
- 节点状态监控
- 负载均衡信息

### 3. 模型配置同步

**当前问题：**
- 每个节点从本地文件系统加载配置
- 配置更新需要重启所有节点

**解决方案：**
- 配置存储在数据库中
- 使用Redis Pub/Sub通知配置变更
- 节点热加载配置，无需重启

### 4. 部署架构建议

```
                    ┌─────────────────┐
                    │   Load Balancer │
                    │   (Nginx/ALB)   │
                    └────────┬────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
        ┌─────▼─────┐ ┌─────▼─────┐ ┌─────▼─────┐
        │  Node-01  │ │  Node-02  │ │  Node-03  │
        │ Wolink    │ │ Wolink    │ │ Wolink    │
        └─────┬─────┘ └─────┬─────┘ └─────┬─────┘
              │              │              │
              └──────────────┼──────────────┘
                             │
              ┌──────────────┼──────────────┐
              │              │              │
        ┌─────▼─────┐ ┌─────▼─────┐ ┌─────▼─────┐
        │   MySQL   │ │   Redis   │ │   NFS/    │
        │  (共享)   │ │  (共享)   │ │   S3      │
        └───────────┘ └───────────┘ └───────────┘
```

## 实施步骤

### Phase 1: 基础支持（必须）
1. ✅ 添加节点标识配置
2. ✅ CommunicationLogger支持Redis同步
3. ✅ 验证Redis队列多节点消费

### Phase 2: 配置管理（推荐）
1. 模型配置数据库化
2. Redis Pub/Sub配置同步
3. 热加载支持

### Phase 3: 监控运维（可选）
1. 节点注册和心跳
2. 健康检查端点
3. 监控指标暴露

## 配置文件示例

```yaml
# 多节点部署配置
server:
  port: "8080"
  mode: "release"

# 节点标识
node:
  id: "${HOSTNAME}"  # 使用环境变量
  region: "cn-east-1"

database:
  type: "mysql"
  host: "mysql-cluster.internal"
  port: 3306
  user: "wolink"
  password: "${DB_PASSWORD}"
  dbname: "wolink_core"

redis:
  host: "redis-cluster.internal"
  port: 6379
  password: "${REDIS_PASSWORD}"
  db: 0

communication_log:
  enabled: true
  mode: "local"  # local 或 shared
  storage_path: "./logs/communications"
  # 启用远程同步到Redis
  enable_remote_sync: true
  redis_queue_key: "wolink:comm_logs"
  max_file_size_mb: 100
  flush_interval_ms: 1000

# 基础设施配置
infrastructure:
  shutdown_timeout: "30s"
  read_timeout: "15s"
  write_timeout: "30s"
  idle_timeout: "120s"
  read_header_timeout: "5s"
  database_pool:
    max_open_conns: 50
    max_idle_conns: 20
    conn_max_lifetime: "5m"
  redis_pool:
    pool_size: 50
    min_idle_conns: 10
    conn_max_lifetime: "5m"
```

## 部署脚本示例

### Docker Compose

```yaml
version: '3.8'

services:
  wolink-node-1:
    image: wolink-core:latest
    environment:
      - HOSTNAME=wolink-node-1
      - DB_PASSWORD=secret
      - REDIS_PASSWORD=secret
    volumes:
      - ./config:/app/config
      - ./logs:/app/logs
    depends_on:
      - mysql
      - redis

  wolink-node-2:
    image: wolink-core:latest
    environment:
      - HOSTNAME=wolink-node-2
      - DB_PASSWORD=secret
      - REDIS_PASSWORD=secret
    volumes:
      - ./config:/app/config
      - ./logs:/app/logs
    depends_on:
      - mysql
      - redis

  mysql:
    image: mysql:8.0
    # ... MySQL配置

  redis:
    image: redis:7-alpine
    # ... Redis配置
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: wolink-core
spec:
  replicas: 3
  selector:
    matchLabels:
      app: wolink-core
  template:
    metadata:
      labels:
        app: wolink-core
    spec:
      containers:
      - name: wolink-core
        image: wolink-core:latest
        env:
        - name: HOSTNAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: DB_PASSWORD
          valueFrom:
            secretKeyRef:
              name: wolink-secrets
              key: db-password
        volumeMounts:
        - name: config
          mountPath: /app/config
        - name: logs
          mountPath: /app/logs
      volumes:
      - name: config
        configMap:
          name: wolink-config
      - name: logs
        emptyDir: {}  # 或使用PVC共享存储
```

## 注意事项

1. **数据库连接池**：多节点需要调整连接池大小
   - 单节点：25连接
   - 3节点：每节点50连接（总共150）

2. **Redis连接池**：同样需要调整
   - 建议每节点50连接

3. **日志收集**：
   - 使用Fluentd/Logstash收集各节点日志
   - 或写入共享存储（NFS/S3）

4. **会话保持**：
   - 如果有WebSocket连接，需要会话保持
   - 或使用Redis存储会话状态

5. **健康检查**：
   - 添加 `/health` 端点
   - 监控数据库、Redis连接状态

## 性能优化建议

1. **读写分离**：数据库主从复制，读操作走从库
2. **Redis集群**：使用Redis Cluster或Sentinel
3. **连接复用**：合理配置连接池
4. **缓存策略**：充分利用Redis缓存热点数据
5. **异步处理**：所有写操作尽量异步

## 监控指标

需要监控的关键指标：
- 各节点QPS/TPS
- 数据库连接池使用率
- Redis连接和内存使用
- 队列长度和消费延迟
- 响应时间P50/P95/P99
- 错误率

## 总结

当前架构已经**基本支持分布式部署**，主要改进点：
1. ✅ CommunicationLogger需要添加Redis同步能力
2. ✅ 添加节点标识和管理
3. ⚠️ 模型配置可以考虑数据库化（可选）
4. ⚠️ 添加监控和运维工具（可选）

核心数据层（数据库+Redis）已经是共享的，这是分布式部署的基础，非常好！
