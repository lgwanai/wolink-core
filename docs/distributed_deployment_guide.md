# WoLink Core 分布式部署指南

## 概述

WoLink Core 从设计之初就支持分布式部署，核心数据层（数据库 + Redis）采用共享架构，多个节点可以并行处理请求。

## 架构设计

### 核心原则

```
                    ┌─────────────────┐
                    │  Load Balancer  │
                    │ (Nginx/ALB/SLB) │
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
              ┌──────────────┴──────────────┐
              │                             │
        ┌─────▼─────┐               ┌──────▼──────┐
        │  MySQL    │               │    Redis    │
        │ (主从)    │               │  (集群)     │
        └───────────┘               └─────────────┘
```

### 组件分布式支持状态

| 组件 | 分布式支持 | 说明 |
|------|-----------|------|
| 数据库 | ✅ 完全支持 | 使用共享MySQL/PostgreSQL |
| Redis缓存 | ✅ 完全支持 | 使用共享Redis |
| API Key验证 | ✅ 完全支持 | 基于Redis缓存 |
| 速率限制 | ✅ 完全支持 | 基于Redis计数器 |
| 队列服务 | ✅ 完全支持 | Redis队列，多节点消费 |
| 通信日志 | ✅ 支持 | 本地+Redis双写模式 |
| 模型配置 | ⚠️ 部分支持 | 需要手动同步配置文件 |

## 快速开始

### 1. 基础配置

每个节点使用相同的配置文件，但节点ID不同：

```yaml
# config.yaml
server:
  port: "8080"
  mode: "release"

# 节点配置（每个节点不同）
node:
  id: "node-01"  # 使用环境变量: ${HOSTNAME}
  region: "cn-east-1"

# 共享数据库
database:
  type: "mysql"
  host: "mysql-cluster.internal"
  port: 3306
  user: "wolink"
  password: "${DB_PASSWORD}"
  dbname: "wolink_core"

# 共享Redis
redis:
  host: "redis-cluster.internal"
  port: 6379
  password: "${REDIS_PASSWORD}"
  db: 0

# 通信日志配置
communication_log:
  enabled: true
  mode: "local"  # local: 本地存储
  storage_path: "./logs/communications"
  # 可选：同步到Redis集中收集
  enable_remote_sync: true
  redis_queue_key: "wolink:comm_logs"
  max_file_size_mb: 100
  flush_interval_ms: 1000

# 连接池配置（多节点需要调整）
infrastructure:
  database_pool:
    max_open_conns: 50  # 单节点连接数
    max_idle_conns: 20
    conn_max_lifetime: "5m"
  redis_pool:
    pool_size: 50  # 单节点连接数
    min_idle_conns: 10
    conn_max_lifetime: "5m"
```

### 2. 环境变量

推荐使用环境变量区分节点：

```bash
# Node 1
export HOSTNAME=wolink-node-1
export DB_PASSWORD=secret123
export REDIS_PASSWORD=redis123

# Node 2
export HOSTNAME=wolink-node-2
export DB_PASSWORD=secret123
export REDIS_PASSWORD=redis123
```

### 3. 启动节点

```bash
# 每个节点执行相同命令
./wolink-core --config configs/config.yaml
```

## Docker Compose 部署

