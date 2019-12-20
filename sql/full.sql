CREATE TABLE IF NOT EXISTS `traces` (
  `id`        INT(11) NOT NULL AUTO_INCREMENT,
  `trace_id` varchar(100) DEFAULT NULL,
  `span_id` bigint(20) DEFAULT NULL,
  `span_hash` bigint(20) DEFAULT NULL,
  `parent_id` bigint(20) DEFAULT NULL,
  `operation_name` varchar(128) DEFAULT NULL,
  `flags` int(11) DEFAULT NULL,
  `start_time` bigint(20) DEFAULT NULL,
  `duration` bigint(20) DEFAULT NULL,
  `tags` text,
  `logs` text,
  `refs` text,
  `process` text,
  `service_name` varchar(128) DEFAULT NULL,
  `http_code` int(11) DEFAULT 0,
  `error`  tinyint(1) DEFAULT 0,
  PRIMARY KEY (`id`),
  KEY `idx_trace_id` (`trace_id`),
  KEY `idx_service_name` (`service_name`),
  KEY `idx_operation_name` (`operation_name`),
  KEY `idx_tart_time` (`start_time`),
  KEY `idx_duration` (`duration`),
  KEY `idx_http_code` (`http_code`),
  KEY `idx_error` (`error`),
  KEY `idx_time_svc_operation` (`start_time`,`service_name`,`operation_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8; 

CREATE TABLE IF NOT EXISTS `operation_names` (
  `service_name` varchar(128) NOT NULL,
  `operation_name` varchar(128) NOT NULL,
  PRIMARY KEY (`service_name`,`operation_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;


CREATE TABLE IF NOT EXISTS `service_names` (
  `service_name` varchar(128) NOT NULL,
  PRIMARY KEY (`service_name`),
  UNIQUE KEY `service_name` (`service_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8;
