package service_test

import (
	"context"
	"testing"

	"github.com/mondegor/go-core/mrentity"
	"github.com/mondegor/go-core/mrevent"
	"github.com/stretchr/testify/suite"
	"go.uber.org/mock/gomock"

	"github.com/mondegor/go-components/mrordering"
	"github.com/mondegor/go-components/mrordering/entity"
	"github.com/mondegor/go-components/mrordering/service"
	"github.com/mondegor/go-components/mrordering/service/mock"
)

//go:generate mockgen -source=node_mover.go -destination=mock/node_mover.go -package=mock

// orderIndexStep - шаг order_index между соседними элементами, который назначает NodeMover.
const orderIndexStep mrentity.ZeronullUint64 = 1024 * 1024

type NodeMoverTestSuite struct {
	suite.Suite

	ctx     context.Context
	storage *mock.MocknodeStorage
	mover   *service.NodeMover
}

func TestNodeMoverTestSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, new(NodeMoverTestSuite))
}

func (ts *NodeMoverTestSuite) SetupTest() {
	ts.ctx = context.Background()
	ts.storage = mock.NewMocknodeStorage(gomock.NewController(ts.T()))
	ts.mover = service.New(ts.storage, mrevent.NewEmitter())
}

// Test_PrependWhenEmpty - в пустой список элемент вставляется первым без обращения к соседям.
func (ts *NodeMoverTestSuite) Test_PrependWhenEmpty() {
	ts.storage.EXPECT().FetchFirstNode(gomock.Any(), gomock.Any()).Return(entity.Node{}, nil)
	ts.storage.EXPECT().UpdateNode(gomock.Any(), entity.Node{ID: 5, OrderIndex: orderIndexStep}, gomock.Any()).Return(nil)

	ts.Require().NoError(ts.mover.Prepend(ts.ctx, 5, nil))
}

// Test_Prepend - элемент вставляется перед первым элементом списка, тот получает его соседом.
func (ts *NodeMoverTestSuite) Test_Prepend() {
	ts.storage.EXPECT().FetchFirstNode(gomock.Any(), gomock.Any()).Return(entity.Node{ID: 1, NextID: 2, OrderIndex: 2 * orderIndexStep}, nil)
	ts.storage.EXPECT().UpdateNodePrevID(gomock.Any(), uint64(1), mrentity.ZeronullUint64(5), gomock.Any()).Return(nil)
	ts.storage.EXPECT().UpdateNode(gomock.Any(), entity.Node{ID: 5, NextID: 1, OrderIndex: orderIndexStep}, gomock.Any()).Return(nil)

	ts.Require().NoError(ts.mover.Prepend(ts.ctx, 5, nil))
}

// Test_AppendWhenEmpty - в пустой список элемент вставляется последним без обращения к соседям.
func (ts *NodeMoverTestSuite) Test_AppendWhenEmpty() {
	ts.storage.EXPECT().FetchLastNode(gomock.Any(), gomock.Any()).Return(entity.Node{}, nil)
	ts.storage.EXPECT().UpdateNode(gomock.Any(), entity.Node{ID: 5, OrderIndex: orderIndexStep}, gomock.Any()).Return(nil)

	ts.Require().NoError(ts.mover.Append(ts.ctx, 5, nil))
}

// Test_Append - элемент вставляется после последнего элемента списка, тот получает его соседом.
func (ts *NodeMoverTestSuite) Test_Append() {
	ts.storage.EXPECT().FetchLastNode(gomock.Any(), gomock.Any()).Return(entity.Node{ID: 3, PrevID: 2, OrderIndex: 3 * orderIndexStep}, nil)
	ts.storage.EXPECT().UpdateNodeNextID(gomock.Any(), uint64(3), mrentity.ZeronullUint64(5), gomock.Any()).Return(nil)
	ts.storage.EXPECT().UpdateNode(gomock.Any(), entity.Node{ID: 5, PrevID: 3, OrderIndex: 4 * orderIndexStep}, gomock.Any()).Return(nil)

	ts.Require().NoError(ts.mover.Append(ts.ctx, 5, nil))
}

// Test_MoveToFirstWhenEmpty - элемент вне списка становится первым в пустом списке без обращения к соседям.
func (ts *NodeMoverTestSuite) Test_MoveToFirstWhenEmpty() {
	ts.storage.EXPECT().FetchFirstNode(gomock.Any(), gomock.Any()).Return(entity.Node{}, nil)
	ts.storage.EXPECT().FetchNode(gomock.Any(), uint64(5), gomock.Any()).Return(entity.Node{ID: 5}, nil)
	ts.storage.EXPECT().UpdateNode(gomock.Any(), entity.Node{ID: 5, OrderIndex: orderIndexStep}, gomock.Any()).Return(nil)

	ts.Require().NoError(ts.mover.MoveToFirst(ts.ctx, 5, nil))
}

// Test_MoveToFirstWhenAlreadyFirst - первый элемент остаётся на месте без изменений.
func (ts *NodeMoverTestSuite) Test_MoveToFirstWhenAlreadyFirst() {
	ts.storage.EXPECT().FetchFirstNode(gomock.Any(), gomock.Any()).Return(entity.Node{ID: 5, NextID: 6, OrderIndex: orderIndexStep}, nil)

	ts.Require().NoError(ts.mover.MoveToFirst(ts.ctx, 5, nil))
}

