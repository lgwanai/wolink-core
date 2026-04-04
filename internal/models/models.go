package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Department 部门表
type Department struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"type:varchar(100);uniqueIndex;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	
	APIKeys []APIKey `json:"api_keys" gorm:"foreignKey:DepartmentID"`
}

// APIKey 部门API密钥表
type APIKey struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	DepartmentID uint      `json:"department_id" gorm:"not null"`
	KeyID        string    `json:"key_id" gorm:"type:varchar(100);uniqueIndex;not null"` // 对外显示的key
	KeySecret    string    `json:"-" gorm:"type:varchar(100);not null"`                  // 实际的密钥，不返回给前端
	Name         string    `json:"name" gorm:"type:varchar(100)"`
	Status       string    `json:"status" gorm:"type:varchar(20);default:'active'"` // active, disabled
	
	// 使用限制
	DailyLimit    int64 `json:"daily_limit" gorm:"default:10000"`    // 每日调用限制
	MonthlyLimit  int64 `json:"monthly_limit" gorm:"default:300000"` // 每月调用限制
	ConcurrentLimit int `json:"concurrent_limit" gorm:"default:10"`  // 并发限制
	
	// 使用统计
	DailyUsage   int64 `json:"daily_usage" gorm:"default:0"`
	MonthlyUsage int64 `json:"monthly_usage" gorm:"default:0"`
	TotalUsage   int64 `json:"total_usage" gorm:"default:0"`
	
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	
	Department Department `json:"department" gorm:"foreignKey:DepartmentID"`
}

// ModelRegistry 模型注册表（只存储映射关系）
type ModelRegistry struct {
	ID         uint   `json:"id" gorm:"primaryKey"`
	ConfigID   string `json:"config_id" gorm:"type:varchar(50);uniqueIndex;not null"` // 配置文件中的ID
	Name       string `json:"name" gorm:"type:varchar(100);index;not null"`           // 模型名称
	ConfigFile string `json:"config_file" gorm:"type:varchar(200);not null"`          // 配置文件名
	
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ModelConfig 运行时模型配置（从配置文件加载）
type ModelConfig struct {
	// 基本信息
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	IconURI     string                 `json:"icon_uri"`
	IconURL     string                 `json:"icon_url"`
	Description map[string]string      `json:"description"`
	
	// 元数据
	Protocol    string                 `json:"protocol"`
	Capability  CapabilityConfig       `json:"capability"`
	ConnConfig  ConnectionConfig       `json:"conn_config"`
	Parameters  []ParameterConfig      `json:"parameters"`
	Status      int                    `json:"status"`
}

// APIKeyModelMapping API密钥与模型的映射关系
type APIKeyModelMapping struct {
	ID              uint `json:"id" gorm:"primaryKey"`
	APIKeyID        uint `json:"api_key_id" gorm:"not null;index"`
	ModelRegistryID uint `json:"model_registry_id" gorm:"not null;index"`
	
	// 路由配置
	RouteType string `json:"route_type" gorm:"type:varchar(20);default:'random'"` // random, round_robin, weighted
	Priority  int    `json:"priority" gorm:"default:0"`                           // 优先级
	
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	
	APIKey        APIKey        `json:"api_key" gorm:"foreignKey:APIKeyID"`
	ModelRegistry ModelRegistry `json:"model_registry" gorm:"foreignKey:ModelRegistryID"`
}

// Conversation 对话记录表
type Conversation struct {
	ID           string    `json:"id" gorm:"type:varchar(50);primaryKey"` // UUID
	APIKeyID     uint      `json:"api_key_id" gorm:"not null"`
	DepartmentID uint      `json:"department_id" gorm:"not null"`
	ModelName    string    `json:"model_name" gorm:"type:varchar(100);not null"`
	
	// 请求信息
	RequestID    string `json:"request_id" gorm:"type:varchar(50);index"`
	UserMessage  string `json:"user_message" gorm:"type:text"`
	SystemPrompt string `json:"system_prompt" gorm:"type:text"`
	
	// 响应信息
	AssistantMessage string `json:"assistant_message" gorm:"type:text"`
	TokensUsed       int    `json:"tokens_used"`
	ResponseTime     int64  `json:"response_time"` // 毫秒
	
	// 敏感信息检测
	HasSensitiveInfo bool   `json:"has_sensitive_info" gorm:"default:false"`
	SensitiveTypes   string `json:"sensitive_types" gorm:"type:varchar(500)"` // JSON数组，存储检测到的敏感信息类型
	
	CreatedAt time.Time `json:"created_at" gorm:"index"`
	UpdatedAt time.Time `json:"updated_at"`
	
	APIKey     APIKey     `json:"api_key" gorm:"foreignKey:APIKeyID"`
	Department Department `json:"department" gorm:"foreignKey:DepartmentID"`
}

// UsageLog 使用日志表（用于异步记录）
type UsageLog struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	APIKeyID     uint      `json:"api_key_id" gorm:"index"`
	DepartmentID uint      `json:"department_id" gorm:"index"`
	ModelName    string    `json:"model_name" gorm:"type:varchar(100);index"`
	TokensUsed   int       `json:"tokens_used"`
	RequestTime  time.Time `json:"request_time" gorm:"index"`
	ResponseTime int64     `json:"response_time"`
	Status       string    `json:"status" gorm:"type:varchar(20)"` // success, error
	ErrorMessage string    `json:"error_message" gorm:"type:text"`
	
	CreatedAt time.Time `json:"created_at"`
}

