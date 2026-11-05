/*
 * Tencent is pleased to support the open source community by making Polaris available.
 *
 * Copyright (C) 2019 THL A29 Limited, a Tencent company. All rights reserved.
 *
 * Licensed under the BSD 3-Clause License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://opensource.org/licenses/BSD-3-Clause
 *
 * Unless required by applicable law or agreed to in writing, software distributed
 * under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR
 * CONDITIONS OF ANY KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 */
SET
    SQL_MODE = "NO_AUTO_VALUE_ON_ZERO";

SET
    time_zone = "+00:00";

--
-- Database: `pole_server`
--
CREATE DATABASE IF NOT EXISTS `pole_server` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_bin;

USE `pole_server`;

/* 服务实例 */
-- --------------------------------------------------------
--
-- Table structure `instance`
--
CREATE TABLE
    `instance` (
        `id` VARCHAR(128) NOT NULL COMMENT 'Unique ID',
        `service_id` VARCHAR(32) NOT NULL COMMENT 'Service ID',
        `vpc_id` VARCHAR(64) DEFAULT NULL COMMENT 'VPC ID',
        `host` VARCHAR(128) NOT NULL COMMENT 'instance Host Information',
        `port` INT (11) NOT NULL COMMENT 'instance port information',
        `protocol` VARCHAR(32) DEFAULT NULL COMMENT 'Listening protocols for corresponding ports, such as TPC, UDP, GRPC, DUBBO, etc.',
        `version` VARCHAR(32) DEFAULT NULL COMMENT 'The version of the instance can be used for version routing',
        `health_status` TINYINT (4) NOT NULL DEFAULT '1' COMMENT 'The health status of the instance, 1 is health, 0 is unhealthy',
        `isolate` TINYINT (4) NOT NULL DEFAULT '0' COMMENT 'Example isolation status flag, 0 is not isolated, 1 is isolated',
        `weight` SMALLINT (6) NOT NULL DEFAULT '100' COMMENT 'The weight of the instance is mainly used for LoadBalance, default is 100',
        `enable_health_check` TINYINT (4) NOT NULL DEFAULT '0' COMMENT 'Whether to open a heartbeat on an instance, check the logic, 0 is not open, 1 is open',
        `logic_set` VARCHAR(128) DEFAULT NULL COMMENT 'Example logic packet information',
        `cmdb_region` VARCHAR(128) DEFAULT NULL COMMENT 'The region information of the instance is mainly used to close the route',
        `cmdb_zone` VARCHAR(128) DEFAULT NULL COMMENT 'The ZONE information of the instance is mainly used to close the route.',
        `cmdb_idc` VARCHAR(128) DEFAULT NULL COMMENT 'The IDC information of the instance is mainly used to close the route',
        `priority` TINYINT (4) NOT NULL DEFAULT '0' COMMENT 'Example priority, currently useless',
        `revision` VARCHAR(32) NOT NULL COMMENT 'Instance version information',
        `flag` TINYINT (4) NOT NULL DEFAULT '0' COMMENT 'Logic delete flag, 0 means visible, 1 means that it has been logically deleted',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last updated time',
        `metadata` TEXT COMMENT 'instance metadata',
        PRIMARY KEY (`id`),
        KEY `service_id` (`service_id`),
        KEY `mtime` (`mtime`),
        KEY `host` (`host`)
    ) ENGINE = InnoDB;

/* 健康检查类型 */
-- --------------------------------------------------------
--
-- Table structure `health_check`
--
CREATE TABLE
    `health_check` (
        `id` VARCHAR(128) NOT NULL COMMENT 'Instance ID',
        `type` TINYINT (4) NOT NULL DEFAULT '0' COMMENT 'Instance health check type',
        `ttl` INT (11) NOT NULL COMMENT 'TTL time jumping',
        PRIMARY KEY (`id`)
    ) ENGINE = InnoDB;

-- --------------------------------------------------------
--
-- Table structure `instance_manual_metadata`, 记录非 SDK 主动上报的实例标签数据信息
--
CREATE TABLE
    `instance_manual_metadata` (
        `id` VARCHAR(128) NOT NULL COMMENT 'Instance ID',
        `mkey` VARCHAR(128) NOT NULL COMMENT 'instance label of Key',
        `mvalue` VARCHAR(4096) NOT NULL COMMENT 'instance label Value',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last updated time',
        PRIMARY KEY (`id`, `mkey`),
        KEY `mkey` (`mkey`)
    ) ENGINE = InnoDB;

/* 命名空间 */
-- --------------------------------------------------------
--
-- Table structure `namespace`
--
CREATE TABLE
    `namespace` (
        `name` VARCHAR(64) NOT NULL COMMENT 'Namespace name, unique',
        `comment` VARCHAR(1024) DEFAULT NULL COMMENT 'Description of namespace',
        `token` VARCHAR(64) NOT NULL COMMENT 'TOKEN named space for write operation check',
        `owner` VARCHAR(1024) NOT NULL COMMENT 'Responsible for named space Owner',
        `flag` TINYINT (4) NOT NULL DEFAULT '0' COMMENT 'Logic delete flag, 0 means visible, 1 means that it has been logically deleted',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last updated time',
        `service_export_to` TEXT COMMENT 'namespace metadata',
        `metadata` TEXT COMMENT 'namespace metadata',
        `kind` TINYINT (4) NOT NULL DEFAULT '0' COMMENT 'namespace kind: 0 business, 1 system',
        PRIMARY KEY (`name`)
    ) ENGINE = InnoDB;

--
-- Data in the conveyor `namespace`
--
INSERT INTO
    `namespace` (
        `name`,
        `comment`,
        `token`,
        `owner`,
        `kind`,
        `flag`,
        `ctime`,
        `mtime`
    )
