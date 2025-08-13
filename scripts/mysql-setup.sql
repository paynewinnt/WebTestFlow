-- MySQL权限修复SQL脚本
-- 请使用以下命令执行: mysql -u root -p < mysql-setup.sql

-- 查看当前用户认证方式
SELECT user,authentication_string,plugin,host FROM mysql.user WHERE user='root';

-- 修改root用户认证方式为密码认证
ALTER USER 'root'@'localhost' IDENTIFIED WITH mysql_native_password BY '123456';

-- 创建专用数据库用户
CREATE USER IF NOT EXISTS 'webtestflow'@'localhost' IDENTIFIED BY '123456';

-- 授予权限
GRANT ALL PRIVILEGES ON *.* TO 'webtestflow'@'localhost' WITH GRANT OPTION;

-- 创建数据库
CREATE DATABASE IF NOT EXISTS webtestflow CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- 授予数据库权限
GRANT ALL PRIVILEGES ON webtestflow.* TO 'webtestflow'@'localhost';

-- 刷新权限
FLUSH PRIVILEGES;

-- 显示用户信息
SELECT user,host,plugin FROM mysql.user WHERE user IN ('root','webtestflow');