```yaml
version: '3.8'

services:
  # 负载均衡（可选）
  nginx:
    image: nginx:alpine
    ports:
      - "80:80"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf
    depends_on:
      - wolink-node-1
      - wolink-node-2
      - wolink-node-3

  # WoLink 节点 1
  wolink-node-1:
    image: wolink-core:latest
    environment:
      - HOSTNAME=wolink-node-1
      - DB_PASSWORD=secret123
      - REDIS_PASSWORD=redis123
    volumes:
      - ./config:/app/config
      - node1-logs:/app/logs
    ports:
      - "8081:8080"
    depends_on:
      - mysql
      - redis

  # WoLink 节点 2
  wolink-node-2:
    image: wolink-core:latest
    environment:
      - HOSTNAME=wolink-node-2
      - DB_PASSWORD=secret123
      - REDIS_PASSWORD=redis123
    volumes:
      - ./config:/app/config
      - node2-logs:/app/logs
    ports:
      - "8082:8080"
    depends_on:
      - mysql
      - redis

  # WoLink 节点 3
  wolink-node-3:
    image: wolink-core:latest
    environment:
      - HOSTNAME=wolink-node-3
      - DB_PASSWORD=secret123
      - REDIS_PASSWORD=redis123
    volumes:
      - ./config:/app/config
      - node3-logs:/app/logs
    ports:
      - "8083:8080"
    depends_on:
      - mysql
      - redis

  # MySQL
  mysql:
    image: mysql:8.0
    environment:
      MYSQL_ROOT_PASSWORD: root123
      MYSQL_DATABASE: wolink_core
      MYSQL_USER: wolink
      MYSQL_PASSWORD: secret123
    volumes:
      - mysql-data:/var/lib/mysql
    ports:
      - "3306:3306"

  # Redis
  redis:
    image: redis:7-alpine
    command: redis-server --requirepass redis123
    volumes:
      - redis-data:/data
    ports:
      - "6379:6379"

volumes:
  mysql-data:
  redis-data:
  node1-logs:
  node2-logs:
  node3-logs:
```

## Kubernetes 部署

### 1. ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: wolink-config
data:
  config.yaml: |
    server:
      port: "8080"
      mode: "release"
    
    node:
      id: "${HOSTNAME}"
      region: "cn-east-1"
    
    database:
      type: "mysql"
      host: "mysql-service.default.svc.cluster.local"
      port: 3306
      user: "wolink"
      password: "${DB_PASSWORD}"
      dbname: "wolink_core"
    
    redis:
      host: "redis-service.default.svc.cluster.local"
      port: 6379
      password: "${REDIS_PASSWORD}"
      db: 0
    
    communication_log:
      enabled: true
      mode: "local"
      storage_path: "/data/logs/communications"
      enable_remote_sync: true
      redis_queue_key: "wolink:comm_logs"
      max_file_size_mb: 100
      flush_interval_ms: 1000
    
    infrastructure:
      database_pool:
        max_open_conns: 50
        max_idle_conns: 20
      redis_pool:
        pool_size: 50
        min_idle_conns: 10
```

### 2. Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: wolink-core
  labels:
    app: wolink-core
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
        ports:
        - containerPort: 8080
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
        - name: REDIS_PASSWORD
          valueFrom:
            secretKeyRef:
              name: wolink-secrets
              key: redis-password
        volumeMounts:
        - name: config
          mountPath: /app/config
          subPath: config.yaml
        - name: logs
          mountPath: /data/logs
        resources:
          requests:
            cpu: 500m
            memory: 512Mi
          limits:
            cpu: 2000m
            memory: 2Gi
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: config
        configMap:
          name: wolink-config
      - name: logs
        emptyDir: {}
```

### 3. Service

```yaml
apiVersion: v1
kind: Service
metadata:
  name: wolink-service
spec:
  selector:
    app: wolink-core
  ports:
  - protocol: TCP
    port: 80
    targetPort: 8080
  type: ClusterIP
```

### 4. HPA（水平自动扩展）

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: wolink-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: wolink-core
  minReplicas: 3
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

## 通信日志集中收集

多节点部署时，建议使用Redis同步功能集中收集日志：

### 方案1：使用Logstash收集

```ruby
# logstash.conf
input {
  redis {
    host => "redis-cluster.internal"
    port => 6379
    password => "redis123"
    key => "wolink:comm_logs"
    data_type => "list"
    codec => "json"
  }
}

output {
  elasticsearch {
    hosts => ["http://elasticsearch:9200"]
    index => "wolink-comm-logs-%{+YYYY.MM.dd}"
  }
  
  # 或写入文件
  file {
    path => "/data/logs/communications/all-nodes/%{+YYYY-MM-dd-HH}.log.jsonl"
    codec => json_lines
  }
}
```

