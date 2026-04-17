# AI Gateway - 企业级大模型调用网关

## 项目概述

企业级 AI 调用网关，提供统一的大模型接口调用、API-Key 管理、敏感信息检测、负载均衡、监控等功能。

## 核心功能

1. **管理员认证系统**: JWT 认证，角色权限控制，安全的管理员登录/登出
2. **API-Key 管理**: 部门级别的 API-Key 管理，使用量控制，并发限制
3. **敏感信息检测**: 自动检测和替换敏感信息（手机号、身份证等）
4. **对话存储**: 完整对话记录，支持知识库构建和缓存优化
5. **负载均衡**: 多模型负载均衡，健康检查，自动故障切换
6. **统一接口**: 标准 OpenAI 接口，支持文本、多模态、Embedding 模型
7. **高性能**: 异步处理，零拷贝流式响应，热插拔插件架构
8. **扩展能力**: 多模型 PK，结果对比等高级功能

## 技术架构

- **后端**: Go + Gin + GORM + Redis
- **数据库**: PostgreSQL + Redis
- **消息队列**: Redis Streams
- **配置管理**: YAML 配置文件
- **插件系统**: 热插拔插件架构
- **监控**: Prometheus + Grafana

## 项目结构

```
wolink-core/
├── cmd/                    # 应用入口
│   └── main.go            # 主程序
├── internal/              # 内部包
│   ├── api/              # API 路由和处理器
│   │   ├── handlers/     # 请求处理器
│   │   ├── middleware/   # 中间件
│   │   └── routes.go     # 路由配置
│   ├── config/           # 配置管理
│   ├── models/           # 数据模型
│   ├── services/         # 业务逻辑
│   └── utils/            # 工具函数
├── configs/              # 配置文件
│   ├── config.yaml       # 主配置文件
│   ├── models/           # 模型配置目录
│   └── templates/        # 配置模板目录
├── scripts/              # 脚本文件
└── Dockerfile            # Docker 配置
```

## 快速开始

### 1. 环境准备

确保你的系统已安装：
- Go 1.21+
- MySQL 8.0+ 或 PostgreSQL 12+
- Redis 6+

### 2. 数据库设置

#### 使用 MySQL（推荐）
```bash
# 创建数据库
./scripts/init_db.sh mysql

# 或手动创建
mysql -u root -p -e "CREATE DATABASE ai_gateway CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
```

#### 使用 PostgreSQL
```bash
# 创建数据库
./scripts/init_db.sh postgres

# 或手动创建
createdb ai_gateway
```

#### 启动 Redis
```bash
redis-server
```

### 3. 配置文件

根据你的数据库类型选择配置文件：

#### MySQL 配置
```bash
cp configs/config.mysql.yaml configs/config.yaml
# 编辑 configs/config.yaml，设置数据库连接信息
```

#### PostgreSQL 配置
```bash
cp configs/config.postgres.yaml configs/config.yaml
# 编辑 configs/config.yaml，设置数据库连接信息
```

#### 主要配置项说明
```yaml
database:
  type: "mysql"          # 数据库类型：mysql 或 postgres
  host: "localhost"      # 数据库主机
  port: 3306            # 数据库端口（MySQL: 3306, PostgreSQL: 5432）
  user: "root"          # 数据库用户名
  password: "password"   # 数据库密码
  dbname: "ai_gateway"  # 数据库名称
  
  # MySQL 专用配置
  charset: "utf8mb4"    # 字符集
  parsetime: true       # 解析时间
  loc: "Local"         # 时区
  
  # PostgreSQL 专用配置
  sslmode: "disable"    # SSL 模式
```

### 4. 启动应用

```bash
# 安装依赖
go mod tidy

# 启动应用
./scripts/start.sh
# 或者直接运行
go run cmd/main.go
```

### 5. 管理员登录和初始化

```bash
# 管理员登录（默认账号：admin，密码：password）
curl -X POST http://localhost:8990/admin/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "password"}'

# 获取返回的 JWT token，用于后续管理接口调用
# 示例响应：{"token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."}

# 使用 token 创建部门
curl -X POST http://localhost:8990/admin/departments \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"name": "技术部"}'

# 创建API密钥
curl -X POST http://localhost:8990/admin/api-keys \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"department_id": 1, "name": "技术部测试密钥"}'
```

## API 使用示例

### ASR 语音转文本与逐字时间戳 (兼容 OpenAI)

支持将音频文件转录为文本，并可通过 oMLX 补丁支持强大的**强制对齐 (Forced Alignment)** 以获取逐字级别的时间戳 (`char_level_info`)。

```bash
curl -X POST http://localhost:8080/v1/audio/transcriptions \
  -H "Authorization: Bearer ak-your-api-key" \
  -F "file=@/path/to/your/audio.mp3" \
  -F "model=Qwen3-ASR-1.7B-8bit" \
  -F "response_format=verbose_json" \
  -F "timestamp_granularities[]=word" \
  -F "forced_aligner=Qwen3-ForcedAligner-0.6B"
```

> **oMLX 逐字时间戳补丁**：默认的 oMLX 底层服务不返回 `char_level_info`。如果你使用 macOS 上的 oMLX.app 作为本地模型后端，请运行本项目提供的 `patch/omlx_asr_patch/apply_patch.sh` 脚本来修改 oMLX 源码，使其支持强制对齐与逐字时间戳返回。详细说明请参考 `patch/omlx_asr_patch/README.md`。

