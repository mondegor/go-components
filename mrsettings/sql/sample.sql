-- --------------------------------------------------------------------------------------------------
-- --------------------------------------------------------------------------------------------------
-- --------------------------------------------------------------------------------------------------

CREATE TABLE sample_schema.sample_settings (
    setting_id int4 NOT NULL CONSTRAINT pk_sample_settings PRIMARY KEY,
    setting_name character varying(64) NOT NULL,
    setting_type int2 NOT NULL, -- 1=STRING, 2=STRING_LIST, 3=INTEGER, 4=INTEGER_LIST, 5=BOOLEAN
    setting_value character varying(65536) NOT NULL,
    setting_description character varying(1024) NOT NULL,
    created_at timestamp with time zone NOT NULL DEFAULT NOW(),
    updated_at timestamp with time zone NOT NULL DEFAULT NOW()
);

CREATE UNIQUE INDEX uk_sample_settings_setting_name ON sample_schema.sample_settings (setting_name);
CREATE INDEX ix_sample_settings_updated_at ON sample_schema.sample_settings (updated_at);

-- --------------------------------------------------------------------------------------------------