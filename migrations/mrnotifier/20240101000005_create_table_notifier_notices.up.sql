-- --------------------------------------------------------------------------------------------------

CREATE SCHEMA sample_schema AUTHORIZATION user_pg;

-- --------------------------------------------------------------------------------------------------

CREATE TABLE sample_schema.notifier_notices (
    notice_id int8 NOT NULL CONSTRAINT pk_notifier_notices PRIMARY KEY,
    notice_key character varying(128) NOT NULL,
    notice_data jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT NOW()
);
