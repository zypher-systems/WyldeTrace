INSERT INTO app_settings (key, value)
VALUES ('webhook_mute_hours', '6')
ON CONFLICT (key) DO NOTHING;

CREATE TABLE IF NOT EXISTS webhook_event_mutes (
    event TEXT NOT NULL,
    entity_key TEXT NOT NULL,
    muted_until TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (event, entity_key)
);

CREATE TABLE IF NOT EXISTS discovery_job_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    job_id UUID NOT NULL REFERENCES discovery_jobs(id) ON DELETE CASCADE,
    status TEXT NOT NULL DEFAULT '',
    summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_discovery_job_runs_job_started
    ON discovery_job_runs (job_id, started_at DESC);

CREATE TABLE IF NOT EXISTS app_notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    kind TEXT NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    entity_type TEXT NOT NULL DEFAULT '',
    entity_id TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    read_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_app_notifications_created
    ON app_notifications (created_at DESC);