### 聊天完成（兼容 OpenAI）

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer ak-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "你好，请介绍一下人工智能"}
    ],
    "temperature": 0.7,
    "max_tokens": 1000
  }'
```

### 流式响应

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer ak-your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "写一首关于春天的诗"}
    ],
    "stream": true
  }'
```

### 获取模型列表

```bash
curl -X GET http://localhost:8080/v1/models \
  -H "Authorization: Bearer ak-your-api-key"
```

## 管理接口

所有管理接口都需要 JWT 认证，请先登录获取 token。

### 管理员认证

```bash
# 登录
curl -X POST http://localhost:8990/admin/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username": "admin", "password": "password"}'

# 获取当前管理员信息
curl -X GET http://localhost:8990/admin/profile \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"

# 登出
curl -X POST http://localhost:8990/admin/auth/logout \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### 管理员用户管理（仅超级管理员）

```bash
# 获取管理员列表
curl -X GET http://localhost:8990/admin/users \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"

# 创建新管理员
curl -X POST http://localhost:8990/admin/users \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"username": "newadmin", "password": "password123", "name": "新管理员", "role": "admin"}'
```

### 查看使用统计

```bash
curl -X GET "http://localhost:8990/admin/usage/stats?department_id=1" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### 查看对话记录

```bash
curl -X GET "http://localhost:8990/admin/conversations?department_id=1&limit=10" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## 核心特性详解

### 1. API-Key 管理
- 部门级别的密钥管理
- 每日/每月使用量限制
- 并发请求控制
- Redis 缓存提升性能

### 2. 敏感信息检测
- 自动检测手机号、身份证号等敏感信息
- 可配置的正则表达式规则
- 自动替换敏感内容

### 3. 负载均衡
- 基于权重的模型选择
- 健康检查和自动故障切换
- 支持多个相同模型实例

### 4. 高性能设计
- 异步日志记录
- Redis 缓存热点数据
- 流式响应零拷贝
- 数据库连接池

### 5. 监控和统计
- 完整的对话记录
- 使用量统计
- 响应时间监控
- 敏感信息检测记录

## 扩展功能

### 插件系统
支持热插拔的模型调用插件，方便扩展新的大模型提供商。

### 多模型对比
一次请求可以同时调用多个模型，进行结果对比和优选。

### 缓存优化
基于对话内容的智能缓存，减少重复请求的 token 消耗。

## 部署

### 快速部署（推荐）

使用 Docker Compose 一键部署：

```bash
# MySQL 版本（推荐）
./scripts/deploy.sh mysql up

# PostgreSQL 版本
./scripts/deploy.sh postgres up

# 查看服务状态
docker-compose ps

# 查看日志
./scripts/deploy.sh mysql logs

# 停止服务
./scripts/deploy.sh mysql down
```

### Docker 部署

#### 使用 MySQL
```bash
# 构建镜像
docker build -t wolink-core .

# 运行容器（MySQL）
docker run -d \
  --name wolink-core \
  -p 8080:8080 \
  -e AI_GATEWAY_DATABASE_TYPE=mysql \
  -e AI_GATEWAY_DATABASE_HOST=your-mysql-host \
  -e AI_GATEWAY_DATABASE_PORT=3306 \
  -e AI_GATEWAY_DATABASE_USER=root \
  -e AI_GATEWAY_DATABASE_PASSWORD=your-password \
  -e AI_GATEWAY_REDIS_HOST=your-redis-host \
  wolink-core
```

#### 使用 PostgreSQL
```bash
# 运行容器（PostgreSQL）
docker run -d \
  --name wolink-core \
  -p 8080:8080 \
  -e AI_GATEWAY_DATABASE_TYPE=postgres \
  -e AI_GATEWAY_DATABASE_HOST=your-postgres-host \
  -e AI_GATEWAY_DATABASE_PORT=5432 \
  -e AI_GATEWAY_DATABASE_USER=postgres \
  -e AI_GATEWAY_DATABASE_PASSWORD=your-password \
  -e AI_GATEWAY_REDIS_HOST=your-redis-host \
  wolink-core
```

#### Docker Compose 示例
```yaml
version: '3.8'
services:
  wolink-core:
    build: .
    ports:
      - "8080:8080"
    environment:
      - AI_GATEWAY_DATABASE_TYPE=mysql
      - AI_GATEWAY_DATABASE_HOST=mysql
      - AI_GATEWAY_DATABASE_USER=root
      - AI_GATEWAY_DATABASE_PASSWORD=password
      - AI_GATEWAY_REDIS_HOST=redis
    depends_on:
      - mysql
      - redis

  mysql:
    image: mysql:8.0
    environment:
      - MYSQL_ROOT_PASSWORD=password
      - MYSQL_DATABASE=ai_gateway
    ports:
      - "3306:3306"
    volumes:
      - mysql_data:/var/lib/mysql

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"

volumes:
  mysql_data:
```

### 生产环境配置

1. 使用环境变量覆盖敏感配置
2. 配置反向代理（Nginx）
3. 设置监控和日志收集
4. 配置数据库备份策略