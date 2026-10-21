INSERT INTO app_settings (key, value)
VALUES ('subnet_utilization_alert_percent', '90')
ON CONFLICT (key) DO NOTHING;

CREATE TABLE IF NOT EXISTS subnet_settings (
    subnet_id UUID PRIMARY KEY REFERENCES subnets(id) ON DELETE CASCADE,
    util_alert_percent INTEGER NULL
        CHECK (util_alert_percent IS NULL OR (util_alert_percent >= 1 AND util_alert_percent <= 100)),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
