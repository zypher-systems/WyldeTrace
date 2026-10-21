CREATE TABLE api_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    prefix TEXT NOT NULL UNIQUE,
    token_hash TEXT NOT NULL UNIQUE,
    role user_role NOT NULL DEFAULT 'readonly',
    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    last_used_at TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_api_tokens_prefix ON api_tokens(prefix);

CREATE TABLE discovery_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL,
    subnet_id UUID NOT NULL REFERENCES subnets(id) ON DELETE CASCADE,
    interval_minutes INTEGER NOT NULL DEFAULT 60 CHECK (interval_minutes >= 1),
    enabled BOOLEAN NOT NULL DEFAULT true,
    enrich BOOLEAN NOT NULL DEFAULT true,
    last_run_at TIMESTAMPTZ,
    last_status TEXT NOT NULL DEFAULT '',
    last_summary JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_discovery_jobs_enabled ON discovery_jobs(enabled);
CREATE INDEX idx_discovery_jobs_subnet ON discovery_jobs(subnet_id);
