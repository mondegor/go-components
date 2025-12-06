package move

import (
	"context"
	"errors"

	"github.com/mondegor/go-storage/mrentity"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrargs"
	"github.com/mondegor/go-sysmess/mrerr"
	"github.com/mondegor/go-sysmess/mrerr/mr"
	"github.com/mondegor/go-sysmess/mrevent"

	"github.com/mondegor/go-components/mrordering"
	"github.com/mondegor/go-components/mrordering/entity"
)

const (
	orderIndexStep mrentity.ZeronullUint64 = 1024 * 1024
)

type (
	// NodeMover - объект, который обращаясь напрямую к служебным полям таблиц других репозиториев,
	// позволяет организовать управление порядком следования элементов этих репозиториев.
	// А именно позволяет вставлять элементы на нужную позицию, перемещать и отвязывать от их от текущих позиций.
	NodeMover struct {
		storage      mrordering.Storage
		eventEmitter mrevent.Emitter
		errorWrapper mrerr.UseCaseErrorWrapper
	}
)

// New - создаёт объект NodeMover.
func New(
	storage mrordering.Storage,
	eventEmitter mrevent.Emitter,
	errorWrapper mrerr.UseCaseErrorWrapper,
) *NodeMover {
	return &NodeMover{
		storage:      storage,
		eventEmitter: mrevent.NewSourceEmitter(eventEmitter, "mrordering.NodeMover"),
		errorWrapper: mrerr.NewUseCaseErrorWrapper(errorWrapper, "mrordering.NodeMover"),
	}
}

// InsertToFirst - вставляет указанный элемент на первое место отсортированного списка с учётом указанного условия.
// Использовать если есть уверенность, что элемент ещё не привязан к списку (например, он только что был создан).
func (co *NodeMover) InsertToFirst(ctx context.Context, nodeID uint64, condition mrstorage.SQLPartFunc) error {
	if nodeID == 0 {
		return mr.ErrUseCaseIncorrectInputData.New("nodeId is zero")
	}

	firstNode, err := co.storage.FetchFirstNode(ctx, condition)
	if err != nil {
		return co.wrapErrorMustEntityExists(err)
	}

	if nodeID == firstNode.ID {
		return mr.ErrUseCaseIncorrectInputData.New("nodeId=firstNode.Id", "nodeId", nodeID)
	}

	if err = co.storage.UpdateNodePrevID(ctx, firstNode.ID, mrentity.ZeronullUint64(nodeID), condition); err != nil {
		return co.wrapErrorMustEntityExists(err)
	}

	currentNode := entity.Node{
		ID:         nodeID,
		PrevID:     0,
		NextID:     mrentity.ZeronullUint64(firstNode.ID),
		OrderIndex: firstNode.OrderIndex / 2,
	}

	if currentNode.OrderIndex == 0 {
		if err = co.storage.RecalcOrderIndex(ctx, 0, 2*uint64(orderIndexStep), condition); err != nil {
			return co.errorWrapper.WrapErrorFailed(err)
		}

		currentNode.OrderIndex = orderIndexStep
	}

	if err = co.storage.UpdateNode(ctx, currentNode, condition); err != nil {
		return co.errorWrapper.WrapErrorNotFoundOrFailed(err)
	}

	co.eventEmitter.Emit(ctx, "InsertToFirst", mrargs.Group{"id": nodeID})

	return nil
}

