CREATE TABLE IF NOT EXISTS `config` (
    `id` INTEGER PRIMARY KEY NOT NULL,
    `key` TEXT NOT NULL,
    `value` TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS `config_ix1` ON `config`(`key`);

CREATE TABLE IF NOT EXISTS `user_session` (
    `id` INTEGER PRIMARY KEY NOT NULL,
    `key` TEXT NOT NULL,
    `valid_until` INTEGER NOT NULL,
    `modified_date` INTEGER NOT NULL,
    `create_date` INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS `user_session_ix1` ON `user_session`(`key`);

CREATE TABLE IF NOT EXISTS `user_verify` (
    `id` INTEGER PRIMARY KEY NOT NULL,
    `key` TEXT NOT NULL,
    `valid_until` INTEGER NOT NULL,
    `modified_date` INTEGER NOT NULL,
    `create_date` INTEGER NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS `user_verify_ix1` ON `user_verify`(`key`);
