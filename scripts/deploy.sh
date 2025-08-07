#!/bin/bash

# AI Gateway 快速部署脚本

DB_TYPE=${1:-mysql}  # 默认使用 MySQL
ACTION=${2:-up}      # 默认启动服务

echo "=== AI Gateway 快速部署 ==="
echo "数据库类型: $DB_TYPE"
echo "操作: $ACTION"

case $DB_TYPE in
    "mysql")
        COMPOSE_FILE="docker-compose.yml"
        ;;
    "postgres"|"postgresql")
        COMPOSE_FILE="docker-compose.postgres.yml"
        ;;
    *)
        echo "不支持的数据库类型: $DB_TYPE"
        echo "支持的类型: mysql, postgres"
        exit 1
        ;;
esac

case $ACTION in
    "up"|"start")
        echo "启动服务..."
        docker-compose -f $COMPOSE_FILE up -d
        echo "等待服务启动..."
        sleep 10
        echo "检查服务状态..."
        docker-compose -f $COMPOSE_FILE ps
        echo ""
        echo "服务已启动！"
        echo "API 地址: http://localhost:8080"
        echo "健康检查: curl http://localhost:8080/health"
        ;;
    "down"|"stop")
        echo "停止服务..."
        docker-compose -f $COMPOSE_FILE down
        ;;
    "restart")
        echo "重启服务..."
        docker-compose -f $COMPOSE_FILE restart
        ;;
    "logs")
        echo "查看日志..."
        docker-compose -f $COMPOSE_FILE logs -f wolink-core
        ;;
    "clean")
        echo "清理所有数据..."
        docker-compose -f $COMPOSE_FILE down -v
        docker system prune -f
        ;;
    *)
        echo "不支持的操作: $ACTION"
        echo "支持的操作: up, down, restart, logs, clean"
        exit 1
        ;;
esac