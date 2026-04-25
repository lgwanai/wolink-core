# Research: Phase 5 - wolink-core 网关核心化重构

**Phase:** 5
**Goal:** 去掉管理员鉴权、部门管理等跟网关无关的功能，wolink-core仅提供最内核的网关能力。添加admin管理端通信接口，使用token认证。
**Research Date:** 2026-04-25

## 1. 当前架构分析

### 1.1 需要移除的功能

#### Admin认证系统 (需移除)
| 文件 | 功能 | 处理方式 |
|------|------|---------|
| `internal/api/middleware/admin_auth.go` | JWT认证中间件 | 删除 |
| `internal/api/handlers/admin_auth_handler.go` | 登录/登出/管理员CRUD | 删除 |
| `internal/services/admin_auth_service.go` | JWT生成/验证/会话管理 | 删除 |
| `init_admin.go` | 初始化管理员 | 删除 |
| `internal/models/models.go` | AdminUser, AdminSession模型 | 删除这些模型 |

#### 部门管理 (需移除)
| 文件 | 功能 | 处理方式 |
|------|------|---------|
| `internal/api/handlers/admin_handler.go` | CreateDepartment, ListDepartments | 移除这些方法 |
| `internal/models/models.go` | Department模型 | 删除 |

#### Admin路由 (需移除)
`internal/api/routes.go` 中的以下路由组需移除:
- `/admin/auth/*` - 登录登出
- `/admin/users/*` - 管理员用户管理
- `/admin/departments/*` - 部门管理

### 1.2 需要保留的功能

#### 网关核心功能 (保留)
- `/v1/*` - OpenAI兼容API (chat, embeddings, audio等)
- `/health`, `/ready`, `/metrics` - 健康检查和监控
- API Key认证 (`middleware.APIKeyAuth`)
- 插件系统 (`PluginService`)
- 模型管理 (`ModelConfig`, `ModelRegistry`)
- 使用统计 (`UsageLog`, `Conversation`)

#### 需要改造的功能
- API Key管理 - 目前依赖Department，需改为独立管理
- 使用统计 - 目前按department_id查询，需改为按api_key_id查询

## 2. 新增功能需求

### 2.1 Admin管理接口

需要新增一组接口供外部admin管理端调用，使用简单的token认证：

#### 节点状态接口
```
GET /admin/node/status
Response: {
  "node_id": "node-1",
  "status": "healthy",
  "uptime": "2h30m",
  "version": "1.0.0",
  "goroutines": 150,
  "memory_usage_mb": 256
}
```

#### 节点控制接口
```
POST /admin/node/restart
Response: { "message": "restart initiated", "graceful_shutdown": true }
```

#### 插件管理接口
```
GET    /admin/plugins          # 列出插件
POST   /admin/plugins/reload   # 重载插件
DELETE /admin/plugins/:protocol # 卸载插件
```

#### API Key管理接口 (改造)
```
GET    /admin/api-keys
POST   /admin/api-keys
PUT    /admin/api-keys/:id
DELETE /admin/api-keys/:id
```

#### 模型配置接口 (保留)
```
GET    /admin/models
POST   /admin/models
PUT    /admin/models/:id
DELETE /admin/models/:id
```

### 2.2 Token认证方案

推荐使用**静态Token**方案，原因：
1. 部署简单 - 1个admin管理N个节点，无需复杂的JWT管理
2. 安全足够 - Token只在内部网络使用，admin-to-node通信
3. 配置简单 - 在配置文件中设置admin_token

**实现方案：**
```yaml
# config.yaml
admin:
  token: "your-secure-admin-token-here"  # 至少32字符
```

**中间件实现：**
```go
func AdminTokenAuth(cfg *config.Config) gin.HandlerFunc {
    return func(c *gin.Context) {
        token := c.GetHeader("X-Admin-Token")
        if token == "" || token != cfg.Admin.Token {
            c.JSON(401, gin.H{"error": "unauthorized"})
            c.Abort()
            return
        }
        c.Next()
    }
}
```

## 3. 数据模型变更

### 3.1 需要删除的模型
```go
// 删除
type Department struct { ... }
type AdminUser struct { ... }
type AdminSession struct { ... }
```

### 3.2 需要修改的模型
```go
// APIKey 移除 DepartmentID，改为独立
type APIKey struct {
    ID           uint   `json:"id" gorm:"primaryKey"`
    // DepartmentID uint   // 删除此字段
    KeyID        string `json:"key_id"`
    KeySecret    string `json:"-"`
    Name         string `json:"name"`
    // ... 其他字段保留
}
```

### 3.3 需要添加的配置
```go
type Config struct {
    // ... 现有配置
    Admin AdminConfig `mapstructure:"admin"`
}

type AdminConfig struct {
    Token string `mapstructure:"token"` // Admin API Token
}
```

