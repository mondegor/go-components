package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrstorage/mrsql"
	"github.com/mondegor/go-storage/mrtests/pgtest"
	"github.com/stretchr/testify/suite"

	"github.com/mondegor/go-components/mrqueue/dto"
	"github.com/mondegor/go-components/mrqueue/enum/itemstatus"
	"github.com/mondegor/go-components/mrqueue/repository"
	"github.com/mondegor/go-components/tests"
)

const queueTableName = "sample_schema.mrqueue"

type (
	QueuePostgresTestSuite struct {
		suite.Suite

		ctx  context.Context
		pgt  *pgtest.Tester
		repo *repository.QueuePostgres
	}

	// queueRow - строка очереди, считанная сырым SELECT (репозиторий read-метода не имеет).
	queueRow struct {
		ID                uint64
		RemainingAttempts int16
		Status            itemstatus.Enum
		UpdatedAt         time.Time
	}
)

// ВНИМАНИЕ: t.Parallel() здесь не ставится - каждый suite поднимает свой контейнер
// Postgres, одновременный запуск нескольких suite'ов исчерпывает память Docker.
func TestQueuePostgresTestSuite(t *testing.T) {
	suite.Run(t, new(QueuePostgresTestSuite))
}

func (ts *QueuePostgresTestSuite) SetupSuite() {
	ts.ctx = context.Background()
	ts.pgt = pgtest.NewTester(ts.T(), tests.DBSchemas(), tests.ExcludedDBTables())
	ts.pgt.ApplyMigrations(ts.T(), tests.MigrationsDir("mrqueue"))

	ts.repo = repository.NewQueuePostgres(
		ts.pgt.ConnManager(),
		mrsql.DBTableInfo{
			Name:       queueTableName,
			PrimaryKey: "item_id",
		},
	)
}

func (ts *QueuePostgresTestSuite) SetupTest() {
	ts.pgt.TruncateTables(ts.T(), ts.ctx)
}

// Test_Insert - записи добавляются в статусе READY с указанным числом попыток;
// время готовности отложенной записи сдвигается на указанную задержку.
func (ts *QueuePostgresTestSuite) Test_Insert() {
	err := ts.repo.Insert(ts.ctx, []dto.Item{
		{ID: 1, RetryAttempts: 3},
		{ID: 2, RetryAttempts: 5, ReadyDelayed: time.Hour},
	})
	ts.Require().NoError(err)

	rows := ts.fetchRows()
	ts.Require().Len(rows, 2)

	ts.Equal(uint64(1), rows[0].ID)
	ts.Equal(int16(3), rows[0].RemainingAttempts)
	ts.Equal(itemstatus.Ready, rows[0].Status)
	ts.WithinDuration(time.Now(), rows[0].UpdatedAt, time.Minute)

	ts.Equal(uint64(2), rows[1].ID)
	ts.Equal(int16(5), rows[1].RemainingAttempts)
	ts.Equal(itemstatus.Ready, rows[1].Status)
	ts.WithinDuration(time.Now().Add(time.Hour), rows[1].UpdatedAt, time.Minute)
}

// Test_InsertWhenEmpty - пустой список не доходит до БД и не ошибка.
func (ts *QueuePostgresTestSuite) Test_InsertWhenEmpty() {
	ts.Require().NoError(ts.repo.Insert(ts.ctx, nil))
}

// Test_FetchAndUpdateStatusReadyToProcessing - захватываются не более limit готовых записей
// в порядке очерёдности и переводятся в PROCESSING; отложенные и записи в других статусах
// не захватываются.
func (ts *QueuePostgresTestSuite) Test_FetchAndUpdateStatusReadyToProcessing() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Queue/FetchAndUpdateStatusReadyToProcessing")

	ids, err := ts.repo.FetchAndUpdateStatusReadyToProcessing(ts.ctx, 2)
	ts.Require().NoError(err)
	ts.ElementsMatch([]uint64{1, 2}, ids)

	ids, err = ts.repo.FetchAndUpdateStatusReadyToProcessing(ts.ctx, 10)
	ts.Require().NoError(err)
	ts.Equal([]uint64{3}, ids)

	ts.Equal(
		[]itemstatus.Enum{
			itemstatus.Processing, itemstatus.Processing, itemstatus.Processing,
			itemstatus.Ready, itemstatus.Processing, itemstatus.Retry,
		},
		ts.fetchStatuses(),
	)
}

