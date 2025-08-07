#!/bin/bash

# AI Gateway 性能测试脚本

BASE_URL="http://localhost:8080"
API_KEY="your-api-key-here"
CONCURRENT_USERS=10
REQUESTS_PER_USER=100

echo "=== AI Gateway 性能测试 ==="
echo "并发用户数: $CONCURRENT_USERS"
echo "每用户请求数: $REQUESTS_PER_USER"
echo "总请求数: $((CONCURRENT_USERS * REQUESTS_PER_USER))"

# 创建测试请求文件
cat > /tmp/chat_request.json << EOF
{
  "model": "gpt-3.5-turbo",
  "messages": [
    {"role": "user", "content": "这是一个性能测试请求，请简短回复"}
  ],
  "temperature": 0.7,
  "max_tokens": 50
}
EOF

# 使用 Apache Bench 进行压力测试
if command -v ab &> /dev/null; then
  echo -e "\n使用 Apache Bench 进行压力测试..."
  ab -n $((CONCURRENT_USERS * REQUESTS_PER_USER)) \
     -c $CONCURRENT_USERS \
     -H "Authorization: Bearer $API_KEY" \
     -H "Content-Type: application/json" \
     -p /tmp/chat_request.json \
     "$BASE_URL/v1/chat/completions"
else
  echo "Apache Bench (ab) 未安装，跳过压力测试"
  echo "安装方法: brew install httpie (macOS) 或 apt-get install apache2-utils (Ubuntu)"
fi

# 清理临时文件
rm -f /tmp/chat_request.json

echo -e "\n=== 性能测试完成 ==="