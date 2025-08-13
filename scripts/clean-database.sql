-- WebTestFlow Database Clean Mode SQL Script
-- 用于将数据库恢复到纯净模式，只保留基础配置数据
-- 
-- 使用方法：
-- mysql -u root -p webtestflow < clean-database.sql
-- 或在MySQL命令行中执行：source clean-database.sql
--
-- ⚠️ 警告：此脚本将清除大部分业务数据，请谨慎使用！

-- ========================================
-- 开始清理数据
-- ========================================

-- 临时关闭外键检查
SET FOREIGN_KEY_CHECKS = 0;

-- ----------------------------------------
-- 1. 清空执行记录相关表
-- ----------------------------------------
TRUNCATE TABLE test_report_executions;
TRUNCATE TABLE test_reports;
TRUNCATE TABLE test_executions;
TRUNCATE TABLE screenshots;
TRUNCATE TABLE performance_metrics;

-- ----------------------------------------
-- 2. 清空测试用例和测试套件相关表
-- ----------------------------------------
TRUNCATE TABLE test_suite_cases;
TRUNCATE TABLE test_suites;
TRUNCATE TABLE test_cases;

-- ----------------------------------------
-- 3. 清空项目表
-- ----------------------------------------
TRUNCATE TABLE projects;

-- ----------------------------------------
-- 4. 保留admin用户，删除其他用户
-- ----------------------------------------
DELETE FROM users WHERE username != 'admin';

-- ----------------------------------------
-- 5. 保留环境表ID=2的数据，删除其他
-- ----------------------------------------
DELETE FROM environments WHERE id != 2;

-- ----------------------------------------
-- 6. 设备表保留所有数据（不删除）
-- ----------------------------------------
-- 设备表数据全部保留

-- 重新启用外键检查
SET FOREIGN_KEY_CHECKS = 1;

-- ========================================
-- 显示清理结果
-- ========================================
SELECT '=== 数据清理完成 ===' as '执行结果';

SELECT 
    '数据表统计' as '信息类型',
    CONCAT(
        'Users: ', (SELECT COUNT(*) FROM users), ', ',
        'Environments: ', (SELECT COUNT(*) FROM environments), ', ',
        'Devices: ', (SELECT COUNT(*) FROM devices), ', ',
        'Projects: ', (SELECT COUNT(*) FROM projects), ', ',
        'Test Cases: ', (SELECT COUNT(*) FROM test_cases), ', ',
        'Test Suites: ', (SELECT COUNT(*) FROM test_suites), ', ',
        'Test Executions: ', (SELECT COUNT(*) FROM test_executions), ', ',
        'Test Reports: ', (SELECT COUNT(*) FROM test_reports)
    ) as '详情';

-- 显示保留的用户信息
SELECT '=== 保留的用户 ===' as '信息';
SELECT id, username, email, status FROM users;

-- 显示保留的环境信息
SELECT '=== 保留的环境 ===' as '信息';
SELECT id, name, type FROM environments;

-- 显示保留的设备信息
SELECT '=== 保留的设备 ===' as '信息';
SELECT id, name, CONCAT(width, 'x', height) as resolution, 
       CASE WHEN is_default = 1 THEN '是' ELSE '否' END as '默认设备' 
FROM devices;

SELECT '=== 数据库已恢复到纯净模式 ===' as '执行结果';