// AdminUser 管理员用户表
type AdminUser struct {
	ID       uint   `json:"id" gorm:"primaryKey"`
	Username string `json:"username" gorm:"type:varchar(50);uniqueIndex;not null"`
	Password string `json:"-" gorm:"type:varchar(255);not null"` // 密码哈希，不返回给前端
	Email    string `json:"email" gorm:"type:varchar(100);uniqueIndex"`
	Name     string `json:"name" gorm:"type:varchar(100)"`
	Role     string `json:"role" gorm:"type:varchar(20);default:'admin'"` // admin, super_admin
	Status   string `json:"status" gorm:"type:varchar(20);default:'active'"` // active, disabled
	
	// 最后登录信息
	LastLoginAt *time.Time `json:"last_login_at"`
	LastLoginIP string     `json:"last_login_ip" gorm:"type:varchar(45)"`
	
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AdminSession 管理员会话表（用于token管理）
type AdminSession struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	AdminID   uint      `json:"admin_id" gorm:"not null;index"`
	Token     string    `json:"-" gorm:"type:varchar(500);uniqueIndex;not null"` // JWT token哈希
	ExpiresAt time.Time `json:"expires_at" gorm:"index"`
	CreatedAt time.Time `json:"created_at"`
	
	AdminUser AdminUser `json:"admin_user" gorm:"foreignKey:AdminID"`
}

// LoginRequest 登录请求结构
type LoginRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=8,max=128"`
}

// CreateAdminRequest 创建管理员请求结构
type CreateAdminRequest struct {
	Username string `json:"username" binding:"required,min=3,max=50,alphanum"`
	Password string `json:"password" binding:"required,min=8,max=128"`
	Email    string `json:"email" binding:"required,email"`
	Name     string `json:"name" binding:"required,min=1,max=100"`
	Role     string `json:"role" binding:"omitempty,oneof=admin super_admin"`
}

// ChangePasswordRequest 修改密码请求结构
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required,min=8,max=128"`
	NewPassword string `json:"new_password" binding:"required,min=8,max=128"`
}

// LoginResponse 登录响应结构
type LoginResponse struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expires_at"`
	User      AdminUser `json:"user"`
}

// BeforeCreate 在创建对话记录前生成UUID
func (c *Conversation) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}

// OpenAI 兼容的请求和响应结构

type ChatCompletionRequest struct {
	Model       string                 `json:"model" binding:"required"`
	Messages    []ChatMessage          `json:"messages" binding:"required,min=1,dive"`
	Temperature *float32               `json:"temperature,omitempty" binding:"omitempty,min=0,max=2"`
	MaxTokens   *int                   `json:"max_tokens,omitempty" binding:"omitempty,min=1,max=128000"`
	Stream      bool                   `json:"stream,omitempty"`
	User        string                 `json:"user,omitempty"`
	Extra       map[string]interface{} `json:"-"` // 额外参数
}

