-- +goose Up
CREATE TABLE IF NOT EXISTS metrics (
    record_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    id VARCHAR(200) NOT NULL,
    type VARCHAR(200) NOT NULL,
    delta BIGINT,
    value DOUBLE PRECISION
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_metrics_id_type ON metrics (id, type);

-- +goose Down
DROP INDEX IF EXISTS idx_metrics_id_type;
DROP TABLE IF EXISTS metrics;