// InsertToLast - вставляет указанный элемент на последнее место отсортированного списка с учётом указанного условия.
// Использовать если есть уверенность, что элемент ещё не привязан к списку (например, он только что был создан).
func (co *NodeMover) InsertToLast(ctx context.Context, nodeID uint64, condition mrstorage.SQLPartFunc) error {
	if nodeID == 0 {
		return mr.ErrUseCaseIncorrectInputData.New("nodeId is zero")
	}

	lastNode, err := co.storage.FetchLastNode(ctx, condition)
	if err != nil {
		return co.wrapErrorMustEntityExists(err)
	}

	if nodeID == lastNode.ID {
		return mr.ErrUseCaseIncorrectInputData.New("nodeId=lastNode.Id", "nodeId", nodeID)
	}

	if err = co.storage.UpdateNodeNextID(ctx, lastNode.ID, mrentity.ZeronullUint64(nodeID), condition); err != nil {
		return co.wrapErrorMustEntityExists(err)
	}

	currentNode := entity.Node{
		ID:         nodeID,
		PrevID:     mrentity.ZeronullUint64(lastNode.ID),
		NextID:     0,
		OrderIndex: lastNode.OrderIndex + orderIndexStep,
	}

	if err = co.storage.UpdateNode(ctx, currentNode, condition); err != nil {
		return co.errorWrapper.WrapErrorNotFoundOrFailed(err)
	}

	co.eventEmitter.Emit(ctx, "InsertToLast", mrargs.Group{"id": nodeID})

	return nil
}

// MoveToFirst - перемещает указанный элемент на первое место отсортированного списка с учётом указанного условия.
func (co *NodeMover) MoveToFirst(ctx context.Context, nodeID uint64, condition mrstorage.SQLPartFunc) error {
	if nodeID == 0 {
		return mr.ErrUseCaseIncorrectInputData.New("nodeId is zero")
	}

	firstNode, err := co.storage.FetchFirstNode(ctx, condition)
	if err != nil {
		return co.wrapErrorMustEntityExists(err)
	}

	if firstNode.ID == nodeID {
		if firstNode.OrderIndex == 0 {
			currentNode := entity.Node{
				ID:         nodeID,
				OrderIndex: orderIndexStep,
			}

			if err = co.storage.UpdateNode(ctx, currentNode, condition); err != nil {
				return co.wrapErrorMustEntityExists(err)
			}
		}

		return nil
	}

	currentNode, err := co.storage.FetchNode(ctx, nodeID, condition)
	if err != nil {
		return co.errorWrapper.WrapErrorNotFoundOrFailed(err)
	}

	if uint64(currentNode.NextID) == firstNode.ID {
		return mr.ErrInternal.New().WithAttr(
			"node",
			mrargs.Group{
				"currentNode.Id":                  currentNode.ID,
				"currentNode.NextId=firstNode.Id": currentNode.NextID,
			},
		)
	}

	if err = co.storage.UpdateNodePrevID(ctx, firstNode.ID, mrentity.ZeronullUint64(currentNode.ID), condition); err != nil {
		return co.wrapErrorMustEntityExists(err)
	}

	if currentNode.PrevID > 0 {
		if err = co.storage.UpdateNodeNextID(ctx, uint64(currentNode.PrevID), currentNode.NextID, condition); err != nil {
			return co.wrapErrorMustEntityExists(err)
		}
	}

	if currentNode.NextID > 0 {
		if err = co.storage.UpdateNodePrevID(ctx, uint64(currentNode.NextID), currentNode.PrevID, condition); err != nil {
			return co.wrapErrorMustEntityExists(err)
		}
	}

	currentNode.PrevID = 0
	currentNode.NextID = mrentity.ZeronullUint64(firstNode.ID)
	currentNode.OrderIndex = firstNode.OrderIndex / 2

	if currentNode.OrderIndex == 0 {
		if err = co.storage.RecalcOrderIndex(ctx, 0, 2*uint64(orderIndexStep), condition); err != nil {
			return co.errorWrapper.WrapErrorFailed(err)
		}

		currentNode.OrderIndex = orderIndexStep
	}

	if err = co.storage.UpdateNode(ctx, currentNode, condition); err != nil {
		return co.wrapErrorMustEntityExists(err)
	}

	co.eventEmitter.Emit(ctx, "MoveToFirst", mrargs.Group{"id": nodeID})

	return nil
}

