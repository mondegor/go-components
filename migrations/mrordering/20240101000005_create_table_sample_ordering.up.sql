-- --------------------------------------------------------------------------------------------------

CREATE SCHEMA sample_schema AUTHORIZATION user_pg;

-- --------------------------------------------------------------------------------------------------

-- пример таблицы хоста, в которую встроен порядок следования элементов (двусвязный список):
-- элемент вне списка имеет NULL во всех трёх полях сортировки
CREATE TABLE sample_schema.sample_ordering (
    node_id int8 NOT NULL CONSTRAINT pk_sample_ordering PRIMARY KEY,
    group_id int8 NOT NULL, -- список, в котором упорядочен элемент (передаётся условием вызова)
    node_caption character varying(128) NOT NULL,
    deleted_at timestamp with time zone NULL,
    prev_field_id int8 NULL CHECK(prev_field_id IS NULL OR prev_field_id > 0),
    next_field_id int8 NULL CHECK(next_field_id IS NULL OR next_field_id > 0),
    order_index int8 NULL CHECK(order_index IS NULL OR order_index > 0)
);

CREATE INDEX ix_sample_ordering_group_id_order_index ON sample_schema.sample_ordering (group_id, order_index) WHERE deleted_at IS NULL;
