#!/bin/bash

# PostgreSQL 数据库初始化脚本

DB_HOST="localhost"
DB_PORT="5432"
DB_USER="wuliang"
DB_NAME="wolink_core"

echo "正在连接到 PostgreSQL..."
echo "主机: $DB_HOST:$DB_PORT"
echo "用户: $DB_USER"
echo "数据库: $DB_NAME"

# 检查 psql 是否可用
if ! command -v psql &> /dev/null; then
    echo "错误: 未找到 psql 命令"
    echo "请安装 PostgreSQL 客户端: brew install postgresql"
    exit 1
fi

# 创建数据库（如果不存在）
echo ""
echo "检查数据库是否存在..."
psql -h $DB_HOST -p $DB_PORT -U $DB_USER -lqt | cut -d \| -f 1 | grep -qw $DB_NAME

if [ $? -eq 0 ]; then
    echo "✓ 数据库 '$DB_NAME' 已存在"
else
    echo "创建数据库 '$DB_NAME'..."
    psql -h $DB_HOST -p $DB_PORT -U $DB_USER -c "CREATE DATABASE $DB_NAME;"
    
    if [ $? -eq 0 ]; then
        echo "✓ 数据库创建成功"
    else
        echo "✗ 数据库创建失败"
        exit 1
    fi
fi

# 连接数据库并测试
echo ""
echo "测试数据库连接..."
psql -h $DB_HOST -p $DB_PORT -U $DB_USER -d $DB_NAME -c "SELECT version();"

if [ $? -eq 0 ]; then
    echo ""
    echo "✓ 数据库连接成功！"
    echo ""
    echo "数据库信息："
    echo "  主机: $DB_HOST:$DB_PORT"
    echo "  用户: $DB_USER"
    echo "  数据库: $DB_NAME"
    echo ""
    echo "现在可以启动 WoLink Core 服务："
    echo "  go run cmd/main.go"
else
    echo ""
    echo "✗ 数据库连接失败"
    exit 1
fi