// MoveToLast - перемещает указанный элемент на последнее место с учётом указанного условия.
func (co *NodeMover) MoveToLast(ctx context.Context, nodeID uint64, condition mrstorage.SQLPartFunc) error {
	if nodeID == 0 {
		return mr.ErrUseCaseIncorrectInputData.New("nodeId is zero")
	}

	lastNode, err := co.storage.FetchLastNode(ctx, condition)
	if err != nil {
		return co.wrapErrorMustEntityExists(err)
	}

	if lastNode.ID == nodeID {
		if lastNode.OrderIndex == 0 {
			currentNode := entity.Node{
				ID:         nodeID,
				OrderIndex: orderIndexStep,
			}

			if err = co.storage.UpdateNode(ctx, currentNode, condition); err != nil {
				return co.wrapErrorMustEntityExists(err)
			}
		}

		return nil
	}

	currentNode, err := co.storage.FetchNode(ctx, nodeID, condition)
	if err != nil {
		return co.errorWrapper.WrapErrorNotFoundOrFailed(err)
	}

	if lastNode.ID > 0 {
		if uint64(currentNode.PrevID) == lastNode.ID {
			return mr.ErrInternal.New().WithAttr(
				"node",
				mrargs.Group{
					"currentNode.Id":                 currentNode.ID,
					"currentNode.PrevId=lastNode.Id": currentNode.PrevID,
				},
			)
		}

		if err = co.storage.UpdateNodeNextID(ctx, lastNode.ID, mrentity.ZeronullUint64(currentNode.ID), condition); err != nil {
			return co.wrapErrorMustEntityExists(err)
		}
	}

	if currentNode.PrevID > 0 {
		if err = co.storage.UpdateNodeNextID(ctx, uint64(currentNode.PrevID), currentNode.NextID, condition); err != nil {
			return co.wrapErrorMustEntityExists(err)
		}
	}

	if currentNode.NextID > 0 {
		if err = co.storage.UpdateNodePrevID(ctx, uint64(currentNode.NextID), currentNode.PrevID, condition); err != nil {
			return co.wrapErrorMustEntityExists(err)
		}
	}

	currentNode.PrevID = mrentity.ZeronullUint64(lastNode.ID)
	currentNode.NextID = 0
	currentNode.OrderIndex = lastNode.OrderIndex + orderIndexStep

	if err = co.storage.UpdateNode(ctx, currentNode, condition); err != nil {
		return co.wrapErrorMustEntityExists(err)
	}

	co.eventEmitter.Emit(ctx, "MoveToLast", mrargs.Group{"id": nodeID})

	return nil
}

