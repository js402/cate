CREATE TABLE IF NOT EXISTS ollama_models (
    id VARCHAR(36) PRIMARY KEY,
    model VARCHAR(255) NOT NULL UNIQUE,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS llm_pool (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    purpose_type VARCHAR(255) NOT NULL,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS llm_backends (
    id VARCHAR(36) PRIMARY KEY,
    name VARCHAR(255) NOT NULL UNIQUE,
    base_url TEXT NOT NULL UNIQUE,
    type VARCHAR(255) NOT NULL,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS llm_pool_backend_assignments (
    pool_id VARCHAR(36) NOT NULL REFERENCES llm_pool(id) ON DELETE CASCADE,
    backend_id VARCHAR(36) NOT NULL REFERENCES llm_backends(id) ON DELETE CASCADE,
    PRIMARY KEY (pool_id, backend_id),
    assigned_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS ollama_model_assignments (
    model_id VARCHAR(36) NOT NULL REFERENCES ollama_models(id) ON DELETE CASCADE,
    llm_pool_id VARCHAR(36) NOT NULL REFERENCES llm_pool(id) ON DELETE CASCADE,
    PRIMARY KEY (model_id, llm_pool_id),

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
    id VARCHAR(255) PRIMARY KEY,
    friendly_name TEXT,
    email TEXT NOT NULL UNIQUE,
    subject TEXT NOT NULL UNIQUE,
    hashed_password TEXT,
    recovery_code_hash TEXT,
    salt TEXT,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS accesslists (
    id VARCHAR(36) PRIMARY KEY,

    identity VARCHAR(255) NOT NULL REFERENCES users(subject) ON DELETE CASCADE,
    resource VARCHAR(255) NOT NULL,
    permission INT NOT NULL,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS job_queue_v2 (
    id VARCHAR(255) PRIMARY KEY,
    task_type VARCHAR(255) NOT NULL,
    payload JSONB NOT NULL,

    scheduled_for INT,
    valid_until INT,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS files (
    id VARCHAR(36) PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,
    type VARCHAR(255) NOT NULL,

    meta JSONB NOT NULL,
    blobs_id TEXT,
    is_folder BOOLEAN DEFAULT FALSE,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL

);

CREATE TABLE IF NOT EXISTS blobs (
    id VARCHAR(36) PRIMARY KEY,
    meta JSONB NOT NULL,

    data bytea NOT NULL,

    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE IF NOT EXISTS messages (
    id VARCHAR(36) PRIMARY KEY,
    stream VARCHAR(36) NOT NULL,

    payload JSONB NOT NULL,
    added_at TIMESTAMP NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_job_queue_v2_task_type ON job_queue_v2 USING hash(task_type);
CREATE INDEX IF NOT EXISTS idx_accesslists_identity ON accesslists USING hash(identity);
CREATE INDEX IF NOT EXISTS idx_users_email ON users USING hash(email);
CREATE INDEX IF NOT EXISTS idx_users_subject ON users USING hash(subject);
-- ALTER TABLE users ADD COLUMN IF NOT EXISTS salt TEXT;
CREATE INDEX IF NOT EXISTS idx_messages_stream ON messages USING hash(stream);

-- For pagination --
CREATE INDEX IF NOT EXISTS idx_users_created_at ON users (created_at);
CREATE INDEX IF NOT EXISTS idx_accesslists_created_at ON accesslists (created_at);

-- For filesystem --
CREATE INDEX IF NOT EXISTS idx_files_path ON files (path);
ALTER TABLE accesslists ADD CONSTRAINT fk_accesslists_identity FOREIGN KEY (identity) REFERENCES users(subject) ON DELETE CASCADE;

-- CREATE INDEX IF NOT EXISTS idx_files_created_at ON files (created_at);
-- CREATE INDEX IF NOT EXISTS idx_blobs_created_at ON blobs (created_at);

CREATE OR REPLACE FUNCTION estimate_row_count(table_name TEXT)
RETURNS BIGINT AS $$
DECLARE
    result BIGINT;
BEGIN
    SELECT reltuples::BIGINT
    INTO result
    FROM pg_class
    WHERE relname = table_name;

    RETURN COALESCE(result, 0);
END;
$$ LANGUAGE plpgsql STABLE;
