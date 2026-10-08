package repository_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrentity"
	"github.com/mondegor/go-core/mrpostgres/builder/part"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/mrstorage/mrsql"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrordering/entity"
	"github.com/mondegor/go-components/mrordering/repository"
	"github.com/mondegor/go-components/tests"
)

const orderingTableName = "sample_schema.sample_ordering"

type NodePostgresTestSuite struct {
	suite.Suite

	ctx  context.Context
	pgt  *pgtest.Tester
	repo *repository.NodePostgres
}

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestNodePostgresTestSuite(t *testing.T) {
	suite.Run(t, new(NodePostgresTestSuite))
}

func (ts *NodePostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrordering"))

	// условие хоста: удалённые элементы в списке не участвуют
	ts.repo = repository.NewNodePostgres(
		ts.pgt.ConnManager(),
		mrsql.DBTableInfo{
			Name:       orderingTableName,
			PrimaryKey: "node_id",
		},
		part.NewSQLConditionBuilder(),
		func(int) (string, []any) {
			return "deleted_at IS NULL", nil
		},
	)
}

func (ts *NodePostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_FetchNode - элемент списка читается по ID в пределах указанной группы.
func (ts *NodePostgresTestSuite) Test_FetchNode() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Node/FetchNode")

	row, err := ts.repo.FetchNode(ts.ctx, 2, ts.group(1))
	ts.Require().NoError(err)
	ts.Equal(entity.Node{ID: 2, PrevID: 1, NextID: 3, OrderIndex: 20}, row)

	row, err = ts.repo.FetchNode(ts.ctx, 4, ts.group(1))
	ts.Require().NoError(err)
	ts.Equal(entity.Node{ID: 4}, row, "у элемента вне списка поля сортировки нулевые")
}

// Test_FetchNodeWhenNotExists - элемент другой группы, удалённый (скрытый условием хоста)
// и неизвестный не находятся: ErrEventStorageNoRecordFound.
func (ts *NodePostgresTestSuite) Test_FetchNodeWhenNotExists() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Node/FetchNode")

	for _, nodeID := range []uint64{6, 5, 99} {
		_, err := ts.repo.FetchNode(ts.ctx, nodeID, ts.group(1))
		ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound, "nodeID=%d", nodeID)
	}
}

// Test_FetchFirstNode - возвращается элемент группы с наименьшим order_index;
// элементы вне списка и удалённые не учитываются.
func (ts *NodePostgresTestSuite) Test_FetchFirstNode() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Node/FetchFirstNode")

	row, err := ts.repo.FetchFirstNode(ts.ctx, ts.group(1))
	ts.Require().NoError(err)
	ts.Equal(entity.Node{ID: 1, NextID: 2, OrderIndex: 10}, row)

	row, err = ts.repo.FetchFirstNode(ts.ctx, ts.group(2))
	ts.Require().NoError(err)
	ts.Equal(entity.Node{ID: 6, OrderIndex: 1}, row)
}

// Test_FetchFirstNodeWhenEmpty - у группы без упорядоченных элементов возвращается пустой элемент без ошибки.
func (ts *NodePostgresTestSuite) Test_FetchFirstNodeWhenEmpty() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Node/FetchFirstNode")

	row, err := ts.repo.FetchFirstNode(ts.ctx, ts.group(3))
	ts.Require().NoError(err)
	ts.Equal(entity.Node{}, row)
}

// Test_FetchLastNode - возвращается элемент группы с наибольшим order_index.
func (ts *NodePostgresTestSuite) Test_FetchLastNode() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Node/FetchLastNode")

	row, err := ts.repo.FetchLastNode(ts.ctx, ts.group(1))
	ts.Require().NoError(err)
	ts.Equal(entity.Node{ID: 3, PrevID: 2, OrderIndex: 30}, row)
}

// Test_FetchLastNodeWhenEmpty - у группы без упорядоченных элементов возвращается пустой элемент без ошибки.
func (ts *NodePostgresTestSuite) Test_FetchLastNodeWhenEmpty() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Node/FetchLastNode")

	row, err := ts.repo.FetchLastNode(ts.ctx, ts.group(3))
	ts.Require().NoError(err)
	ts.Equal(entity.Node{}, row)
}

// Test_UpdateNode - местоположение элемента (соседи и order_index) обновляется целиком,
// нулевые значения записываются как NULL.
func (ts *NodePostgresTestSuite) Test_UpdateNode() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Node/UpdateNode")

	ts.Require().NoError(ts.repo.UpdateNode(ts.ctx, entity.Node{ID: 4, PrevID: 3, OrderIndex: 40}, ts.group(1)))

	row, err := ts.repo.FetchNode(ts.ctx, 4, ts.group(1))
	ts.Require().NoError(err)
	ts.Equal(entity.Node{ID: 4, PrevID: 3, OrderIndex: 40}, row)

	ts.Require().NoError(ts.repo.UpdateNode(ts.ctx, entity.Node{ID: 1}, ts.group(1)))

	row, err = ts.repo.FetchNode(ts.ctx, 1, ts.group(1))
	ts.Require().NoError(err)
	ts.Equal(entity.Node{ID: 1}, row)
}

// Test_UpdateNodeWhenNotExists - элемент другой группы, удалённый и неизвестный не обновляются:
// возвращается ErrEventStorageNoRecordFound.
func (ts *NodePostgresTestSuite) Test_UpdateNodeWhenNotExists() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Node/UpdateNode")

	for _, nodeID := range []uint64{6, 5, 99} {
		err := ts.repo.UpdateNode(ts.ctx, entity.Node{ID: nodeID, OrderIndex: 100}, ts.group(1))
		ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound, "nodeID=%d", nodeID)
	}

	row, err := ts.repo.FetchNode(ts.ctx, 6, ts.group(2))
	ts.Require().NoError(err)
	ts.Equal(entity.Node{ID: 6, OrderIndex: 1}, row)
}