### 方案2：使用Fluentd

```xml
# fluent.conf
<source>
  @type redis
  host redis-cluster.internal
  port 6379
  password redis123
  key wolink:comm_logs
  data_type list
  tag wolink.comm_logs
</source>

<match wolink.comm_logs>
  @type elasticsearch
  host elasticsearch
  port 9200
  index_name wolink-comm-logs
  type_name _doc
</match>
```

## 监控和运维

### 1. 健康检查

```bash
# 检查节点健康状态
curl http://node-ip:8080/health

# 检查就绪状态
curl http://node-ip:8080/ready

# Prometheus指标
curl http://node-ip:8080/metrics
```

### 2. 关键监控指标

- **QPS/TPS**：各节点的请求处理速率
- **响应时间**：P50, P95, P99延迟
- **错误率**：5xx错误占比
- **数据库连接池**：使用率、等待时间
- **Redis连接**：连接数、内存使用
- **队列长度**：待处理任务数量
- **磁盘使用**：日志文件大小

### 3. 日志聚合

推荐使用集中式日志系统：
- ELK Stack（Elasticsearch + Logstash + Kibana）
- EFK Stack（Fluentd代替Logstash）
- Loki + Grafana

## 性能调优

### 1. 数据库连接池

总连接数 = 节点数 × 每节点连接数

```yaml
# 3节点集群，总共建议150连接
infrastructure:
  database_pool:
    max_open_conns: 50  # 每节点50，总共150
    max_idle_conns: 20
```

### 2. Redis连接池

```yaml
infrastructure:
  redis_pool:
    pool_size: 50  # 每节点50连接
    min_idle_conns: 10
```

### 3. 负载均衡策略

如果使用Nginx：

```nginx
upstream wolink_backend {
    # 轮询
    server node1:8080;
    server node2:8080;
    server node3:8080;
    
    # 或加权轮询
    # server node1:8080 weight=3;
    # server node2:8080 weight=2;
    # server node3:8080 weight=2;
    
    # 或最少连接
    # least_conn;
}

server {
    listen 80;
    
    location / {
        proxy_pass http://wolink_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## 故障处理

### 1. 单节点故障

- 负载均衡器自动剔除故障节点
- 其他节点继续服务
- 修复后重新加入集群

### 2. 数据库故障

- 使用主从复制
- 应用层配置多个数据库地址
- 故障自动切换

### 3. Redis故障

- 使用Redis Sentinel或Cluster
- 自动故障转移
- 降级到数据库查询

## 扩缩容

### 扩展（增加节点）

1. 启动新节点（使用相同配置）
2. 负载均衡器自动发现
3. 无需重启现有节点

### 缩容（减少节点）

1. 从负载均衡器移除节点
2. 等待现有请求完成
3. 停止节点

## 最佳实践

1. **无状态设计**：确保节点无状态，可随时替换
2. **健康检查**：配置liveness和readiness探针
3. **资源限制**：设置CPU和内存限制
4. **日志收集**：集中收集所有节点日志
5. **监控告警**：设置关键指标告警阈值
6. **定期备份**：数据库和Redis定期备份
7. **灰度发布**：使用滚动更新策略
8. **配置管理**：使用ConfigMap或外部配置中心

## 安全建议

1. **网络安全**：节点间通信使用VPC内网
2. **加密传输**：数据库和Redis启用TLS
3. **密码管理**：使用Secret管理敏感信息
4. **访问控制**：限制数据库和Redis访问权限
5. **审计日志**：记录所有管理操作

## 总结

WoLink Core 的分布式部署非常简单：
- ✅ 核心组件天然支持分布式
- ✅ 无状态设计，水平扩展容易
- ✅ 共享数据库和Redis保证一致性
- ✅ 灵活的日志收集方案

只需确保：
1. 所有节点连接到相同的数据库和Redis
2. 配置负载均衡器
3. 设置合理的连接池大小
4. 集中收集日志和监控

就可以轻松实现高可用、高性能的分布式部署！