VALUES
    (
        'pole-system',
        'system namespace only for pole.io server',
        '2d1bfe5d12e04d54b8ee69e62494c7fd',
        'pole',
        1,
        0,
        '2019-09-06 07:55:07',
        '2019-09-06 07:55:07'
    ),
    (
        'default',
        'Default Environment',
        'e2e473081d3d4306b52264e49f7ce227',
        'pole',
        0,
        0,
        '2021-07-27 19:37:37',
        '2021-07-27 19:37:37'
    );

/* 服务信息 */
-- --------------------------------------------------------
--
-- Table structure `service`
--
CREATE TABLE
    `service` (
        `id` VARCHAR(32) NOT NULL COMMENT 'Service ID',
        `name` VARCHAR(128) NOT NULL COMMENT 'Service name, only under the namespace',
        `namespace` VARCHAR(64) NOT NULL COMMENT 'Namespace belongs to the service',
        `ports` TEXT DEFAULT NULL COMMENT 'Service will have a list of all port information of the external exposure (single process exposing multiple protocols)',
        `business` VARCHAR(64) DEFAULT NULL COMMENT 'Service business information',
        `department` VARCHAR(1024) DEFAULT NULL COMMENT 'Service department information',
        `cmdb_mod1` VARCHAR(1024) DEFAULT NULL COMMENT '',
        `cmdb_mod2` VARCHAR(1024) DEFAULT NULL COMMENT '',
        `cmdb_mod3` VARCHAR(1024) DEFAULT NULL COMMENT '',
        `comment` VARCHAR(1024) DEFAULT NULL COMMENT 'Description information',
        `token` VARCHAR(2048) NOT NULL COMMENT 'Service token, used to handle all the services involved in the service',
        `revision` VARCHAR(32) NOT NULL COMMENT 'Service version information',
        `owner` VARCHAR(1024) NOT NULL COMMENT 'Owner information belonging to the service',
        `flag` TINYINT (4) NOT NULL DEFAULT '0' COMMENT 'Logic delete flag, 0 means visible, 1 means that it has been logically deleted',
        `reference` VARCHAR(32) DEFAULT NULL COMMENT 'Service alias, what is the actual service name that the service is actually pointed out?',
        `refer_filter` VARCHAR(1024) DEFAULT NULL COMMENT '',
        `platform_id` VARCHAR(32) DEFAULT '' COMMENT 'The platform ID to which the service belongs',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last updated time',
        `export_to` TEXT COMMENT 'service export to some namespace',
        PRIMARY KEY (`id`),
        UNIQUE KEY `name` (`name`, `namespace`),
        KEY `namespace` (`namespace`),
        KEY `token` (`token`(255)),
        KEY `mtime` (`mtime`),
        KEY `reference` (`reference`),
        KEY `platform_id` (`platform_id`)
    ) ENGINE = InnoDB;

-- Internal data-plane identity. This table is intentionally not joined into
-- public service queries because subject is SDK-internal and immutable.
CREATE TABLE
    `service_identity` (
        `service_id` VARCHAR(32) NOT NULL COMMENT 'Service resource ID',
        `subject` VARCHAR(255) NOT NULL COMMENT 'Stable internal data-plane subject',
        `revision` VARCHAR(32) NOT NULL COMMENT 'Identity descriptor revision',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last updated time',
        PRIMARY KEY (`service_id`),
        UNIQUE KEY `subject` (`subject`)
    ) ENGINE = InnoDB;