// Test_MoveToFirst - элемент из середины списка переносится в начало: его прежние соседи связываются
// друг с другом, бывший первый элемент получает его предыдущим.
func (ts *NodeMoverTestSuite) Test_MoveToFirst() {
	ts.storage.EXPECT().FetchFirstNode(gomock.Any(), gomock.Any()).Return(entity.Node{ID: 1, NextID: 2, OrderIndex: orderIndexStep}, nil)
	ts.storage.EXPECT().FetchNode(gomock.Any(), uint64(5), gomock.Any()).Return(entity.Node{ID: 5, PrevID: 2, NextID: 6, OrderIndex: 3 * orderIndexStep}, nil)
	ts.storage.EXPECT().UpdateNodePrevID(gomock.Any(), uint64(1), mrentity.ZeronullUint64(5), gomock.Any()).Return(nil)
	ts.storage.EXPECT().UpdateNodeNextID(gomock.Any(), uint64(2), mrentity.ZeronullUint64(6), gomock.Any()).Return(nil)
	ts.storage.EXPECT().UpdateNodePrevID(gomock.Any(), uint64(6), mrentity.ZeronullUint64(2), gomock.Any()).Return(nil)
	ts.storage.EXPECT().UpdateNode(gomock.Any(), entity.Node{ID: 5, NextID: 1, OrderIndex: orderIndexStep / 2}, gomock.Any()).Return(nil)

	ts.Require().NoError(ts.mover.MoveToFirst(ts.ctx, 5, nil))
}

// Test_MoveToLastWhenEmpty - элемент вне списка становится последним в пустом списке без обращения к соседям.
func (ts *NodeMoverTestSuite) Test_MoveToLastWhenEmpty() {
	ts.storage.EXPECT().FetchLastNode(gomock.Any(), gomock.Any()).Return(entity.Node{}, nil)
	ts.storage.EXPECT().FetchNode(gomock.Any(), uint64(5), gomock.Any()).Return(entity.Node{ID: 5}, nil)
	ts.storage.EXPECT().UpdateNode(gomock.Any(), entity.Node{ID: 5, OrderIndex: orderIndexStep}, gomock.Any()).Return(nil)

	ts.Require().NoError(ts.mover.MoveToLast(ts.ctx, 5, nil))
}

// Test_MoveToLastWhenAlreadyLast - последний элемент остаётся на месте без изменений.
func (ts *NodeMoverTestSuite) Test_MoveToLastWhenAlreadyLast() {
	ts.storage.EXPECT().FetchLastNode(gomock.Any(), gomock.Any()).Return(entity.Node{ID: 5, PrevID: 4, OrderIndex: orderIndexStep}, nil)

	ts.Require().NoError(ts.mover.MoveToLast(ts.ctx, 5, nil))
}

// Test_MoveToLast - элемент из середины списка переносится в конец: его прежние соседи связываются
// друг с другом, бывший последний элемент получает его следующим.
func (ts *NodeMoverTestSuite) Test_MoveToLast() {
	ts.storage.EXPECT().FetchLastNode(gomock.Any(), gomock.Any()).Return(entity.Node{ID: 9, PrevID: 6, OrderIndex: 4 * orderIndexStep}, nil)
	ts.storage.EXPECT().FetchNode(gomock.Any(), uint64(5), gomock.Any()).Return(entity.Node{ID: 5, PrevID: 2, NextID: 6, OrderIndex: 2 * orderIndexStep}, nil)
	ts.storage.EXPECT().UpdateNodeNextID(gomock.Any(), uint64(9), mrentity.ZeronullUint64(5), gomock.Any()).Return(nil)
	ts.storage.EXPECT().UpdateNodeNextID(gomock.Any(), uint64(2), mrentity.ZeronullUint64(6), gomock.Any()).Return(nil)
	ts.storage.EXPECT().UpdateNodePrevID(gomock.Any(), uint64(6), mrentity.ZeronullUint64(2), gomock.Any()).Return(nil)
	ts.storage.EXPECT().UpdateNode(gomock.Any(), entity.Node{ID: 5, PrevID: 9, OrderIndex: 5 * orderIndexStep}, gomock.Any()).Return(nil)

	ts.Require().NoError(ts.mover.MoveToLast(ts.ctx, 5, nil))
}

// Test_MoveAfterIDWhenAfterZeroAndEmpty - перемещение «в начало» (afterNodeID = 0) работает и в пустом
// списке: элемент вне списка становится первым.
func (ts *NodeMoverTestSuite) Test_MoveAfterIDWhenAfterZeroAndEmpty() {
	ts.storage.EXPECT().FetchFirstNode(gomock.Any(), gomock.Any()).Return(entity.Node{}, nil)
	ts.storage.EXPECT().FetchNode(gomock.Any(), uint64(5), gomock.Any()).Return(entity.Node{ID: 5}, nil)
	ts.storage.EXPECT().UpdateNode(gomock.Any(), entity.Node{ID: 5, OrderIndex: orderIndexStep}, gomock.Any()).Return(nil)

	ts.Require().NoError(ts.mover.MoveAfterID(ts.ctx, 5, 0, nil))
}

// Test_MoveAfterIDWhenAfterNodeNotInList - элемент вне списка (например, мягко удалённый хостом)
// не может быть опорным: возвращается ErrAfterNodeNotFound, связи в списке не меняются.
func (ts *NodeMoverTestSuite) Test_MoveAfterIDWhenAfterNodeNotInList() {
	ts.storage.EXPECT().FetchNode(gomock.Any(), uint64(5), gomock.Any()).Return(entity.Node{ID: 5, PrevID: 2, NextID: 6, OrderIndex: 2 * orderIndexStep}, nil)
	ts.storage.EXPECT().FetchNode(gomock.Any(), uint64(7), gomock.Any()).Return(entity.Node{ID: 7}, nil)

	err := ts.mover.MoveAfterID(ts.ctx, 5, 7, nil)
	ts.Require().ErrorIs(err, mrordering.ErrAfterNodeNotFound)
}
