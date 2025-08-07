#!/bin/bash

# AI Gateway API 测试脚本

BASE_URL="http://localhost:8080"
API_KEY=""
DB_TYPE=${1:-mysql}  # 默认使用 MySQL

echo "=== AI Gateway API 测试 (数据库: $DB_TYPE) ==="

# 1. 健康检查
echo "1. 健康检查..."
curl -s "$BASE_URL/health" | jq .

# 2. 创建部门
echo -e "\n2. 创建部门..."
DEPT_RESPONSE=$(curl -s -X POST "$BASE_URL/admin/departments" \
  -H "Content-Type: application/json" \
  -d '{"name": "测试部门"}')
echo $DEPT_RESPONSE | jq .
DEPT_ID=$(echo $DEPT_RESPONSE | jq -r '.id')

# 3. 创建API密钥
echo -e "\n3. 创建API密钥..."
KEY_RESPONSE=$(curl -s -X POST "$BASE_URL/admin/api-keys" \
  -H "Content-Type: application/json" \
  -d "{\"department_id\": $DEPT_ID, \"name\": \"测试密钥\"}")
echo $KEY_RESPONSE | jq .
API_KEY=$(echo $KEY_RESPONSE | jq -r '.key_id')

# 4. 创建模型配置
echo -e "\n4. 创建模型配置..."
curl -s -X POST "$BASE_URL/admin/models" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "gpt-3.5-turbo",
    "provider": "openai",
    "model_type": "text",
    "endpoint": "https://api.openai.com/v1/chat/completions",
    "api_key": "your-openai-key",
    "max_tokens": 2048,
    "temperature": 0.7,
    "weight": 1,
    "is_enabled": true,
    "is_healthy": true
  }' | jq .

# 5. 获取模型列表
echo -e "\n5. 获取模型列表..."
curl -s -X GET "$BASE_URL/v1/models" \
  -H "Authorization: Bearer $API_KEY" | jq .

# 6. 测试聊天完成（如果有有效的API密钥）
if [ "$API_KEY" != "" ] && [ "$API_KEY" != "null" ]; then
  echo -e "\n6. 测试聊天完成..."
  curl -s -X POST "$BASE_URL/v1/chat/completions" \
    -H "Authorization: Bearer $API_KEY" \
    -H "Content-Type: application/json" \
    -d '{
      "model": "gpt-3.5-turbo",
      "messages": [
        {"role": "user", "content": "你好，这是一个测试消息，我的手机号是13812345678"}
      ],
      "temperature": 0.7,
      "max_tokens": 100
    }' | jq .
fi

# 7. 查看使用统计
echo -e "\n7. 查看使用统计..."
curl -s -X GET "$BASE_URL/admin/usage/stats?department_id=$DEPT_ID" | jq .

# 8. 查看对话记录
echo -e "\n8. 查看对话记录..."
curl -s -X GET "$BASE_URL/admin/conversations?department_id=$DEPT_ID&limit=5" | jq .

echo -e "\n=== 测试完成 ==="