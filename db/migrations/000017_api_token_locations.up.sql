CREATE TABLE IF NOT EXISTS api_token_locations (
    token_id UUID NOT NULL REFERENCES api_tokens(id) ON DELETE CASCADE,
    location_id UUID NOT NULL REFERENCES locations(id) ON DELETE CASCADE,
    PRIMARY KEY (token_id, location_id)
);

CREATE INDEX IF NOT EXISTS idx_api_token_locations_location
    ON api_token_locations (location_id);
