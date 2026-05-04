-- Aggregated schema for sqlc code generation (matches applied migrations).
CREATE TYPE task_state AS ENUM ('received', 'processing', 'done');

CREATE TABLE tasks (
    id BIGSERIAL PRIMARY KEY,
    type SMALLINT NOT NULL CHECK (type >= 0 AND type <= 9),
    value SMALLINT NOT NULL CHECK (value >= 0 AND value <= 99),
    state task_state NOT NULL DEFAULT 'received',
    creation_time DOUBLE PRECISION NOT NULL,
    last_update_time DOUBLE PRECISION NOT NULL
);