// Test_FetchAndUpdateStatusReadyToProcessingWhenEmpty - когда захватывать нечего - пустой срез без ошибки.
func (ts *QueuePostgresTestSuite) Test_FetchAndUpdateStatusReadyToProcessingWhenEmpty() {
	ids, err := ts.repo.FetchAndUpdateStatusReadyToProcessing(ts.ctx, 10)
	ts.Require().NoError(err)
	ts.Empty(ids)
}

// Test_UpdateStatusProcessingToReady - в READY возвращаются только записи из списка,
// находящиеся в PROCESSING; число попыток не меняется.
func (ts *QueuePostgresTestSuite) Test_UpdateStatusProcessingToReady() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Queue/UpdateStatusProcessingToReady")

	ts.Require().NoError(ts.repo.UpdateStatusProcessingToReady(ts.ctx, []uint64{1, 3, 4, 99}))

	ts.Equal(
		[]itemstatus.Enum{itemstatus.Ready, itemstatus.Processing, itemstatus.Ready, itemstatus.Retry},
		ts.fetchStatuses(),
	)
	ts.Equal(int16(3), ts.fetchRows()[0].RemainingAttempts)
}

// Test_UpdateStatusProcessingToReadyWhenNotProcessing - если ни одна запись из списка не находится
// в PROCESSING, возвращается ErrEventStorageRecordsNotAffected (на нём вызывающий строит
// идемпотентную отмену обработки).
func (ts *QueuePostgresTestSuite) Test_UpdateStatusProcessingToReadyWhenNotProcessing() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Queue/UpdateStatusProcessingToReady")

	err := ts.repo.UpdateStatusProcessingToReady(ts.ctx, []uint64{3, 4, 99})
	ts.Require().ErrorIs(err, errors.ErrEventStorageRecordsNotAffected)
}

// Test_UpdateStatusProcessingToRetry - запись из PROCESSING переводится в RETRY с расходом попытки.
func (ts *QueuePostgresTestSuite) Test_UpdateStatusProcessingToRetry() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Queue/UpdateStatusProcessingToRetry")

	ts.Require().NoError(ts.repo.UpdateStatusProcessingToRetry(ts.ctx, 1))

	row := ts.fetchRows()[0]
	ts.Equal(itemstatus.Retry, row.Status)
	ts.Equal(int16(2), row.RemainingAttempts)
}

// Test_UpdateStatusProcessingToRetryWhenNotProcessing - запись не в PROCESSING (как и неизвестная)
// не меняется: возвращается ErrEventStorageNoRecordFound.
func (ts *QueuePostgresTestSuite) Test_UpdateStatusProcessingToRetryWhenNotProcessing() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Queue/UpdateStatusProcessingToRetry")

	err := ts.repo.UpdateStatusProcessingToRetry(ts.ctx, 2)
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)

	err = ts.repo.UpdateStatusProcessingToRetry(ts.ctx, 99)
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)

	row := ts.fetchRows()[1]
	ts.Equal(itemstatus.Ready, row.Status)
	ts.Equal(int16(3), row.RemainingAttempts)
}

// Test_UpdateStatusProcessingToRetryByTimeout - подвисшие в PROCESSING дольше timeout записи
// переводятся в RETRY не более limit за вызов, начиная с самых давних; попытки не расходуются.
func (ts *QueuePostgresTestSuite) Test_UpdateStatusProcessingToRetryByTimeout() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Queue/UpdateStatusProcessingToRetryByTimeout")

	ids, err := ts.repo.UpdateStatusProcessingToRetryByTimeout(ts.ctx, time.Hour, 1)
	ts.Require().NoError(err)
	ts.Equal([]uint64{1}, ids)

	ids, err = ts.repo.UpdateStatusProcessingToRetryByTimeout(ts.ctx, time.Hour, 10)
	ts.Require().NoError(err)
	ts.Equal([]uint64{2}, ids)

	ts.Equal(
		[]itemstatus.Enum{itemstatus.Retry, itemstatus.Retry, itemstatus.Processing, itemstatus.Ready},
		ts.fetchStatuses(),
	)
	ts.Equal(int16(3), ts.fetchRows()[0].RemainingAttempts)
}

