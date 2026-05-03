CREATE TABLE notification_channels (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type       TEXT        NOT NULL,
    name       TEXT        NOT NULL,
    config     JSONB       NOT NULL DEFAULT '{}',
    enabled    BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX notification_channels_user_id_idx ON notification_channels (user_id);

CREATE TABLE alert_rules (
    id                UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel_id        UUID        NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    type              TEXT        NOT NULL,
    params            JSONB       NOT NULL DEFAULT '{}',
    enabled           BOOLEAN     NOT NULL DEFAULT TRUE,
    cooldown_minutes  INT         NOT NULL DEFAULT 60,
    last_triggered_at TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX alert_rules_user_id_idx    ON alert_rules (user_id);
CREATE INDEX alert_rules_channel_id_idx ON alert_rules (channel_id);
