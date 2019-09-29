DROP TABLE IF EXISTS `devices`;
CREATE TABLE `devices` (
  `id` int(10) unsigned NOT NULL AUTO_INCREMENT,
  `machine_id` varchar(32) CHARACTER SET ascii COLLATE ascii_bin NOT NULL COMMENT 'contents of /etc/machine-id',
  `name` text CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci COMMENT 'human readable name for the machine if any',
  `created_at` timestamp NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq-machine_id` (`machine_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci;

DROP TABLE IF EXISTS `barcodes`;
CREATE TABLE `barcodes` (
  `id` bigint(20) unsigned NOT NULL AUTO_INCREMENT,
  `deviceid` int(10) unsigned NOT NULL,
  `barcode` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'the barcode',
  `direction` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'ingress/egress',
  `currier_service` text CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL COMMENT 'ingress/egress postfix',
  `created_at` bigint(20) NOT NULL COMMENT 'timestamp of scanning (UTC, unix timestamp, usec accuracy)',
  `timestamp` timestamp NOT NULL ON UPDATE CURRENT_TIMESTAMP COMMENT 'timestamp of database entry (seconds accuracy)',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uq-createdat` (`created_at`),
  KEY `ix-direction_timestamp` (`direction`(20),`timestamp`),
  KEY `deviceid` (`deviceid`),
  CONSTRAINT `barcodes_ibfk_1` FOREIGN KEY (`deviceid`) REFERENCES `devices` (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- barcode-scanner user must use ssl
-- ALTER USER 'barcode-scanner'@'%' REQUIRE SSL;
-- barcode-scanner user can only select/insert from the devices table:
-- GRANT SELECT, INSERT ON `barcode-scanner`.devices TO 'barcode-scanner'@'%';
-- and can only insert into the barcodes table
-- GRANT INSERT ON `barcode-scanner`.barcodes TO 'barcode-scanner'@'%';