// Test_UpdateStatusRetryToReady - записи, пробывшие в RETRY не меньше delayed и имеющие попытки,
// переводятся в READY не более limit за вызов, начиная с самых давних.
func (ts *QueuePostgresTestSuite) Test_UpdateStatusRetryToReady() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Queue/UpdateStatusRetryToReady")

	ids, err := ts.repo.UpdateStatusRetryToReady(ts.ctx, time.Hour, 1)
	ts.Require().NoError(err)
	ts.Equal([]uint64{1}, ids)

	ids, err = ts.repo.UpdateStatusRetryToReady(ts.ctx, time.Hour, 10)
	ts.Require().NoError(err)
	ts.Equal([]uint64{2}, ids)

	ts.Equal(
		[]itemstatus.Enum{itemstatus.Ready, itemstatus.Ready, itemstatus.Retry, itemstatus.Retry, itemstatus.Processing},
		ts.fetchStatuses(),
	)
}

// Test_DeleteRetryWithoutAttempts - из очереди удаляются записи в RETRY с исчерпанными попытками
// не более limit за вызов, начиная с самых давних; возвращаются их ID.
func (ts *QueuePostgresTestSuite) Test_DeleteRetryWithoutAttempts() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Queue/DeleteRetryWithoutAttempts")

	ids, err := ts.repo.DeleteRetryWithoutAttempts(ts.ctx, 1)
	ts.Require().NoError(err)
	ts.Equal([]uint64{1}, ids)

	ids, err = ts.repo.DeleteRetryWithoutAttempts(ts.ctx, 10)
	ts.Require().NoError(err)
	ts.Equal([]uint64{2}, ids)

	ids, err = ts.repo.DeleteRetryWithoutAttempts(ts.ctx, 10)
	ts.Require().NoError(err)
	ts.Empty(ids)

	ts.Equal([]itemstatus.Enum{itemstatus.Retry, itemstatus.Processing}, ts.fetchStatuses())
}

// Test_Delete - запись удаляется, если находится в указанном статусе.
func (ts *QueuePostgresTestSuite) Test_Delete() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Queue/Delete")

	ts.Require().NoError(ts.repo.Delete(ts.ctx, 1, itemstatus.Processing))

	rows := ts.fetchRows()
	ts.Require().Len(rows, 1)
	ts.Equal(uint64(2), rows[0].ID)
}

// Test_DeleteWhenOtherStatus - запись в другом статусе (как и неизвестная) не удаляется:
// возвращается ErrEventStorageNoRecordFound.
func (ts *QueuePostgresTestSuite) Test_DeleteWhenOtherStatus() {
	ts.pgt.ApplyFixtures(ts.T(), "testdata/Queue/Delete")

	err := ts.repo.Delete(ts.ctx, 2, itemstatus.Processing)
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)

	err = ts.repo.Delete(ts.ctx, 99, itemstatus.Processing)
	ts.Require().ErrorIs(err, errors.ErrEventStorageNoRecordFound)

	ts.Equal(2, ts.pgt.CountRows(ts.T(), ts.ctx, queueTableName))
}

// fetchRows - все строки очереди в порядке item_id.
func (ts *QueuePostgresTestSuite) fetchRows() []queueRow {
	rows, err := ts.pgt.ConnManager().Conn(ts.ctx).Query(
		ts.ctx,
		`SELECT item_id, remaining_attempts, item_status, updated_at FROM `+queueTableName+` ORDER BY item_id;`,
	)
	ts.Require().NoError(err)

	defer rows.Close()

	var out []queueRow

	for rows.Next() {
		var r queueRow

		ts.Require().NoError(rows.Scan(&r.ID, &r.RemainingAttempts, &r.Status, &r.UpdatedAt))

		out = append(out, r)
	}

	ts.Require().NoError(rows.Err())

	return out
}

// fetchStatuses - статусы всех строк очереди в порядке item_id.
func (ts *QueuePostgresTestSuite) fetchStatuses() []itemstatus.Enum {
	rows := ts.fetchRows()
	statuses := make([]itemstatus.Enum, 0, len(rows))

	for _, row := range rows {
		statuses = append(statuses, row.Status)
	}

	return statuses
}
