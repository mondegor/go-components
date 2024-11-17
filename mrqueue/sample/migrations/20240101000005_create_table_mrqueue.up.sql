-- --------------------------------------------------------------------------------------------------

CREATE SCHEMA sample_schema AUTHORIZATION user_pg;

-- --------------------------------------------------------------------------------------------------

-- sequence name = table_name + "_" + primary_key_name + "_seq"
CREATE SEQUENCE sample_schema.mrqueue_item_id_seq START 1;

-- --------------------------------------------------------------------------------------------------

-- for select, insert, update, delete
CREATE TABLE sample_schema.mrqueue (
    item_id int8 NOT NULL CONSTRAINT pk_mrqueue PRIMARY KEY,
    remaining_attempts int2 NOT NULL, -- кол-во оставшихся попыток отправки сообщения
    item_status int2 NOT NULL, -- 1=READY, 2=PROCESSING, 3=RETRY
    updated_at timestamp with time zone NOT NULL DEFAULT NOW() -- item with status = READY and updated_at > NOW() = delayed
);

CREATE INDEX ix_mrqueue_item_status ON sample_schema.mrqueue  (item_status, updated_at);

-- --------------------------------------------------------------------------------------------------

-- OPTIONAL
-- for select, insert, delete (in background)
CREATE TABLE sample_schema.mrqueue_errors (
    item_id int8 NOT NULL,
    error_message text NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT NOW()
);

CREATE INDEX ix_mrqueue_errors_item_id ON sample_schema.mrqueue_errors (item_id);
CREATE INDEX ix_mrqueue_errors_created_at ON sample_schema.mrqueue_errors (created_at);

-- --------------------------------------------------------------------------------------------------

-- OPTIONAL
-- for select, insert, delete (in background)
CREATE TABLE sample_schema.mrqueue_completed (
    item_id int8 NOT NULL CONSTRAINT pk_mrqueue_completed PRIMARY KEY,
    updated_at timestamp with time zone NOT NULL DEFAULT NOW()
);

CREATE INDEX ix_mrqueue_completed_updated_at ON sample_schema.mrqueue_completed (updated_at);
