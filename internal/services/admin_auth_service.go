package services

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"wolink-core/internal/config"
	"wolink-core/internal/models"

	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AdminAuthService struct {
	db     *gorm.DB
	logger *logrus.Logger
	config *config.Config
}

type JWTClaims struct {
	AdminID  uint   `json:"admin_id"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func NewAdminAuthService(db *gorm.DB, logger *logrus.Logger, cfg *config.Config) *AdminAuthService {
	return &AdminAuthService{
		db:     db,
		logger: logger,
		config: cfg,
	}
}

// Login 管理员登录
func (s *AdminAuthService) Login(username, password, clientIP string) (*models.LoginResponse, error) {
	// 查找管理员用户
	var admin models.AdminUser
	if err := s.db.Where("username = ? AND status = 'active'", username).First(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户名或密码错误")
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	// 验证密码
	if err := bcrypt.CompareHashAndPassword([]byte(admin.Password), []byte(password)); err != nil {
		return nil, errors.New("用户名或密码错误")
	}

	// 生成JWT token
	token, expiresAt, err := s.generateJWT(&admin)
	if err != nil {
		return nil, fmt.Errorf("生成token失败: %w", err)
	}

	// 保存会话信息
	tokenHash := s.hashToken(token)
	session := &models.AdminSession{
		AdminID:   admin.ID,
		Token:     tokenHash,
		ExpiresAt: expiresAt,
	}

	if err := s.db.Create(session).Error; err != nil {
		s.logger.Errorf("保存会话失败: %v", err)
		// 不影响登录流程，继续执行
	}

	// 更新最后登录信息
	now := time.Now()
	s.db.Model(&admin).Updates(map[string]interface{}{
		"last_login_at": &now,
		"last_login_ip": clientIP,
	})

	// 清理过期会话
	go s.cleanExpiredSessions()

	s.logger.Infof("管理员 %s 登录成功，IP: %s", username, clientIP)

	return &models.LoginResponse{
		Token:     token,
		ExpiresAt: expiresAt,
		User:      admin,
	}, nil
}

// ValidateToken 验证JWT token
func (s *AdminAuthService) ValidateToken(tokenString string) (*models.AdminUser, error) {
	// 解析JWT token
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(s.config.Security.JWTSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("token解析失败: %w", err)
	}

	claims, ok := token.Claims.(*JWTClaims)
	if !ok || !token.Valid {
		return nil, errors.New("无效的token")
	}

	// 检查token是否在数据库中存在且未过期
	tokenHash := s.hashToken(tokenString)
	var session models.AdminSession
	if err := s.db.Where("token = ? AND expires_at > ?", tokenHash, time.Now()).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("token已过期或无效")
		}
		return nil, fmt.Errorf("验证token失败: %w", err)
	}

	// 获取管理员信息
	var admin models.AdminUser
	if err := s.db.Where("id = ? AND status = 'active'", claims.AdminID).First(&admin).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("用户不存在或已被禁用")
		}
		return nil, fmt.Errorf("查询用户失败: %w", err)
	}

	return &admin, nil
}

// Logout 管理员登出
func (s *AdminAuthService) Logout(tokenString string) error {
	tokenHash := s.hashToken(tokenString)
	return s.db.Where("token = ?", tokenHash).Delete(&models.AdminSession{}).Error
}

// CreateAdminUser 创建管理员用户
func (s *AdminAuthService) CreateAdminUser(username, password, email, name, role string) (*models.AdminUser, error) {
	// 检查用户名是否已存在
	var count int64
	s.db.Model(&models.AdminUser{}).Where("username = ?", username).Count(&count)
	if count > 0 {
		return nil, errors.New("用户名已存在")
	}

	// 检查邮箱是否已存在
	if email != "" {
		s.db.Model(&models.AdminUser{}).Where("email = ?", email).Count(&count)
		if count > 0 {
			return nil, errors.New("邮箱已存在")
		}
	}

	// 加密密码
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("密码加密失败: %w", err)
	}

	admin := &models.AdminUser{
		Username: username,
		Password: string(hashedPassword),
		Email:    email,
		Name:     name,
		Role:     role,
		Status:   "active",
	}

	if err := s.db.Create(admin).Error; err != nil {
		return nil, fmt.Errorf("创建用户失败: %w", err)
	}

	s.logger.Infof("创建管理员用户成功: %s", username)
	return admin, nil
}

// generateJWT 生成JWT token
func (s *AdminAuthService) generateJWT(admin *models.AdminUser) (string, time.Time, error) {
	expiresAt := time.Now().Add(24 * time.Hour) // 24小时过期

	claims := &JWTClaims{
		AdminID:  admin.ID,
		Username: admin.Username,
		Role:     admin.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "wolink-core",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(s.config.Security.JWTSecret))
	if err != nil {
		return "", time.Time{}, err
	}

	return tokenString, expiresAt, nil
}

// hashToken 对token进行哈希处理
func (s *AdminAuthService) hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// cleanExpiredSessions 清理过期会话
func (s *AdminAuthService) cleanExpiredSessions() {
	result := s.db.Where("expires_at < ?", time.Now()).Delete(&models.AdminSession{})
	if result.Error != nil {
		s.logger.Errorf("清理过期会话失败: %v", result.Error)
	} else if result.RowsAffected > 0 {
		s.logger.Infof("清理了 %d 个过期会话", result.RowsAffected)
	}
}

// GetAdminUsers 获取管理员用户列表
func (s *AdminAuthService) GetAdminUsers() ([]models.AdminUser, error) {
	var admins []models.AdminUser
	err := s.db.Find(&admins).Error
	return admins, err
}

// UpdateAdminUser 更新管理员用户
func (s *AdminAuthService) UpdateAdminUser(id uint, updates map[string]interface{}) error {
	// 如果更新密码，需要加密
	if password, ok := updates["password"]; ok {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password.(string)), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("密码加密失败: %w", err)
		}
		updates["password"] = string(hashedPassword)
	}

	return s.db.Model(&models.AdminUser{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteAdminUser 删除管理员用户
func (s *AdminAuthService) DeleteAdminUser(id uint) error {
	// 删除用户的所有会话
	s.db.Where("admin_id = ?", id).Delete(&models.AdminSession{})
	// 删除用户
	return s.db.Delete(&models.AdminUser{}, id).Error
}