// Test_UpdateNodePrevID - меняется только предыдущий сосед элемента, ноль записывается как NULL.
func (ts *NodePostgresTestSuite) Test_UpdateNodePrevID() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Node/UpdateNodePrevID")

	ts.Require().NoError(ts.repo.UpdateNodePrevID(ts.ctx, 1, 4, ts.group(1)))

	row, err := ts.repo.FetchNode(ts.ctx, 1, ts.group(1))
	ts.Require().NoError(err)
	ts.Equal(entity.Node{ID: 1, PrevID: 4, NextID: 2, OrderIndex: 10}, row)

	ts.Require().NoError(ts.repo.UpdateNodePrevID(ts.ctx, 2, 0, ts.group(1)))

	row, err = ts.repo.FetchNode(ts.ctx, 2, ts.group(1))
	ts.Require().NoError(err)
	ts.Equal(entity.Node{ID: 2, NextID: 3, OrderIndex: 20}, row)
}

// Test_UpdateNodePrevIDWhenNotExists - элемент другой группы и неизвестный не обновляются:
// возвращается ErrEventStorageNoRecordFound.
func (ts *NodePostgresTestSuite) Test_UpdateNodePrevIDWhenNotExists() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Node/UpdateNodePrevID")

	err := ts.repo.UpdateNodePrevID(ts.ctx, 6, 1, ts.group(1))
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)

	err = ts.repo.UpdateNodePrevID(ts.ctx, 99, 1, ts.group(1))
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_UpdateNodeNextID - меняется только следующий сосед элемента, ноль записывается как NULL.
func (ts *NodePostgresTestSuite) Test_UpdateNodeNextID() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Node/UpdateNodeNextID")

	ts.Require().NoError(ts.repo.UpdateNodeNextID(ts.ctx, 3, 4, ts.group(1)))

	row, err := ts.repo.FetchNode(ts.ctx, 3, ts.group(1))
	ts.Require().NoError(err)
	ts.Equal(entity.Node{ID: 3, PrevID: 2, NextID: 4, OrderIndex: 30}, row)

	ts.Require().NoError(ts.repo.UpdateNodeNextID(ts.ctx, 2, 0, ts.group(1)))

	row, err = ts.repo.FetchNode(ts.ctx, 2, ts.group(1))
	ts.Require().NoError(err)
	ts.Equal(entity.Node{ID: 2, PrevID: 1, OrderIndex: 20}, row)
}

// Test_UpdateNodeNextIDWhenNotExists - элемент другой группы и неизвестный не обновляются:
// возвращается ErrEventStorageNoRecordFound.
func (ts *NodePostgresTestSuite) Test_UpdateNodeNextIDWhenNotExists() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Node/UpdateNodeNextID")

	err := ts.repo.UpdateNodeNextID(ts.ctx, 6, 1, ts.group(1))
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)

	err = ts.repo.UpdateNodeNextID(ts.ctx, 99, 1, ts.group(1))
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)
}

// Test_RecalcOrderIndex - order_index элементов группы строго после границы сдвигается на шаг;
// элементы до границы, вне списка, удалённые и других групп не меняются.
func (ts *NodePostgresTestSuite) Test_RecalcOrderIndex() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Node/RecalcOrderIndex")

	ts.Require().NoError(ts.repo.RecalcOrderIndex(ts.ctx, 10, 100, ts.group(1)))
	ts.Equal([]uint64{10, 120, 130, 0, 5, 1}, ts.fetchOrderIndexes())

	ts.Require().NoError(ts.repo.RecalcOrderIndex(ts.ctx, 120, 1, ts.group(1)))
	ts.Equal([]uint64{10, 120, 131, 0, 5, 1}, ts.fetchOrderIndexes())
}

// Test_RecalcOrderIndexWhenNothingToShift - когда сдвигать нечего, это не ошибка.
func (ts *NodePostgresTestSuite) Test_RecalcOrderIndexWhenNothingToShift() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Node/RecalcOrderIndex")

	ts.Require().NoError(ts.repo.RecalcOrderIndex(ts.ctx, 1000, 1, ts.group(1)))
	ts.Equal([]uint64{10, 20, 30, 0, 5, 1}, ts.fetchOrderIndexes())
}

// group - условие вызова: элементы указанной группы.
func (ts *NodePostgresTestSuite) group(groupID uint64) mrstorage.SQLPartFunc {
	return func(argumentNumber int) (string, []any) {
		return "group_id = $" + strconv.Itoa(argumentNumber), []any{groupID}
	}
}

// fetchOrderIndexes - order_index всех элементов таблицы в порядке node_id (NULL - как ноль).
func (ts *NodePostgresTestSuite) fetchOrderIndexes() []uint64 {
	rows, err := ts.pgt.ConnManager().Conn(ts.ctx).Query(
		ts.ctx,
		`SELECT order_index FROM `+orderingTableName+` ORDER BY node_id;`,
	)
	ts.Require().NoError(err)

	defer rows.Close()

	var indexes []uint64

	for rows.Next() {
		var index mrentity.ZeronullUint64

		ts.Require().NoError(rows.Scan(&index))

		indexes = append(indexes, uint64(index))
	}

	ts.Require().NoError(rows.Err())

	return indexes
}
