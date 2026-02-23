CREATE TABLE `device` (
                          `id` varchar(50) COLLATE utf8mb4_general_ci NOT NULL,
                          `name` varchar(50) COLLATE utf8mb4_general_ci DEFAULT NULL,
                          `enabled` tinyint(1) DEFAULT NULL,
                          `public_key` text COLLATE utf8mb4_general_ci,
                          `private_key` text COLLATE utf8mb4_general_ci,
                          `created_at` datetime DEFAULT NULL,
                          PRIMARY KEY (`id`),
                          UNIQUE KEY `device_name_uindex` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci

CREATE TABLE `network` (
                           `id` varchar(50) COLLATE utf8mb4_general_ci NOT NULL,
                           `name` varchar(50) COLLATE utf8mb4_general_ci NOT NULL,
                           `enabled` tinyint(1) DEFAULT NULL,
                           `cidr` varchar(100) COLLATE utf8mb4_general_ci NOT NULL,
                           `mtu` int NOT NULL,
                           `description` varchar(200) COLLATE utf8mb4_general_ci DEFAULT NULL,
                           `extra` json DEFAULT NULL,
                           PRIMARY KEY (`id`),
                           UNIQUE KEY `network_name_uindex` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci

CREATE TABLE `network_device` (
                                  `id` int NOT NULL AUTO_INCREMENT,
                                  `device_id` varchar(50) COLLATE utf8mb4_general_ci DEFAULT NULL,
                                  `network_id` varchar(50) COLLATE utf8mb4_general_ci DEFAULT NULL,
                                  `quota` int DEFAULT NULL,
                                  `bandwidth` int DEFAULT NULL COMMENT 'quota id',
                                  `settings` json DEFAULT NULL,
                                  PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=3 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci

CREATE TABLE `quota` (
                         `id` int NOT NULL AUTO_INCREMENT,
                         `name` varchar(20) COLLATE utf8mb4_general_ci NOT NULL,
                         `type` varchar(50) COLLATE utf8mb4_general_ci DEFAULT NULL,
                         `unit` varchar(50) COLLATE utf8mb4_general_ci NOT NULL,
                         `value` int NOT NULL,
                         `period` varchar(20) COLLATE utf8mb4_general_ci DEFAULT NULL,
                         PRIMARY KEY (`id`),
                         UNIQUE KEY `quota_name_uindex` (`name`)
) ENGINE=InnoDB AUTO_INCREMENT=4 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci

CREATE TABLE `quota_flow` (
                              `id` int NOT NULL AUTO_INCREMENT,
                              `quota` int DEFAULT NULL,
                              `usage` bigint NOT NULL,
                              `target` varchar(50) COLLATE utf8mb4_general_ci DEFAULT NULL,
                              `target_type` varchar(20) COLLATE utf8mb4_general_ci DEFAULT NULL,
                              `record_start` bigint DEFAULT NULL,
                              `record_end` bigint DEFAULT NULL,
                              PRIMARY KEY (`id`)
) ENGINE=InnoDB AUTO_INCREMENT=28 DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_general_ci