type ChatMessage struct {
	Role    string `json:"role" binding:"required,oneof=system user assistant"` // system, user, assistant
	Content string `json:"content" binding:"required,min=1"`
}

type ChatCompletionResponse struct {
	ID      string                 `json:"id"`
	Object  string                 `json:"object"`
	Created int64                  `json:"created"`
	Model   string                 `json:"model"`
	Choices []ChatCompletionChoice `json:"choices"`
	Usage   Usage                  `json:"usage"`
}

type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// 流式响应
type ChatCompletionStreamResponse struct {
	ID      string                       `json:"id"`
	Object  string                       `json:"object"`
	Created int64                        `json:"created"`
	Model   string                       `json:"model"`
	Choices []ChatCompletionStreamChoice `json:"choices"`
}

type ChatCompletionStreamChoice struct {
	Index int                      `json:"index"`
	Delta ChatCompletionStreamDelta `json:"delta"`
	FinishReason *string            `json:"finish_reason"`
}

type ChatCompletionStreamDelta struct {
	Role    string `json:"role,omitempty"`
	Content string `json:"content,omitempty"`
}// 模型配置文件结构

type ModelConfigFile struct {
	ID          string                 `yaml:"id"`
	Name        string                 `yaml:"name"`
	IconURI     string                 `yaml:"icon_uri"`
	IconURL     string                 `yaml:"icon_url"`
	Description map[string]string      `yaml:"description"`
	Parameters  []ParameterConfig      `yaml:"default_parameters"`
	Meta        MetaConfig             `yaml:"meta"`
	ConnConfig  ConnectionConfig       `yaml:"conn_config"`
	Status      int                    `yaml:"status"`
}

type ParameterConfig struct {
	Name       string                 `yaml:"name"`
	Label      map[string]string      `yaml:"label"`
	Desc       map[string]string      `yaml:"desc"`
	Type       string                 `yaml:"type"`
	Min        string                 `yaml:"min"`
	Max        string                 `yaml:"max"`
	DefaultVal map[string]interface{} `yaml:"default_val"`
	Precision  int                    `yaml:"precision"`
	Options    []OptionConfig         `yaml:"options"`
	Style      StyleConfig            `yaml:"style"`
}

type OptionConfig struct {
	Label string `yaml:"label"`
	Value string `yaml:"value"`
}

type StyleConfig struct {
	Widget string            `yaml:"widget"`
	Label  map[string]string `yaml:"label"`
}

type MetaConfig struct {
	Protocol   string         `yaml:"protocol"`
	Capability CapabilityConfig `yaml:"capability"`
}

type CapabilityConfig struct {
	FunctionCall   bool     `yaml:"function_call"`
	InputModal     []string `yaml:"input_modal"`
	InputTokens    int      `yaml:"input_tokens"`
	JSONMode       bool     `yaml:"json_mode"`
	MaxTokens      int      `yaml:"max_tokens"`
	OutputModal    []string `yaml:"output_modal"`
	OutputTokens   int      `yaml:"output_tokens"`
	PrefixCaching  bool     `yaml:"prefix_caching"`
	Reasoning      bool     `yaml:"reasoning"`
	PrefillResponse bool    `yaml:"prefill_response"`
}

type ConnectionConfig struct {
	BaseURL          string                 `yaml:"base_url"`
	APIKey           string                 `yaml:"api_key"`
	Timeout          string                 `yaml:"timeout"`
	Model            string                 `yaml:"model"`
	Temperature      float64                `yaml:"temperature"`
	FrequencyPenalty float64                `yaml:"frequency_penalty"`
	PresencePenalty  float64                `yaml:"presence_penalty"`
	MaxTokens        int                    `yaml:"max_tokens"`
	TopP             float64                `yaml:"top_p"`
	TopK             int                    `yaml:"top_k"`
	Stop             []string               `yaml:"stop"`
	Custom           map[string]interface{} `yaml:",inline"`
}