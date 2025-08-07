-- AI Gateway MySQL 初始化脚本

-- 设置字符集
SET NAMES utf8mb4;
SET CHARACTER SET utf8mb4;

-- 创建数据库（如果不存在）
CREATE DATABASE IF NOT EXISTS ai_gateway 
CHARACTER SET utf8mb4 
COLLATE utf8mb4_unicode_ci;

USE ai_gateway;

-- 优化 MySQL 配置
SET sql_mode = 'STRICT_TRANS_TABLES,NO_ZERO_DATE,NO_ZERO_IN_DATE,ERROR_FOR_DIVISION_BY_ZERO';

-- 创建索引优化查询性能
-- 这些索引会在 GORM 自动迁移后创建

-- 为 conversations 表创建复合索引
-- ALTER TABLE conversations ADD INDEX idx_dept_created (department_id, created_at);
-- ALTER TABLE conversations ADD INDEX idx_apikey_created (api_key_id, created_at);

-- 为 usage_logs 表创建索引
-- ALTER TABLE usage_logs ADD INDEX idx_dept_time (department_id, request_time);
-- ALTER TABLE usage_logs ADD INDEX idx_model_time (model_name, request_time);

-- 创建初始管理员部门（可选）
-- INSERT IGNORE INTO departments (name, created_at, updated_at) 
-- VALUES ('系统管理', NOW(), NOW());

FLUSH PRIVILEGES;