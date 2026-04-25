-- Phase 5 Migration: Remove Gateway-Unrelated Models
-- This script removes Department, AdminUser, AdminSession tables
-- and updates API keys and conversations to remove department dependencies
-- 
-- IMPORTANT: Run this AFTER deploying Phase 5 code changes
-- Backup your database before running this migration!

-- MySQL Version
-- ==============

-- Step 1: Drop foreign key constraints first
ALTER TABLE api_keys DROP FOREIGN KEY IF EXISTS fk_api_keys_department;
ALTER TABLE conversations DROP FOREIGN KEY IF EXISTS fk_conversations_department;
ALTER TABLE usage_logs DROP FOREIGN KEY IF EXISTS fk_usage_logs_department;
ALTER TABLE admin_sessions DROP FOREIGN KEY IF EXISTS fk_admin_sessions_user;

-- Step 2: Remove department_id columns from existing tables
ALTER TABLE api_keys DROP COLUMN IF EXISTS department_id;
ALTER TABLE conversations DROP COLUMN IF EXISTS department_id;
ALTER TABLE usage_logs DROP COLUMN IF EXISTS department_id;

-- Step 3: Drop admin-related tables
DROP TABLE IF EXISTS admin_sessions;
DROP TABLE IF EXISTS admin_users;
DROP TABLE IF EXISTS departments;

-- PostgreSQL Version
-- ==================

-- Uncomment below for PostgreSQL:

-- Step 1: Drop foreign key constraints
-- ALTER TABLE api_keys DROP CONSTRAINT IF EXISTS fk_api_keys_department;
-- ALTER TABLE conversations DROP CONSTRAINT IF EXISTS fk_conversations_department;
-- ALTER TABLE usage_logs DROP CONSTRAINT IF EXISTS fk_usage_logs_department;
-- ALTER TABLE admin_sessions DROP CONSTRAINT IF EXISTS fk_admin_sessions_user;

-- Step 2: Remove department_id columns
-- ALTER TABLE api_keys DROP COLUMN IF EXISTS department_id;
-- ALTER TABLE conversations DROP COLUMN IF EXISTS department_id;
-- ALTER TABLE usage_logs DROP COLUMN IF EXISTS department_id;

-- Step 3: Drop admin-related tables
-- DROP TABLE IF EXISTS admin_sessions;
-- DROP TABLE IF EXISTS admin_users;
-- DROP TABLE IF EXISTS departments;

-- Migration complete
SELECT 'Phase 5 migration complete. Gateway-unrelated tables removed.' AS status;
