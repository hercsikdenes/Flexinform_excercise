-- +goose Up
CREATE TABLE clients (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    personal_id TEXT NOT NULL UNIQUE,
    is_active BOOLEAN NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE vehicles (
    id INTEGER PRIMARY KEY,
    type TEXT NOT NULL,
    registered_at TIMESTAMP NOT NULL,
    own_brand BOOLEAN NOT NULL,
    accident_count INTEGER NOT NULL CHECK (accident_count >= 0),
    is_active BOOLEAN NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE car_ownerships (
    id INTEGER PRIMARY KEY,
    client_id TEXT NOT NULL,
    vehicle_id INTEGER NOT NULL,
    client_car_number INTEGER NOT NULL CHECK (client_car_number > 0),
    is_active BOOLEAN NOT NULL,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL,
    CONSTRAINT fk_car_ownerships_client
        FOREIGN KEY (client_id) REFERENCES clients(id) ON DELETE RESTRICT ON UPDATE RESTRICT,
    CONSTRAINT fk_car_ownerships_vehicle
        FOREIGN KEY (vehicle_id) REFERENCES vehicles(id) ON DELETE RESTRICT ON UPDATE RESTRICT
);

CREATE INDEX idx_car_ownerships_client_active_number
    ON car_ownerships (client_id, is_active, client_car_number);
CREATE INDEX idx_car_ownerships_vehicle
    ON car_ownerships (vehicle_id);

CREATE TABLE event_log (
    id INTEGER PRIMARY KEY,
    ownership_id INTEGER NOT NULL,
    log_number INTEGER NOT NULL CHECK (log_number > 0),
    event TEXT NOT NULL,
    event_time TIMESTAMP NULL,
    document_id TEXT NULL,
    created_at TIMESTAMP NOT NULL,
    CONSTRAINT fk_event_log_ownership
        FOREIGN KEY (ownership_id) REFERENCES car_ownerships(id) ON DELETE RESTRICT ON UPDATE RESTRICT,
    CONSTRAINT uq_event_log_ownership_log_number UNIQUE (ownership_id, log_number)
);

-- +goose Down
DROP TABLE IF EXISTS event_log;
DROP TABLE IF EXISTS car_ownerships;
DROP TABLE IF EXISTS vehicles;
DROP TABLE IF EXISTS clients;
