# PaddleOCR-VL-1.5 API 参数指南

## 📋 概述

本文档对比说明两种PaddleOCR-VL-1.5 API的调用方式：
1. **百度官方API** - 文档中描述的原始接口
2. **本地oMLX服务** - OpenAI兼容接口（当前网关使用）

---

## 🔄 两种API格式对比

### 1. 百度官方API格式

**接口地址**: `POST /layout-parsing`

**认证方式**: `Authorization: token {TOKEN}`

**请求格式**:
```json
{
  "file": "<base64编码的文件内容>",
  "fileType": 1,  // 0=PDF, 1=图片
  "useDocOrientationClassify": false,
  "useDocUnwarping": false,
  "useChartRecognition": false
}
```

**特点**:
- ✅ 专业的文档解析功能
- ✅ 支持文档方向分类、展平、图表识别
- ✅ 输出Markdown格式
- ❌ 需要base64编码文件
- ❌ 非OpenAI兼容

**示例代码**:
```python
import base64
import requests

file_path = "document.png"
with open(file_path, "rb") as f:
    file_data = base64.b64encode(f.read()).decode("ascii")

payload = {
    "file": file_data,
    "fileType": 1,
    "useDocOrientationClassify": False,
    "useDocUnwarping": False,
    "useChartRecognition": False
}

response = requests.post(
    "https://aistudio.baidu.com/svc/online/paddleocr/layout-parsing",
    headers={"Authorization": f"token {TOKEN}", "Content-Type": "application/json"},
    json=payload
)

result = response.json()["result"]
markdown_text = result["layoutParsingResults"][0]["markdown"]["text"]
```

---

### 2. 本地oMLX服务（OpenAI兼容格式）

**接口地址**: `POST /v1/chat/completions`

**认证方式**: `Authorization: Bearer {API_KEY}`

**请求格式**:
```json
{
  "model": "PaddleOCR-VL-1.5",
  "messages": [{
    "role": "user",
    "content": [
      {"type": "text", "text": "识别图片中的所有文字"},
      {"type": "image_url", "image_url": {"url": "图片URL"}}
    ]
  }],
  "temperature": 0.1,
  "max_tokens": 2048
}
```

**特点**:
- ✅ OpenAI API兼容
- ✅ 支持图片URL或base64
- ✅ 多轮对话支持
- ✅ 网关可直接转发
- ❌ 功能相对简化
- ❌ 不支持文档解析高级功能

**示例代码**:
```python
import requests

payload = {
    "model": "PaddleOCR-VL-1.5",
    "messages": [{
        "role": "user",
        "content": [
            {"type": "text", "text": "识别图片中的所有文字"},
            {"type": "image_url", "image_url": {"url": "https://example.com/image.png"}}
        ]
    }],
    "temperature": 0.1,
    "max_tokens": 2048
}

response = requests.post(
    "http://127.0.0.1:12345/v1/chat/completions",
    headers={"Authorization": "Bearer lingting", "Content-Type": "application/json"},
    json=payload
)

text = response.json()["choices"][0]["message"]["content"]
```

---

## 🎯 Prompt优化建议

### 基础识别
```json
{"type": "text", "text": "识别图片中的所有文字"}
```

### 详细识别（推荐）
```json
{"type": "text", "text": "请详细识别图片中的所有文字内容，包括标题、正文、列表、注释等，按原始布局输出"}
```

### 表格识别
```json
{"type": "text", "text": "识别图片中的表格，以Markdown表格格式输出"}
```

### 结构化输出
```json
{"type": "text", "text": "识别图片中的文字，按照以下格式输出：\n标题：[标题内容]\n正文：[正文内容]\n列表：[列表项]"}
```

---

## 📊 功能对比表

| 功能 | 百度官方API | 本地oMLX服务 |
|------|------------|--------------|
| 文字识别 | ✅ | ✅ |
| 文档解析 | ✅ | ⚠️ 有限 |
| 方向分类 | ✅ | ❌ |
| 文档展平 | ✅ | ❌ |
| 图表识别 | ✅ | ⚠️ 有限 |
| PDF支持 | ✅ | ❌ |
| Markdown输出 | ✅ | ❌ |
| OpenAI兼容 | ❌ | ✅ |
| 多轮对话 | ❌ | ✅ |
| 图片URL | ❌ | ✅ |

---

## 🔧 网关配置

当前wolink-core网关已配置使用**本地oMLX服务**：

```yaml
# configs/models/paddleocr.yaml
id: PaddleOCR-VL-1.5
name: PaddleOCR-VL-1.5
conn_config:
  base_url: "http://127.0.0.1:12345"
  api_key: "lingting"
  model: "PaddleOCR-VL-1.5"
```

**调用方式**:
```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer test-ocr-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "PaddleOCR-VL-1.5",
    "messages": [{
      "role": "user",
      "content": [
        {"type": "text", "text": "识别图片文字"},
        {"type": "image_url", "image_url": {"url": "图片URL"}}
      ]
    }]
  }'
```

---

## 💡 使用建议

1. **简单文字识别**: 使用本地oMLX服务即可
2. **复杂文档解析**: 需要使用百度官方API
3. **生产环境**: 根据功能需求选择合适的接口
4. **成本考虑**: 本地服务免费，官方API按调用量计费

---

## 📝 注意事项

1. 本地oMLX服务不支持百度官方API的 `fileType`、`useDocOrientationClassify` 等参数
2. 网关已升级支持OpenAI多模态格式，可直接传递图片URL
3. 对于复杂文档解析需求，建议直接调用百度官方API

---

## 📚 参考文档

- [PaddleOCR-VL-1.5 官方文档](https://ai.baidu.com/ai-doc/AISTUDIO/Cmkz2m0ma)
- [OpenAI Vision API](https://platform.openai.com/docs/guides/vision)
- [wolink-core 网关文档](./README.md)
