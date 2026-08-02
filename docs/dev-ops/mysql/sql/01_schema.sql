-- AI Desktop Assistant schema
CREATE DATABASE IF NOT EXISTS `ai_desktop_assistant` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
USE `ai_desktop_assistant`;

CREATE TABLE IF NOT EXISTS `chat_session` (
  `id`            VARCHAR(64)  NOT NULL COMMENT '会话ID',
  `agent_id`      VARCHAR(64)  NOT NULL DEFAULT 'desktop-agent',
  `user_id`       VARCHAR(64)  NOT NULL,
  `title`         VARCHAR(255) NOT NULL DEFAULT '',
  `status`        VARCHAR(32)  NOT NULL DEFAULT 'ACTIVE',
  `message_count` INT          NOT NULL DEFAULT 0,
  `token_used`    INT          NOT NULL DEFAULT 0,
  `working_dir`   VARCHAR(512) NOT NULL DEFAULT '',
  `metadata_json` JSON         NULL,
  `created_at`    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`    DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_user_status` (`user_id`, `status`),
  KEY `idx_updated` (`updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='会话';

CREATE TABLE IF NOT EXISTS `chat_message` (
  `id`           VARCHAR(64)  NOT NULL,
  `session_id`   VARCHAR(64)  NOT NULL,
  `role`         VARCHAR(32)  NOT NULL,
  `content`      MEDIUMTEXT   NOT NULL,
  `tool_name`    VARCHAR(128) NOT NULL DEFAULT '',
  `tool_call_id` VARCHAR(128) NOT NULL DEFAULT '',
  `step`         INT          NOT NULL DEFAULT 0,
  `token_count`  INT          NOT NULL DEFAULT 0,
  `priority`     INT          NOT NULL DEFAULT 1,
  `created_at`   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_session_created` (`session_id`, `created_at`),
  CONSTRAINT `fk_msg_session` FOREIGN KEY (`session_id`) REFERENCES `chat_session` (`id`) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='消息';

CREATE TABLE IF NOT EXISTS `chat_milestone` (
  `id`         BIGINT       NOT NULL AUTO_INCREMENT,
  `session_id` VARCHAR(64)  NOT NULL,
  `type`       VARCHAR(64)  NOT NULL,
  `content`    VARCHAR(1024) NOT NULL DEFAULT '',
  `step`       INT          NOT NULL DEFAULT 0,
  `created_at` DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_session_created` (`session_id`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='里程碑';

CREATE TABLE IF NOT EXISTS `core_memory` (
  `id`         BIGINT       NOT NULL AUTO_INCREMENT,
  `user_id`    VARCHAR(64)  NOT NULL,
  `session_id` VARCHAR(64)  NOT NULL DEFAULT '',
  `category`   VARCHAR(64)  NOT NULL DEFAULT 'general',
  `content`    TEXT         NOT NULL,
  `source`     VARCHAR(64)  NOT NULL DEFAULT '',
  `created_at` DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at` DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  KEY `idx_user` (`user_id`),
  KEY `idx_user_cat` (`user_id`, `category`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='长期记忆';

CREATE TABLE IF NOT EXISTS `mcp_server_config` (
  `id`           BIGINT       NOT NULL AUTO_INCREMENT,
  `name`         VARCHAR(64)  NOT NULL,
  `transport`    VARCHAR(32)  NOT NULL COMMENT 'stdio|sse|http',
  `command`      VARCHAR(512) NOT NULL DEFAULT '',
  `args_json`    JSON         NULL,
  `env_json`     JSON         NULL,
  `url`          VARCHAR(1024) NOT NULL DEFAULT '',
  `enabled`      TINYINT(1)   NOT NULL DEFAULT 1,
  `timeout_sec`  INT          NOT NULL DEFAULT 60,
  `created_at`   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  `updated_at`   DATETIME(3)  NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='MCP 服务配置';
