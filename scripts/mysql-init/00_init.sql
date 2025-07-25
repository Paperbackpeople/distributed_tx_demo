/* ---------- 订单库 ---------- */
CREATE DATABASE IF NOT EXISTS order_db CHARSET utf8mb4;

USE order_db;
CREATE TABLE orders(
  id         BIGINT PRIMARY KEY AUTO_INCREMENT,
  gid        VARCHAR(64) NOT NULL UNIQUE,
  user_id    BIGINT,
  total_amt  INT,
  status     ENUM('PENDING','CONFIRMED','CANCELED') NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
CREATE TABLE order_item(
  order_id   BIGINT,
  product_id BIGINT,
  qty        INT,
  price      INT,
  PRIMARY KEY(order_id, product_id)
);

/* ---------- 库存库 ---------- */
CREATE DATABASE IF NOT EXISTS stock_db CHARSET utf8mb4;

USE stock_db;
CREATE TABLE product(
  id    BIGINT PRIMARY KEY,
  name  VARCHAR(64),
  price INT
);
CREATE TABLE stock(
  product_id BIGINT PRIMARY KEY,
  available  INT,
  reserved   INT
);
CREATE TABLE stock_log(
  id         BIGINT PRIMARY KEY AUTO_INCREMENT,
  gid        VARCHAR(64) NOT NULL UNIQUE,
  product_id BIGINT,
  qty        INT,
  status     ENUM('RESERVED','CONFIRMED','RELEASED') NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);

/* ---------- 支付库 ---------- */
CREATE DATABASE IF NOT EXISTS pay_db CHARSET utf8mb4;

USE pay_db;
CREATE TABLE account(
  user_id  BIGINT PRIMARY KEY,
  balance  INT,
  reserved INT
);
CREATE TABLE payment(
  id         BIGINT AUTO_INCREMENT PRIMARY KEY,
  gid        VARCHAR(64) UNIQUE,
  user_id    BIGINT,
  amount     INT,
  status     ENUM('PENDING','CONFIRMED','REFUNDED') NOT NULL,
  created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
  updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP
);
INSERT IGNORE INTO stock_db.stock (product_id, available, reserved) VALUES (1, 100, 0);
INSERT IGNORE INTO pay_db.account (user_id, balance, reserved) VALUES (1, 1000, 0);