// MoveAfterID - перемещает указанный элемент после указанного элемента с учётом указанного условия.
// Если afterNodeID = 0, то элемент будет перемещён на первое место.
func (co *NodeMover) MoveAfterID(ctx context.Context, nodeID, afterNodeID uint64, condition mrstorage.SQLPartFunc) error {
	if afterNodeID == 0 {
		return co.MoveToFirst(ctx, nodeID, condition)
	}

	if nodeID == 0 {
		return mr.ErrUseCaseIncorrectInputData.New("nodeId is zero")
	}

	if nodeID == afterNodeID {
		return mr.ErrUseCaseIncorrectInputData.New("nodeId=afterNodeId", "nodeId", nodeID)
	}

	currentNode, err := co.storage.FetchNode(ctx, nodeID, condition)
	if err != nil {
		return co.errorWrapper.WrapErrorNotFoundOrFailed(err)
	}

	if uint64(currentNode.PrevID) == afterNodeID {
		return nil
	}

	afterNode, err := co.storage.FetchNode(ctx, afterNodeID, condition)
	if err != nil {
		if errors.Is(err, mr.ErrStorageNoRowFound) {
			return mrordering.ErrAfterNodeNotFound.Wrap(err, afterNodeID)
		}

		return co.errorWrapper.WrapErrorFailed(err)
	}

	afterNextNode := entity.Node{
		ID: uint64(afterNode.NextID),
	}

	if afterNextNode.ID > 0 {
		if afterNextNode, err = co.storage.FetchNode(ctx, afterNextNode.ID, condition); err != nil {
			return co.wrapErrorMustEntityExists(err)
		}
	}

	if err = co.storage.UpdateNodeNextID(ctx, afterNode.ID, mrentity.ZeronullUint64(currentNode.ID), condition); err != nil {
		return co.wrapErrorMustEntityExists(err)
	}

	if afterNextNode.ID > 0 {
		if err = co.storage.UpdateNodePrevID(ctx, afterNextNode.ID, mrentity.ZeronullUint64(currentNode.ID), condition); err != nil {
			return co.wrapErrorMustEntityExists(err)
		}
	}

	if currentNode.PrevID > 0 {
		if err = co.storage.UpdateNodeNextID(ctx, uint64(currentNode.PrevID), currentNode.NextID, condition); err != nil {
			return co.wrapErrorMustEntityExists(err)
		}
	}

	if currentNode.NextID > 0 {
		if err = co.storage.UpdateNodePrevID(ctx, uint64(currentNode.NextID), currentNode.PrevID, condition); err != nil {
			return co.wrapErrorMustEntityExists(err)
		}
	}

	currentNode.PrevID = mrentity.ZeronullUint64(afterNode.ID)
	currentNode.NextID = mrentity.ZeronullUint64(afterNextNode.ID)
	currentNode.OrderIndex = (afterNode.OrderIndex + afterNextNode.OrderIndex) / 2

	if currentNode.OrderIndex <= afterNode.OrderIndex {
		if afterNextNode.ID > 0 {
			if err := co.storage.RecalcOrderIndex(ctx, uint64(afterNode.OrderIndex), uint64(orderIndexStep)*2, condition); err != nil {
				return co.wrapErrorMustEntityExists(err)
			}
		}

		currentNode.OrderIndex = afterNode.OrderIndex + orderIndexStep
	}

	if err := co.storage.UpdateNode(ctx, currentNode, condition); err != nil {
		return co.wrapErrorMustEntityExists(err)
	}

	co.eventEmitter.Emit(ctx, "MoveAfterId", mrargs.Group{"id": nodeID, "afterId": afterNodeID})

	return nil
}

// Unlink - отвязывает указанный элемент находящимся в отсортированном списке с учётом указанного условия.
func (co *NodeMover) Unlink(ctx context.Context, nodeID uint64, condition mrstorage.SQLPartFunc) error {
	if nodeID == 0 {
		return mr.ErrUseCaseIncorrectInputData.New("nodeId is zero")
	}

	currentNode, err := co.storage.FetchNode(ctx, nodeID, condition)
	if err != nil {
		return co.errorWrapper.WrapErrorNotFoundOrFailed(err)
	}

	if currentNode.PrevID == 0 &&
		currentNode.NextID == 0 &&
		currentNode.OrderIndex == 0 {
		return nil
	}

	if currentNode.PrevID > 0 {
		if err = co.storage.UpdateNodeNextID(ctx, uint64(currentNode.PrevID), currentNode.NextID, condition); err != nil {
			return co.wrapErrorMustEntityExists(err)
		}
	}

	if currentNode.NextID > 0 {
		if err = co.storage.UpdateNodePrevID(ctx, uint64(currentNode.NextID), currentNode.PrevID, condition); err != nil {
			return co.wrapErrorMustEntityExists(err)
		}
	}

	currentNode.PrevID = 0
	currentNode.NextID = 0
	currentNode.OrderIndex = 0

	if err = co.storage.UpdateNode(ctx, currentNode, condition); err != nil {
		return co.wrapErrorMustEntityExists(err)
	}

	co.eventEmitter.Emit(ctx, "Unlink", mrargs.Group{"id": nodeID})

	return nil
}

func (co *NodeMover) wrapErrorMustEntityExists(err error) error {
	if errors.Is(err, mr.ErrStorageNoRowFound) {
		return mr.ErrInternal.Wrap(err)
	}

	return co.errorWrapper.WrapErrorFailed(err)
}