## 4. 接口改造详情

### 4.1 移除的接口
| 路由 | 说明 |
|------|------|
| POST /admin/auth/login | 管理员登录 |
| POST /admin/auth/logout | 管理员登出 |
| GET /admin/profile | 管理员信息 |
| POST /admin/users | 创建管理员 |
| GET /admin/users | 管理员列表 |
| PUT /admin/users/:id | 更新管理员 |
| DELETE /admin/users/:id | 删除管理员 |
| POST /admin/departments | 创建部门 |
| GET /admin/departments | 部门列表 |

### 4.2 新增的接口
| 路由 | 说明 |
|------|------|
| GET /admin/node/status | 节点状态 |
| POST /admin/node/restart | 节点重启 |

### 4.3 保留但改造的接口
| 路由 | 改造内容 |
|------|---------|
| GET /admin/api-keys | 移除department关联 |
| POST /admin/api-keys | 移除department_id参数 |
| GET /admin/models | 保持不变 |
| GET /admin/plugins | 移动到/admin路由下 |
| GET /admin/usage/stats | 改为按api_key_id查询 |

## 5. 技术栈确认

基于现有代码分析：
- **Web框架**: Gin
- **ORM**: GORM
- **日志**: Logrus
- **配置**: Viper
- **认证**: golang-jwt/jwt (可移除，改用简单token)

### 依赖变更
```go
// 可移除的依赖
"github.com/golang-jwt/jwt/v5"  // 不再需要JWT
"golang.org/x/crypto/bcrypt"    // 不再需要密码哈希
```

## 6. 实现建议

### 6.1 执行顺序
1. **Wave 1**: 配置和模型层
   - 添加AdminConfig到配置
   - 删除Department, AdminUser, AdminSession模型
   - 修改APIKey模型（移除DepartmentID）
   
2. **Wave 2**: 服务层
   - 删除AdminAuthService
   - 创建NodeService（节点状态和控制）
   - 修改AuthService（移除department依赖）
   
3. **Wave 3**: 路由和处理器层
   - 删除admin_auth_handler
   - 删除admin_auth中间件
   - 创建admin_token中间件
   - 创建node_handler
   - 修改routes.go
   
4. **Wave 4**: 数据库迁移和测试
   - 数据库迁移脚本
   - 更新测试
   - 文档更新

### 6.2 数据库迁移注意
- 需要处理现有数据：APIKeys表中的department_id字段
- 建议先备份数据，然后：
  1. 删除admin_users, admin_sessions, departments表
  2. 从api_keys表删除department_id列
  3. 从conversations表删除department_id列

### 6.3 向后兼容
- `/v1/*` 接口保持不变
- 现有API Key继续有效（迁移后）
- 健康检查接口不变

## 7. 安全考虑

### 7.1 Admin Token安全
- Token长度至少32字符
- 生产环境必须通过环境变量配置：`AI_GATEWAY_ADMIN_TOKEN`
- Token不应出现在日志中
- Token应支持轮换机制（可通过配置更新）

### 7.2 网络安全
- Admin接口应仅在内网暴露
- 考虑添加IP白名单配置
- 重启接口应谨慎使用，考虑添加确认机制

## 8. 测试策略

### 8.1 单元测试
- AdminTokenAuth中间件测试
- NodeService测试
- 修改后的AuthService测试

### 8.2 集成测试
- Admin接口端到端测试
- Token认证测试
- 节点重启测试（mock）

## 9. 参考模式

### 9.1 节点状态模式
参考Kubernetes healthz pattern:
- 简单的状态字段
- 关键指标暴露
- 不依赖外部服务

### 9.2 Graceful Restart
参考：
- signal.Notify + shutdown timeout
- Drain in-flight requests
- Log before exit

## 10. 风险和缓解

| 风险 | 影响 | 缓解措施 |
|------|------|---------|
| 数据丢失 | 现有department数据丢失 | 迁移前备份，提供数据导出工具 |
| API变更 | 前端调用失败 | 明确版本说明，提供迁移指南 |
| Token泄露 | 安全风险 | 环境变量配置，日志脱敏 |
| 节点重启误操作 | 服务中断 | 添加确认机制，审计日志 |

---

## 研究结论

Phase 5 的核心任务是：
1. **移除**: Admin认证系统、部门管理、管理员用户管理
2. **简化**: API Key管理移除department依赖
3. **新增**: Admin Token认证、节点状态/控制接口
4. **保留**: 核心网关功能、插件系统、健康检查

预计工作量：4个plans，分4个waves执行。

**关键决策点：**
- [ ] Admin Token格式：静态Token (推荐) vs JWT
- [ ] 节点重启实现：直接退出 vs HTTP响应后退出
- [ ] API Key模型：完全移除department_id vs 保留但可选