-- Control-plane-only logical services. Runtime SDKs continue to address
-- services by namespace and service name.
CREATE TABLE
    `logical_service` (
        `id` VARCHAR(32) NOT NULL COMMENT 'Control-plane logical service ID',
        `name` VARCHAR(128) COLLATE utf8_bin NOT NULL COMMENT 'Logical service display name',
        `comment` VARCHAR(1024) DEFAULT NULL,
        `owner` VARCHAR(1024) NOT NULL DEFAULT '',
        `business` VARCHAR(64) DEFAULT NULL,
        `department` VARCHAR(1024) DEFAULT NULL,
        `revision` VARCHAR(32) NOT NULL,
        `flag` TINYINT (4) NOT NULL DEFAULT 0,
        `active_name` VARCHAR(128) COLLATE utf8_bin GENERATED ALWAYS AS
            (CASE WHEN `flag` = 0 THEN `name` ELSE NULL END) STORED,
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_logical_service_active_name` (`active_name`),
        KEY `idx_logical_service_mtime` (`mtime`)
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_bin;

CREATE TABLE
    `service_environment_binding` (
        `logical_service_id` VARCHAR(32) NOT NULL,
        `service_id` VARCHAR(32) NOT NULL,
        `namespace` VARCHAR(64) COLLATE utf8_bin NOT NULL,
        `service_name` VARCHAR(128) COLLATE utf8_bin NOT NULL,
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
        PRIMARY KEY (`service_id`),
        UNIQUE KEY `uk_logical_service_namespace` (`logical_service_id`, `namespace`),
        KEY `idx_environment_binding_logical` (`logical_service_id`)
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_bin;

-- --------------------------------------------------------
--
-- Data in the conveyor `service`
--
INSERT INTO
    `service` (
        `id`,
        `name`,
        `namespace`,
        `comment`,
        `business`,
        `token`,
        `revision`,
        `owner`,
        `flag`,
        `ctime`,
        `mtime`
    )
VALUES
    (
        'fbca9bfa04ae4ead86e1ecf5811e32a9',
        'pole.checker',
        'pole-system',
        'pole checker service',
        'pole.io',
        '7d19c46de327408d8709ee7392b7700b',
        '301b1e9f0bbd47a6b697e26e99dfe012',
        'pole',
        0,
        '2021-09-06 07:55:07',
        '2021-09-06 07:55:09'
    );

/* 服务元数据 */
-- --------------------------------------------------------
--
-- Table structure `service_metadata`
--
CREATE TABLE
    `service_metadata` (
        `id` VARCHAR(32) NOT NULL COMMENT 'Service ID',
        `mkey` VARCHAR(128) NOT NULL COMMENT 'Service label key',
        `mvalue` VARCHAR(4096) NOT NULL COMMENT 'Service label Value',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last updated time',
        PRIMARY KEY (`id`, `mkey`),
        KEY `mkey` (`mkey`)
    ) ENGINE = InnoDB;

/* 服务订阅关系 */
--
-- Table structure `service_subscribe_graph`
--
CREATE TABLE
    `service_subscribe_graph` (
        `id` VARCHAR(32) NOT NULL COMMENT 'Service ID',
        `caller_name` VARCHAR(128) NOT NULL COMMENT 'Service name, only under the namespace',
        `caller_namespace` VARCHAR(64) NOT NULL COMMENT 'Namespace belongs to the service',
        `callee_name` VARCHAR(128) NOT NULL COMMENT 'Service name, only under the namespace',
        `callee_namespace` VARCHAR(64) NOT NULL COMMENT 'Namespace belongs to the service',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last updated time',
        PRIMARY KEY (`id`),
        KEY `caller_name` (`caller_name`),
        KEY `caller_namespace` (`caller_namespace`),
        KEY `callee_name` (`callee_name`),
        KEY `callee_namespace` (`callee_namespace`),
        KEY `mtime` (`mtime`)
    ) ENGINE = InnoDB;


/* 启动锁 */
-- --------------------------------------------------------
--
-- Table structure `start_lock`
--
CREATE TABLE
    `start_lock` (
        `lock_id` INT (11) NOT NULL COMMENT '锁序号',
        `lock_key` VARCHAR(32) NOT NULL COMMENT 'Lock name',
        `server` VARCHAR(32) NOT NULL COMMENT 'SERVER holding launch lock',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Update time',
        PRIMARY KEY (`lock_id`, `lock_key`)
    ) ENGINE = InnoDB;

--
-- Data in the conveyor `start_lock`
--
INSERT INTO
    `start_lock` (`lock_id`, `lock_key`, `server`, `mtime`)
VALUES
    (1, 'sz', 'aaa', '2019-12-05 08:35:49');

CREATE TABLE
    `leader_election` (
        `elect_key` VARCHAR(128) NOT NULL,
        `version` BIGINT NOT NULL DEFAULT 0,
        `leader` VARCHAR(128) NOT NULL,
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
        PRIMARY KEY (`elect_key`),
        KEY `version` (`version`)
    ) ENGINE = innodb;

/* 配置文件 */

/* 配置分组 */

/* 配置发布 */


/* 配置发布历史 */

/* 配置模板 */

/* Namespace 范围的配置模板定义草稿 */

/* 环境版本复用的配置模板不可变快照 */

/* Namespace + Template Value 草稿 */

/* Namespace + Template 原子环境配置版本 */

/* 配置文件模板绑定不可变发布 */

/* 全局环境晋升拓扑草稿 */
CREATE TABLE `environment_promotion_topology` (
    `id` TINYINT UNSIGNED NOT NULL,
    `draft_revision` BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `published_revision` BIGINT UNSIGNED NOT NULL DEFAULT 0,
    `draft_json` LONGTEXT COLLATE utf8_bin NOT NULL,
    `modify_by` VARCHAR(64) COLLATE utf8_bin NOT NULL DEFAULT '',
    `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (`id`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_bin COMMENT = '全局环境晋升拓扑草稿';

/* 环境晋升拓扑不可变版本 */
CREATE TABLE `environment_promotion_topology_revision` (
    `revision` BIGINT UNSIGNED NOT NULL,
    `topology_json` LONGTEXT COLLATE utf8_bin NOT NULL,
    `comment` VARCHAR(512) COLLATE utf8_bin NOT NULL DEFAULT '',
    `create_by` VARCHAR(64) COLLATE utf8_bin NOT NULL DEFAULT '',
    `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (`revision`)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_bin COMMENT = '环境晋升拓扑不可变版本';

/* 用户 */
CREATE TABLE
    `user` (
        `id` VARCHAR(128) NOT NULL COMMENT 'User ID',
        `name` VARCHAR(100) NOT NULL COMMENT 'user name',
        `password` VARCHAR(100) NOT NULL COMMENT 'user password',
        `owner` VARCHAR(128) NOT NULL COMMENT 'Main account ID',
        `source` VARCHAR(32) NOT NULL COMMENT 'Account source',
        `mobile` VARCHAR(12) NOT NULL DEFAULT '' COMMENT 'Account mobile phone number',
        `email` VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'Account mailbox',
        `token` VARCHAR(255) NOT NULL COMMENT 'The token information owned by the account can be used for SDK access authentication',
        `token_enable` TINYINT (4) NOT NULL DEFAULT 1,
        `user_type` INT NOT NULL DEFAULT 20 COMMENT 'Account type, 0 is the admin super account, 20 is the primary account, 50 for the child account',
        `comment` VARCHAR(255) NOT NULL COMMENT 'describe',
        `flag` TINYINT (4) NOT NULL DEFAULT '0' COMMENT 'Whether the rules are valid, 0 is valid, 1 is invalid, it is deleted',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last updated time',
        `metadata` TEXT COMMENT 'user metadata',
        PRIMARY KEY (`id`),
        UNIQUE KEY (`name`, `owner`),
        KEY `owner` (`owner`),
        KEY `mtime` (`mtime`)
    ) ENGINE = InnoDB;

/* 用户组 */
CREATE TABLE
    `user_group` (
        `id` VARCHAR(128) NOT NULL COMMENT 'User group ID',
        `name` VARCHAR(100) NOT NULL COMMENT 'User group name',
        `owner` VARCHAR(128) NOT NULL COMMENT 'The main account ID of the user group',
        `token` VARCHAR(255) NOT NULL COMMENT 'TOKEN information of this user group',
        `comment` VARCHAR(255) NOT NULL COMMENT 'Description',
        `token_enable` TINYINT (4) NOT NULL DEFAULT 1,
        `flag` TINYINT (4) NOT NULL DEFAULT '0' COMMENT 'Whether the rules are valid, 0 is valid, 1 is invalid, it is deleted',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last updated time',
        `metadata` TEXT COMMENT 'user_group metadata',
        PRIMARY KEY (`id`),
        UNIQUE KEY (`name`, `owner`),
        KEY `owner` (`owner`),
        KEY `mtime` (`mtime`)
    ) ENGINE = InnoDB;

/* 用户组成员 */
CREATE TABLE
    `user_group_relation` (
        `user_id` VARCHAR(128) NOT NULL COMMENT 'User ID',
        `group_id` VARCHAR(128) NOT NULL COMMENT 'User group ID',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last updated time',
        PRIMARY KEY (`user_id`, `group_id`),
        KEY `mtime` (`mtime`)
    ) ENGINE = InnoDB;

/* 鉴权策略 */
CREATE TABLE
    `auth_strategy` (
        `id` VARCHAR(128) NOT NULL COMMENT 'Strategy ID',
        `name` VARCHAR(100) NOT NULL COMMENT 'Policy name',
        `action` VARCHAR(32) NOT NULL COMMENT 'Read and write permission for this policy, only_read = 0, read_write = 1',
        `owner` VARCHAR(128) NOT NULL COMMENT 'The account ID to which this policy is',
        `comment` VARCHAR(255) NOT NULL COMMENT 'describe',
        `default` TINYINT (4) NOT NULL DEFAULT '0',
        `source` VARCHAR(32) NOT NULL COMMENT 'policy rule source',
        `revision` VARCHAR(128) NOT NULL COMMENT 'Authentication rule version',
        `flag` TINYINT (4) NOT NULL DEFAULT '0' COMMENT 'Whether the rules are valid, 0 is valid, 1 is invalid, it is deleted',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last updated time',
        `metadata` TEXT COMMENT 'policy rule metadata',
        PRIMARY KEY (`id`),
        UNIQUE KEY (`name`, `owner`),
        KEY `owner` (`owner`),
        KEY `mtime` (`mtime`)
    ) ENGINE = InnoDB;

/* 策略成员 */
CREATE TABLE
    `auth_principal` (
        `strategy_id` VARCHAR(128) NOT NULL COMMENT 'Strategy ID',
        `principal_id` VARCHAR(128) NOT NULL COMMENT 'Principal ID',
        `principal_role` INT NOT NULL COMMENT 'PRINCIPAL type, 1 is User, 2 is Group, 3 is Role',
        `extend_info` TEXT COMMENT 'link principal extend info',
        PRIMARY KEY (`strategy_id`, `principal_id`, `principal_role`)
    ) ENGINE = InnoDB;

/* 策略包含资源 */
CREATE TABLE
    `auth_strategy_resource` (
        `strategy_id` VARCHAR(128) NOT NULL COMMENT 'Strategy ID',
        `res_type` VARCHAR(128) NOT NULL COMMENT 'Resource Type, Namespaces = 0, Service = 1, configgroups = 2',
        `res_id` VARCHAR(128) NOT NULL COMMENT 'Resource ID',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last updated time',
        PRIMARY KEY (`strategy_id`, `res_type`, `res_id`),
        KEY `mtime` (`mtime`)
    ) ENGINE = InnoDB;

/* 角色数据 */
CREATE TABLE
    `auth_role` (
        `id` VARCHAR(128) NOT NULL COMMENT 'role id',
        `name` VARCHAR(100) NOT NULL COMMENT 'role name',
        `owner` VARCHAR(128) NOT NULL COMMENT 'Main account ID',
        `source` VARCHAR(32) NOT NULL COMMENT 'role source',
        `role_type` INT NOT NULL DEFAULT 20 COMMENT 'role type',
        `comment` VARCHAR(255) NOT NULL COMMENT 'describe',
        `flag` TINYINT (4) NOT NULL DEFAULT '0' COMMENT 'Whether the rules are valid, 0 is valid, 1 is invalid, it is deleted',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last updated time',
        `metadata` TEXT COMMENT 'user metadata',
        PRIMARY KEY (`id`),
        UNIQUE KEY (`name`, `owner`),
        KEY `owner` (`owner`),
        KEY `mtime` (`mtime`)
    ) ENGINE = InnoDB;

/* 角色关联用户/用户组关系表 */
CREATE TABLE
    `auth_role_principal` (
        `role_id` VARCHAR(128) NOT NULL COMMENT 'role id',
        `principal_id` VARCHAR(128) NOT NULL COMMENT 'principal id',
        `principal_role` INT NOT NULL COMMENT 'PRINCIPAL type, 1 is User, 2 is Group',
        `extend_info` TEXT COMMENT 'link principal extend info',
        PRIMARY KEY (`role_id`, `principal_id`, `principal_role`)
    ) ENGINE = InnoDB;

/* 鉴权策略中的资源标签关联信息 */
CREATE TABLE
    `auth_strategy_label` (
        `strategy_id` VARCHAR(128) NOT NULL COMMENT 'strategy id',
        `key` VARCHAR(128) NOT NULL COMMENT 'tag key',
        `value` TEXT NOT NULL COMMENT 'tag value',
        `compare_type` VARCHAR(128) NOT NULL COMMENT 'tag kv compare func',
        PRIMARY KEY (`strategy_id`, `key`)
    ) ENGINE = InnoDB;

/* 鉴权策略中的资源标签关联信息 */
CREATE TABLE
    `auth_strategy_function` (
        `strategy_id` VARCHAR(128) NOT NULL COMMENT 'strategy id',
        `function` VARCHAR(256) NOT NULL COMMENT 'server provider function name',
        PRIMARY KEY (`strategy_id`, `function`)
    ) ENGINE = InnoDB;

CREATE TABLE
    `client` (
        `id` VARCHAR(128) NOT NULL COMMENT 'client id',
        `host` VARCHAR(100) NOT NULL COMMENT 'client host IP',
        `type` VARCHAR(100) NOT NULL COMMENT 'client type: polaris-java/polaris-go',
        `version` VARCHAR(32) NOT NULL COMMENT 'client SDK version',
        `region` VARCHAR(128) DEFAULT NULL COMMENT 'region info for client',
        `zone` VARCHAR(128) DEFAULT NULL COMMENT 'zone info for client',
        `campus` VARCHAR(128) DEFAULT NULL COMMENT 'campus info for client',
        `flag` TINYINT (4) NOT NULL DEFAULT '0' COMMENT '0 is valid, 1 is invalid(deleted)',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'last updated time',
        PRIMARY KEY (`id`),
        KEY `mtime` (`mtime`)
    ) ENGINE = InnoDB;

CREATE TABLE
    `client_stat` (
        `client_id` VARCHAR(128) NOT NULL COMMENT 'client id',
        `target` VARCHAR(100) NOT NULL COMMENT 'target stat platform',
        `port` INT (11) NOT NULL COMMENT 'client port to get stat information',
        `protocol` VARCHAR(100) NOT NULL COMMENT 'stat info transport protocol',
        `path` VARCHAR(128) NOT NULL COMMENT 'stat metric path',
        PRIMARY KEY (`client_id`, `target`, `port`)
    ) ENGINE = InnoDB;

-- ---------------------------------------------------------------------------
-- 治理规则统一表
-- 当前态与发布态通过 rule_type 区分路由、限流、熔断、主动探测、无损、泳道组。
-- ---------------------------------------------------------------------------

CREATE TABLE
    `governance_rule` (
        `id` VARCHAR(128) NOT NULL COMMENT 'rule id',
        `rule_type` VARCHAR(64) NOT NULL COMMENT 'governance rule type',
        `namespace` VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'namespace',
        `name` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'rule name',
        `service_id` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'service id',
        `service` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'service name',
        `method` VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'method',
        `priority` INT NOT NULL DEFAULT 0 COMMENT 'rule priority',
        `enable` TINYINT (4) NOT NULL DEFAULT 1 COMMENT 'enable flag',
        `disable` TINYINT (4) NOT NULL DEFAULT 0 COMMENT 'disable flag',
        `level` VARCHAR(32) NOT NULL DEFAULT '' COMMENT 'breaker level',
        `src_service` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'source service',
        `src_namespace` VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'source namespace',
        `dst_service` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'destination service',
        `dst_namespace` VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'destination namespace',
        `dst_method` VARCHAR(512) NOT NULL DEFAULT '' COMMENT 'destination method',
        `labels` TEXT COMMENT 'labels json',
        `policy` VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'route policy',
        `config` TEXT COMMENT 'config json',
        `rule` MEDIUMTEXT COMMENT 'rule json',
        `revision` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'rule revision',
        `description` VARCHAR(1024) NOT NULL DEFAULT '' COMMENT 'description',
        `metadata` TEXT COMMENT 'metadata json',
        `flag` TINYINT (4) NOT NULL DEFAULT 0 COMMENT 'delete flag',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'create time',
        `etime` TIMESTAMP NOT NULL DEFAULT '1980-01-01 00:00:01' COMMENT 'enable time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'modify time',
        PRIMARY KEY (`id`),
        KEY `idx_rule_type_name` (`rule_type`, `name`),
        KEY `idx_rule_type_namespace_name` (`rule_type`, `namespace`, `name`),
        KEY `idx_rule_type_mtime` (`rule_type`, `mtime`),
        KEY `idx_rule_type_service` (`rule_type`, `namespace`, `service`)
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT = 'governance rule unified table';

CREATE TABLE
    `governance_rule_release` (
        `id` VARCHAR(128) NOT NULL COMMENT 'release id',
        `rule_type` VARCHAR(64) NOT NULL COMMENT 'governance rule type',
        `name` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'release name',
        `rule_id` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'rule id',
        `rule_name` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'rule name',
        `namespace` VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'namespace',
        `service` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'service name',
        `rule` MEDIUMTEXT COMMENT 'released rule json',
        `version` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'release version',
        `active` TINYINT (4) NOT NULL DEFAULT 0 COMMENT 'active flag',
        `description` VARCHAR(1024) NOT NULL DEFAULT '' COMMENT 'description',
        `release_type` VARCHAR(64) NOT NULL DEFAULT '' COMMENT 'release type',
        `client_labels` TEXT COMMENT 'gray client labels json',
        `metadata` TEXT COMMENT 'metadata json',
        `flag` TINYINT (4) NOT NULL DEFAULT 0 COMMENT 'delete flag',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'modify time',
        PRIMARY KEY (`id`),
        KEY `idx_rule_type_rule_id` (`rule_type`, `rule_id`),
        KEY `idx_rule_type_rule_name` (`rule_type`, `rule_name`),
        KEY `idx_rule_type_release` (`rule_type`, `rule_id`, `name`, `release_type`),
        KEY `idx_rule_type_active` (`rule_type`, `active`, `release_type`),
        KEY `idx_rule_type_mtime` (`rule_type`, `mtime`)
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COMMENT = 'governance rule release unified table';

/* 服务契约表 */
CREATE TABLE
    service_contract (
    `id`        VARCHAR(128) NOT NULL COMMENT '服务契约主键',
    `type`      VARCHAR(128) NOT NULL COMMENT '服务契约类型',
    `namespace` VARCHAR(64)  NOT NULL COMMENT '命名空间',
    `service`   VARCHAR(128) NOT NULL COMMENT '服务名称',
    `protocol`  VARCHAR(32)  NOT NULL COMMENT '当前契约对应的协议信息 e.g. http/dubbo/grpc/thrift',
    `version`   VARCHAR(64)  NOT NULL COMMENT '服务契约版本',
    `revision`  VARCHAR(128) NOT NULL COMMENT '当前服务契约的全部内容版本摘要',
    `flag`      TINYINT(4)            DEFAULT 0 COMMENT '逻辑删除标志位 ， 0 位有效 ， 1 为逻辑删除',
    `content`   LONGTEXT COMMENT '描述信息',
    `ctime`     TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `mtime`     TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `metadata`  TEXT COMMENT 'service_contract metadata',
    `content_digest` VARCHAR(128) NOT NULL COMMENT '当前服务契约的内容摘要，用于比较内容',
    -- 通过 服务 + 协议信息 + 契约版本 + 名称 进行一次 hash 计算，作为主键
    PRIMARY KEY (`id`),
    -- 服务 + 协议信息 + 契约版本 + 辅助标签 必须保证唯一
    UNIQUE KEY (
         `namespace`,
         `service`,
         `type`,
         `version`,
         `protocol`
        )
) ENGINE = InnoDB;

/* 服务契约中针对单个接口定义的详细信息描述表 */
CREATE TABLE
    service_contract_detail (
    `id`          VARCHAR(128) NOT NULL COMMENT '服务契约单个接口定义记录主键',
    `contract_id` VARCHAR(128) NOT NULL COMMENT '服务契约 ID',
    `namespace` VARCHAR(64)  NOT NULL COMMENT '命名空间',
    `service`   VARCHAR(128) NOT NULL COMMENT '服务名称',
    `protocol`  VARCHAR(32)  NOT NULL COMMENT '当前契约对应的协议信息 e.g. http/dubbo/grpc/thrift',
    `version`   VARCHAR(64)  NOT NULL COMMENT '服务契约版本',
    `type`      VARCHAR(128) NOT NULL COMMENT '类型',
    `method`      VARCHAR(32)  NOT NULL COMMENT 'http协议中的 method 字段, eg:POST/GET/PUT/DELETE, 其他 gRPC 可以用来标识 stream 类型',
    `path`        VARCHAR(128) NOT NULL COMMENT '接口具体全路径描述',
    `source`      INT COMMENT '该条记录来源, 0:历史SDK/1:MANUAL/2:CLIENT',
    `content`     LONGTEXT COMMENT '描述信息',
    `revision`    VARCHAR(128) NOT NULL COMMENT '当前接口定义的全部内容版本摘要',
    `flag`        TINYINT(4)            DEFAULT 0 COMMENT '逻辑删除标志位, 0 位有效, 1 为逻辑删除',
    `ctime`       TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP,
    `mtime`       TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    `content_digest` VARCHAR(128) NOT NULL COMMENT '当前服务接口的内容摘要，用于比较内容',
    PRIMARY KEY (`id`),
    -- 服务契约id + method + path + source 需保证唯一
    KEY (`contract_id`, `path`, `method`, `source`)
) ENGINE = InnoDB;

/* 灰度资源 */
CREATE TABLE
    `gray_resource` (
        `name` VARCHAR(128) NOT NULL COMMENT '灰度资源',
        `match_rule` TEXT NOT NULL COMMENT '配置规则',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
        `create_by` VARCHAR(32) DEFAULT "" COMMENT '创建人',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '最后更新时间',
        `modify_by` VARCHAR(32) DEFAULT "" COMMENT '最后更新人',
        `flag` TINYINT (4) DEFAULT 0 COMMENT '逻辑删除标志位, 0 位有效, 1 为逻辑删除',
        PRIMARY KEY (`name`)
    ) ENGINE = InnoDB COMMENT = '灰度资源表';

/* 服务端动态配置相关持久化记录信息 */
CREATE TABLE
    `server_setting` (
        `id` varchar(128) not null comment '配置 id',
        `name` varchar(64) not null comment '配置名称',
        `rule` text not null comment '配置内容',
        `ctime` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
        `etime` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
        `mtime` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
        PRIMARY KEY (`id`),
        UNIQUE KEY `name` (`name`)
    ) ENGINE = InnoDB;

/* AI logical resource definitions */
CREATE TABLE
    `mcp_server_definition` (
        `id` VARCHAR(32) NOT NULL COMMENT 'Control-plane logical definition ID',
        `name` VARCHAR(128) COLLATE utf8_bin NOT NULL COMMENT 'MCP server logical name',
        `description` VARCHAR(1024) DEFAULT NULL,
        `owner` VARCHAR(1024) NOT NULL DEFAULT '',
        `business` VARCHAR(64) DEFAULT NULL,
        `department` VARCHAR(1024) DEFAULT NULL,
        `revision` VARCHAR(32) NOT NULL,
        `flag` TINYINT(4) NOT NULL DEFAULT 0,
        `active_name` VARCHAR(128) COLLATE utf8_bin GENERATED ALWAYS AS
            (CASE WHEN `flag` = 0 THEN `name` ELSE NULL END) STORED,
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_active_name` (`active_name`),
        KEY `idx_mtime` (`mtime`)
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_bin;

CREATE TABLE
    `a2a_agent_definition` (
        `id` VARCHAR(32) NOT NULL COMMENT 'Control-plane logical definition ID',
        `name` VARCHAR(128) COLLATE utf8_bin NOT NULL COMMENT 'A2A agent logical name',
        `description` VARCHAR(1024) DEFAULT NULL,
        `owner` VARCHAR(1024) NOT NULL DEFAULT '',
        `business` VARCHAR(64) DEFAULT NULL,
        `department` VARCHAR(1024) DEFAULT NULL,
        `revision` VARCHAR(32) NOT NULL,
        `flag` TINYINT(4) NOT NULL DEFAULT 0,
        `active_name` VARCHAR(128) COLLATE utf8_bin GENERATED ALWAYS AS
            (CASE WHEN `flag` = 0 THEN `name` ELSE NULL END) STORED,
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
        PRIMARY KEY (`id`),
        UNIQUE KEY `uk_active_name` (`active_name`),
        KEY `idx_mtime` (`mtime`)
    ) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_bin;

/* MCP Server */
CREATE TABLE
    `mcp_server` (
        `id` VARCHAR(32) NOT NULL COMMENT 'mcp-server id',
        `name` VARCHAR(128) NOT NULL COMMENT 'mcp-server name, only under the namespace',
        `namespace` VARCHAR(64) NOT NULL COMMENT 'Namespace belongs to the mcp-server',
        `ports` TEXT DEFAULT NULL COMMENT 'mcp-server will have a list of all port information of the external exposure (single process exposing multiple protocols)',
        `business` VARCHAR(64) DEFAULT NULL COMMENT 'mcp-server business information',
        `department` VARCHAR(1024) DEFAULT NULL COMMENT 'mcp-server department information',
        `description` VARCHAR(1024) DEFAULT NULL COMMENT 'Description information',
        `revision` VARCHAR(32) NOT NULL COMMENT 'mcp-server version information',
        `flag` TINYINT (4) NOT NULL DEFAULT '0' COMMENT 'Logic delete flag, 0 means visible, 1 means that it has been logically deleted',
        `reference` VARCHAR(32) DEFAULT NULL COMMENT 'mcp-server what is the actual service name that the service is actually pointed out?',
        `protocol` VARCHAR(32) NOT NULL DEFAULT 'http' COMMENT 'mcp-server protocol, such as stdout/sse/streamable',
        `backend_type` VARCHAR(32) DEFAULT NULL COMMENT 'mcp-server backend type, service or address',
        `backend_service_namespace` VARCHAR(64) DEFAULT NULL COMMENT 'backend pole service namespace when backend_type is service',
        `backend_service_name` VARCHAR(128) DEFAULT NULL COMMENT 'backend pole service name when backend_type is service',
        `backend_service_id` VARCHAR(32) DEFAULT NULL COMMENT 'stable Pole backend service id',
        `backend_address` TEXT DEFAULT NULL COMMENT 'backend address when backend_type is address',
        `definition_id` VARCHAR(32) DEFAULT NULL COMMENT 'control-plane logical definition id',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last updated time',
        `export_to` TEXT COMMENT 'service export to some namespace',
        PRIMARY KEY (`id`),
        UNIQUE KEY `name` (`name`, `namespace`),
        KEY `namespace` (`namespace`),
        KEY `mtime` (`mtime`),
        KEY `reference` (`reference`),
        KEY `backend_type` (`backend_type`),
        KEY `backend_service` (`backend_service_namespace`, `backend_service_name`),
        KEY `idx_mcp_server_backend_service_id` (`backend_service_id`),
        KEY `idx_mcp_server_definition` (`definition_id`),
        UNIQUE KEY `uk_mcp_server_definition_namespace` (`definition_id`, `namespace`)
) ENGINE = InnoDB;

/* MCP TOOl */
CREATE TABLE
    `mcp_server_tools` (
        `id` VARCHAR(32) NOT NULL COMMENT 'mcp-server id',
        `mcp_server_id` VARCHAR(32) NOT NULL COMMENT 'mcp-server id',
        `name` VARCHAR(128) NOT NULL COMMENT 'mcp-server name, only under the namespace',
        `description` VARCHAR(1024) DEFAULT NULL COMMENT 'Description information',
        `input_schema` TEXT COMMENT 'Input schema information, such as json schema',
        `output_schema` TEXT COMMENT 'Output schema information, such as json schema',
        `annotations` TEXT COMMENT 'Annotations information, such as json schema annotations',
        `flag` TINYINT (4) NOT NULL DEFAULT '0' COMMENT 'Logic delete flag, 0 means visible, 1 means that it has been logically deleted',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last updated time',
        PRIMARY KEY (`id`),
        UNIQUE KEY `name` (`name`, `mcp_server_id`),
        KEY `mcp_server_id` (`mcp_server_id`),
        KEY `mtime` (`mtime`)
) ENGINE = InnoDB;

/* A2A Agent Registry */
CREATE TABLE
    `a2a_agent` (
        `id` VARCHAR(32) NOT NULL COMMENT 'a2a agent id',
        `name` VARCHAR(128) NOT NULL COMMENT 'a2a agent name, unique under namespace',
        `namespace` VARCHAR(64) NOT NULL COMMENT 'Namespace belongs to the a2a agent',
        `visibility` VARCHAR(32) DEFAULT NULL COMMENT 'public/private/internal visibility',
        `description` VARCHAR(1024) DEFAULT NULL COMMENT 'Description information',
        `version` VARCHAR(64) DEFAULT NULL COMMENT 'agent version',
        `protocol_version` VARCHAR(32) DEFAULT NULL COMMENT 'a2a protocol version',
        `provider_organization` VARCHAR(128) DEFAULT NULL COMMENT 'agent provider organization',
        `provider_url` VARCHAR(512) DEFAULT NULL COMMENT 'agent provider url',
        `documentation_url` VARCHAR(512) DEFAULT NULL COMMENT 'agent documentation url',
        `icon_url` VARCHAR(512) DEFAULT NULL COMMENT 'agent icon url',
        `business` VARCHAR(64) DEFAULT NULL COMMENT 'business information',
        `department` VARCHAR(1024) DEFAULT NULL COMMENT 'department information',
        `backend_type` VARCHAR(32) DEFAULT NULL COMMENT 'backend type, service or address',
        `backend_service_namespace` VARCHAR(64) DEFAULT NULL COMMENT 'backend service namespace',
        `backend_service_name` VARCHAR(128) DEFAULT NULL COMMENT 'backend service name',
        `backend_service_id` VARCHAR(32) DEFAULT NULL COMMENT 'stable Pole backend service id',
        `backend_address` VARCHAR(512) DEFAULT NULL COMMENT 'custom backend address',
        `preferred_interface_url` VARCHAR(512) DEFAULT NULL COMMENT 'preferred a2a interface url',
        `preferred_protocol_binding` VARCHAR(32) DEFAULT NULL COMMENT 'JSONRPC, GRPC or HTTP+JSON',
        `preferred_protocol_version` VARCHAR(32) DEFAULT NULL COMMENT 'preferred interface protocol version',
        `streaming` TINYINT(1) NOT NULL DEFAULT '0' COMMENT 'whether agent declares streaming capability',
        `push_notifications` TINYINT(1) NOT NULL DEFAULT '0' COMMENT 'whether agent declares push notification capability',
        `extended_agent_card` TINYINT(1) NOT NULL DEFAULT '0' COMMENT 'whether agent declares authenticated extended card capability',
        `raw_card_json` MEDIUMTEXT COMMENT 'raw public agent card json',
        `source_type` VARCHAR(32) DEFAULT NULL COMMENT 'manual/well-known/curated',
        `source_url` VARCHAR(512) DEFAULT NULL COMMENT 'agent card source url',
        `last_fetch_status` VARCHAR(128) DEFAULT NULL COMMENT 'last agent card fetch status',
        `last_fetch_time` VARCHAR(64) DEFAULT NULL COMMENT 'last agent card fetch time',
        `metadata` TEXT COMMENT 'custom metadata json',
        `definition_id` VARCHAR(32) DEFAULT NULL COMMENT 'control-plane logical definition id',
        `flag` TINYINT(4) NOT NULL DEFAULT '0' COMMENT 'Logic delete flag, 0 means visible, 1 means logically deleted',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last updated time',
        PRIMARY KEY (`id`),
        UNIQUE KEY `name` (`name`, `namespace`),
        KEY `namespace` (`namespace`),
        KEY `mtime` (`mtime`),
        KEY `backend_service` (`backend_service_namespace`, `backend_service_name`),
        KEY `idx_a2a_agent_backend_service_id` (`backend_service_id`),
        KEY `preferred_protocol_binding` (`preferred_protocol_binding`),
        KEY `idx_a2a_agent_definition` (`definition_id`),
        UNIQUE KEY `uk_a2a_agent_definition_namespace` (`definition_id`, `namespace`)
) ENGINE = InnoDB;

CREATE TABLE
    `a2a_agent_interface` (
        `id` VARCHAR(32) NOT NULL COMMENT 'a2a agent interface id',
        `agent_id` VARCHAR(32) NOT NULL COMMENT 'a2a agent id',
        `url` VARCHAR(512) NOT NULL COMMENT 'a2a interface url',
        `protocol_binding` VARCHAR(32) NOT NULL COMMENT 'JSONRPC, GRPC or HTTP+JSON',
        `protocol_version` VARCHAR(32) NOT NULL COMMENT 'a2a protocol version',
        `tenant` VARCHAR(128) DEFAULT NULL COMMENT 'opaque tenant routing value',
        `flag` TINYINT(4) NOT NULL DEFAULT '0' COMMENT 'Logic delete flag, 0 means visible, 1 means logically deleted',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last updated time',
        PRIMARY KEY (`id`),
        KEY `agent_id` (`agent_id`),
        KEY `mtime` (`mtime`)
) ENGINE = InnoDB;

CREATE TABLE
    `a2a_agent_skill` (
        `id` VARCHAR(32) NOT NULL COMMENT 'a2a agent skill row id',
        `agent_id` VARCHAR(32) NOT NULL COMMENT 'a2a agent id',
        `skill_id` VARCHAR(128) NOT NULL COMMENT 'skill id in agent card',
        `name` VARCHAR(128) NOT NULL COMMENT 'skill name',
        `description` VARCHAR(1024) DEFAULT NULL COMMENT 'skill description',
        `tags` TEXT COMMENT 'skill tag json array',
        `examples` TEXT COMMENT 'skill examples json array',
        `input_modes` TEXT COMMENT 'skill input modes json array',
        `output_modes` TEXT COMMENT 'skill output modes json array',
        `security_requirements` TEXT COMMENT 'skill security requirements json',
        `flag` TINYINT(4) NOT NULL DEFAULT '0' COMMENT 'Logic delete flag, 0 means visible, 1 means logically deleted',
        `ctime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT 'Create time',
        `mtime` TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT 'Last updated time',
        PRIMARY KEY (`id`),
        UNIQUE KEY `skill_id` (`agent_id`, `skill_id`),
        KEY `agent_id` (`agent_id`),
        KEY `mtime` (`mtime`)
) ENGINE = InnoDB;
