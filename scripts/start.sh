#!/bin/bash

# AI Gateway 启动脚本

echo "Starting AI Gateway..."

# 检查依赖
echo "Checking dependencies..."

# 检查数据库客户端
if command -v mysql &> /dev/null; then
    echo "MySQL client found."
    DB_TYPE="mysql"
elif command -v psql &> /dev/null; then
    echo "PostgreSQL client found."
    DB_TYPE="postgres"
else
    echo "No database client found. Please install MySQL or PostgreSQL."
    exit 1
fi

# 检查 Redis
if ! command -v redis-cli &> /dev/null; then
    echo "Redis client not found. Please install Redis."
    exit 1
fi

# 设置环境变量
export AI_GATEWAY_SERVER_PORT=8080
export AI_GATEWAY_DATABASE_TYPE=mysql
export AI_GATEWAY_DATABASE_HOST=localhost
export AI_GATEWAY_DATABASE_PORT=3306
export AI_GATEWAY_DATABASE_USER=root
export AI_GATEWAY_DATABASE_PASSWORD=password
export AI_GATEWAY_DATABASE_DBNAME=ai_gateway
export AI_GATEWAY_DATABASE_CHARSET=utf8mb4
export AI_GATEWAY_DATABASE_PARSETIME=true
export AI_GATEWAY_DATABASE_LOC=Local
export AI_GATEWAY_REDIS_HOST=localhost
export AI_GATEWAY_REDIS_PORT=6379

# 根据检测到的数据库类型调整配置
if [ "$DB_TYPE" = "postgres" ]; then
    export AI_GATEWAY_DATABASE_TYPE=postgres
    export AI_GATEWAY_DATABASE_PORT=5432
    export AI_GATEWAY_DATABASE_USER=postgres
    export AI_GATEWAY_DATABASE_SSLMODE=disable
fi

# 启动应用
echo "Starting AI Gateway on port $AI_GATEWAY_SERVER_PORT..."
go run cmd/main.go