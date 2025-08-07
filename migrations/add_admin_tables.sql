-- 创建管理员用户表
CREATE TABLE IF NOT EXISTS admin_users (
    id INT AUTO_INCREMENT PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE COMMENT '用户名',
    password VARCHAR(255) NOT NULL COMMENT '密码哈希',
    email VARCHAR(100) COMMENT '邮箱',
    name VARCHAR(100) NOT NULL COMMENT '姓名',
    role ENUM('super_admin', 'admin', 'operator') NOT NULL DEFAULT 'operator' COMMENT '角色',
    status ENUM('active', 'inactive') NOT NULL DEFAULT 'active' COMMENT '状态',
    last_login_at TIMESTAMP NULL COMMENT '最后登录时间',
    last_login_ip VARCHAR(45) COMMENT '最后登录IP',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='管理员用户表';

-- 创建管理员会话表
CREATE TABLE IF NOT EXISTS admin_sessions (
    id INT AUTO_INCREMENT PRIMARY KEY,
    admin_id INT NOT NULL COMMENT '管理员ID',
    token VARCHAR(255) NOT NULL COMMENT 'Token哈希',
    expires_at TIMESTAMP NOT NULL COMMENT '过期时间',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
    INDEX idx_admin_id (admin_id),
    INDEX idx_token (token),
    INDEX idx_expires_at (expires_at),
    FOREIGN KEY (admin_id) REFERENCES admin_users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='管理员会话表';

-- 插入默认超级管理员账户（用户名: admin, 密码: admin123）
-- 密码哈希是 bcrypt 加密的 "admin123"
INSERT IGNORE INTO admin_users (username, password, name, role, status) VALUES 
('admin', '$2a$10$92IXUNpkjO0rOQ5byMi.Ye4oKoEa3Ro9llC/.og/at2.uheWG/igi', '系统管理员', 'super_admin', 'active');

-- 创建索引优化查询性能（如果不存在）
ALTER TABLE admin_users ADD INDEX idx_admin_users_username (username);
ALTER TABLE admin_users ADD INDEX idx_admin_users_email (email);
ALTER TABLE admin_users ADD INDEX idx_admin_users_status (status);
ALTER TABLE admin_users ADD INDEX idx_admin_users_role (role);