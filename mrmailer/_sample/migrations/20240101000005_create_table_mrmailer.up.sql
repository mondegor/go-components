-- --------------------------------------------------------------------------------------------------

CREATE SCHEMA sample_schema AUTHORIZATION user_pg;

-- --------------------------------------------------------------------------------------------------

CREATE TABLE sample_schema.mrmailer_messages (
    message_id int8 NOT NULL CONSTRAINT pk_mrmailer_messages PRIMARY KEY,
    message_channel character varying(128) NOT NULL,
    message_data jsonb NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT NOW()
);