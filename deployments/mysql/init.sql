-- Create database if not exists
CREATE DATABASE IF NOT EXISTS webtestflow CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;

-- Use the database
USE webtestflow;

-- Grant privileges to user
GRANT ALL PRIVILEGES ON webtestflow.* TO 'webtestflow'@'%';
FLUSH PRIVILEGES;