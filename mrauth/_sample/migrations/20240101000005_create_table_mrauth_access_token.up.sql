-- --------------------------------------------------------------------------------------------------

CREATE SCHEMA sample_schema AUTHORIZATION user_pg;

-- --------------------------------------------------------------------------------------------------

CREATE TABLE sample_schema.mrauth_access_tokens (
    token_name character varying(128) NOT NULL CONSTRAINT pk_mrauth_access_tokens PRIMARY KEY,
    user_id uuid NOT NULL,
    token_scopes jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT NOW(),
    expires_at timestamp with time zone NOT NULL
);

CREATE INDEX ix_mrauth_access_tokens_user_id ON sample_schema.mrauth_access_tokens (user_id);
CREATE INDEX ix_mrauth_access_tokens_expires_at ON sample_schema.mrauth_access_tokens (expires_at);
