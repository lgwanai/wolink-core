#!/bin/bash

# 数据库初始化脚本

DB_TYPE=${1:-mysql}  # 默认使用 MySQL
DB_NAME="ai_gateway"

echo "=== 初始化 $DB_TYPE 数据库 ==="

case $DB_TYPE in
    "mysql")
        echo "创建 MySQL 数据库..."
        mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS $DB_NAME CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
        mysql -u root -p -e "SHOW DATABASES LIKE '$DB_NAME';"
        
        echo "MySQL 数据库创建完成！"
        echo "连接信息："
        echo "  Host: localhost"
        echo "  Port: 3306"
        echo "  Database: $DB_NAME"
        echo "  User: root"
        ;;
        
    "postgres"|"postgresql")
        echo "创建 PostgreSQL 数据库..."
        createdb $DB_NAME 2>/dev/null || echo "数据库可能已存在"
        psql -l | grep $DB_NAME
        
        echo "PostgreSQL 数据库创建完成！"
        echo "连接信息："
        echo "  Host: localhost"
        echo "  Port: 5432"
        echo "  Database: $DB_NAME"
        echo "  User: postgres"
        ;;
        
    *)
        echo "不支持的数据库类型: $DB_TYPE"
        echo "支持的类型: mysql, postgres"
        exit 1
        ;;
esac

echo ""
echo "请确保数据库服务正在运行，然后启动 AI Gateway："
echo "./scripts/start.sh"