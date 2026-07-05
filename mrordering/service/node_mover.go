package service

import (
	"context"

	"github.com/mondegor/go-core/errors"
	"github.com/mondegor/go-core/mrentity"
	"github.com/mondegor/go-core/mrevent"
	"github.com/mondegor/go-core/mrstorage"
	"github.com/mondegor/go-core/util/conv"

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
		storage      nodeStorage
		eventEmitter mrevent.Emitter
		errorWrapper errors.Wrapper
	}

	nodeStorage interface {
		FetchNode(ctx context.Context, rowID uint64, condition mrstorage.SQLPartFunc) (entity.Node, error)
		FetchFirstNode(ctx context.Context, condition mrstorage.SQLPartFunc) (entity.Node, error)
		FetchLastNode(ctx context.Context, condition mrstorage.SQLPartFunc) (entity.Node, error)
		UpdateNode(ctx context.Context, row entity.Node, condition mrstorage.SQLPartFunc) error
		UpdateNodePrevID(ctx context.Context, rowID uint64, prevID mrentity.ZeronullUint64, condition mrstorage.SQLPartFunc) error
		UpdateNodeNextID(ctx context.Context, rowID uint64, nextID mrentity.ZeronullUint64, condition mrstorage.SQLPartFunc) error
		RecalcOrderIndex(ctx context.Context, minBorder, step uint64, condition mrstorage.SQLPartFunc) error
	}
)

// New - создаёт объект NodeMover.
func New(
	storage nodeStorage,
	eventEmitter mrevent.Emitter,
) *NodeMover {
	return &NodeMover{
		storage:      storage,
		eventEmitter: mrevent.EmitterWithSource(eventEmitter, "mrordering.NodeMover"),
		errorWrapper: errors.NewServiceRecordNotFoundWrapper(),
	}
}

// InsertToFirst - вставляет указанный элемент на первое место отсортированного списка с учётом указанного условия.
// Использовать если есть уверенность, что элемент ещё не привязан к списку (например, он только что был создан).
func (sv *NodeMover) InsertToFirst(ctx context.Context, nodeID uint64, condition mrstorage.SQLPartFunc) error {
	if nodeID == 0 {
		return errors.ErrIncorrectInputData.New("nodeId is zero")
	}

	firstNode, err := sv.storage.FetchFirstNode(ctx, condition)
	if err != nil {
		return sv.wrapErrorMustEntityExists(err)
	}

	if nodeID == firstNode.ID {
		return errors.ErrIncorrectInputData.New("nodeId=firstNode.Id", "nodeId", nodeID)
	}

	if err = sv.storage.UpdateNodePrevID(ctx, firstNode.ID, mrentity.ZeronullUint64(nodeID), condition); err != nil {
		return sv.wrapErrorMustEntityExists(err)
	}

	currentNode := entity.Node{
		ID:         nodeID,
		PrevID:     0,
		NextID:     mrentity.ZeronullUint64(firstNode.ID),
		OrderIndex: firstNode.OrderIndex / 2,
	}

	if currentNode.OrderIndex == 0 {
		if err = sv.storage.RecalcOrderIndex(ctx, 0, 2*uint64(orderIndexStep), condition); err != nil {
			return sv.errorWrapper.Wrap(err)
		}

		currentNode.OrderIndex = orderIndexStep
	}

	if err = sv.storage.UpdateNode(ctx, currentNode, condition); err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	sv.eventEmitter.Emit(ctx, "InsertToFirst", "nodeId", nodeID)

	return nil
}

// InsertToLast - вставляет указанный элемент на последнее место отсортированного списка с учётом указанного условия.
// Использовать если есть уверенность, что элемент ещё не привязан к списку (например, он только что был создан).
func (sv *NodeMover) InsertToLast(ctx context.Context, nodeID uint64, condition mrstorage.SQLPartFunc) error {
	if nodeID == 0 {
		return errors.ErrIncorrectInputData.New("nodeId is zero")
	}

	lastNode, err := sv.storage.FetchLastNode(ctx, condition)
	if err != nil {
		return sv.wrapErrorMustEntityExists(err)
	}

	if nodeID == lastNode.ID {
		return errors.ErrIncorrectInputData.New("nodeId=lastNode.Id", "nodeId", nodeID)
	}

	if err = sv.storage.UpdateNodeNextID(ctx, lastNode.ID, mrentity.ZeronullUint64(nodeID), condition); err != nil {
		return sv.wrapErrorMustEntityExists(err)
	}

	currentNode := entity.Node{
		ID:         nodeID,
		PrevID:     mrentity.ZeronullUint64(lastNode.ID),
		NextID:     0,
		OrderIndex: lastNode.OrderIndex + orderIndexStep,
	}

	if err = sv.storage.UpdateNode(ctx, currentNode, condition); err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	sv.eventEmitter.Emit(ctx, "InsertToLast", "nodeId", nodeID)

	return nil
}

// MoveToFirst - перемещает указанный элемент на первое место отсортированного списка с учётом указанного условия.
func (sv *NodeMover) MoveToFirst(ctx context.Context, nodeID uint64, condition mrstorage.SQLPartFunc) error {
	if nodeID == 0 {
		return errors.ErrIncorrectInputData.New("nodeId is zero")
	}

	firstNode, err := sv.storage.FetchFirstNode(ctx, condition)
	if err != nil {
		return sv.wrapErrorMustEntityExists(err)
	}

	if firstNode.ID == nodeID {
		if firstNode.OrderIndex == 0 {
			currentNode := entity.Node{
				ID:         nodeID,
				OrderIndex: orderIndexStep,
			}

			if err = sv.storage.UpdateNode(ctx, currentNode, condition); err != nil {
				return sv.wrapErrorMustEntityExists(err)
			}
		}

		return nil
	}

	currentNode, err := sv.storage.FetchNode(ctx, nodeID, condition)
	if err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	if uint64(currentNode.NextID) == firstNode.ID {
		return errors.NewInternalError(
			"currentNode.NextID = firstNode.ID",
			"node", conv.Group{
				"currentNode.Id":                  currentNode.ID,
				"currentNode.NextId=firstNode.Id": currentNode.NextID,
			},
		)
	}

	if err = sv.storage.UpdateNodePrevID(ctx, firstNode.ID, mrentity.ZeronullUint64(currentNode.ID), condition); err != nil {
		return sv.wrapErrorMustEntityExists(err)
	}

	if currentNode.PrevID > 0 {
		if err = sv.storage.UpdateNodeNextID(ctx, uint64(currentNode.PrevID), currentNode.NextID, condition); err != nil {
			return sv.wrapErrorMustEntityExists(err)
		}
	}

	if currentNode.NextID > 0 {
		if err = sv.storage.UpdateNodePrevID(ctx, uint64(currentNode.NextID), currentNode.PrevID, condition); err != nil {
			return sv.wrapErrorMustEntityExists(err)
		}
	}

	currentNode.PrevID = 0
	currentNode.NextID = mrentity.ZeronullUint64(firstNode.ID)
	currentNode.OrderIndex = firstNode.OrderIndex / 2

	if currentNode.OrderIndex == 0 {
		if err = sv.storage.RecalcOrderIndex(ctx, 0, 2*uint64(orderIndexStep), condition); err != nil {
			return sv.errorWrapper.Wrap(err)
		}

		currentNode.OrderIndex = orderIndexStep
	}

	if err = sv.storage.UpdateNode(ctx, currentNode, condition); err != nil {
		return sv.wrapErrorMustEntityExists(err)
	}

	sv.eventEmitter.Emit(ctx, "MoveToFirst", "nodeId", nodeID)

	return nil
}

// MoveToLast - перемещает указанный элемент на последнее место с учётом указанного условия.
func (sv *NodeMover) MoveToLast(ctx context.Context, nodeID uint64, condition mrstorage.SQLPartFunc) error {
	if nodeID == 0 {
		return errors.ErrIncorrectInputData.New("nodeId is zero")
	}

	lastNode, err := sv.storage.FetchLastNode(ctx, condition)
	if err != nil {
		return sv.wrapErrorMustEntityExists(err)
	}

	if lastNode.ID == nodeID {
		if lastNode.OrderIndex == 0 {
			currentNode := entity.Node{
				ID:         nodeID,
				OrderIndex: orderIndexStep,
			}

			if err = sv.storage.UpdateNode(ctx, currentNode, condition); err != nil {
				return sv.wrapErrorMustEntityExists(err)
			}
		}

		return nil
	}

	currentNode, err := sv.storage.FetchNode(ctx, nodeID, condition)
	if err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	if lastNode.ID > 0 {
		if uint64(currentNode.PrevID) == lastNode.ID {
			return errors.NewInternalError(
				"currentNode.PrevID = lastNode.ID",
				"node", conv.Group{
					"currentNode.Id":                 currentNode.ID,
					"currentNode.PrevId=lastNode.Id": currentNode.PrevID,
				},
			)
		}

		if err = sv.storage.UpdateNodeNextID(ctx, lastNode.ID, mrentity.ZeronullUint64(currentNode.ID), condition); err != nil {
			return sv.wrapErrorMustEntityExists(err)
		}
	}

	if currentNode.PrevID > 0 {
		if err = sv.storage.UpdateNodeNextID(ctx, uint64(currentNode.PrevID), currentNode.NextID, condition); err != nil {
			return sv.wrapErrorMustEntityExists(err)
		}
	}

	if currentNode.NextID > 0 {
		if err = sv.storage.UpdateNodePrevID(ctx, uint64(currentNode.NextID), currentNode.PrevID, condition); err != nil {
			return sv.wrapErrorMustEntityExists(err)
		}
	}

	currentNode.PrevID = mrentity.ZeronullUint64(lastNode.ID)
	currentNode.NextID = 0
	currentNode.OrderIndex = lastNode.OrderIndex + orderIndexStep

	if err = sv.storage.UpdateNode(ctx, currentNode, condition); err != nil {
		return sv.wrapErrorMustEntityExists(err)
	}

	sv.eventEmitter.Emit(ctx, "MoveToLast", "nodeId", nodeID)

	return nil
}

// MoveAfterID - перемещает указанный элемент после указанного элемента с учётом указанного условия.
// Если afterNodeID = 0, то элемент будет перемещён на первое место.
func (sv *NodeMover) MoveAfterID(ctx context.Context, nodeID, afterNodeID uint64, condition mrstorage.SQLPartFunc) error {
	if afterNodeID == 0 {
		return sv.MoveToFirst(ctx, nodeID, condition)
	}

	if nodeID == 0 {
		return errors.ErrIncorrectInputData.New("nodeId is zero")
	}

	if nodeID == afterNodeID {
		return errors.ErrIncorrectInputData.New("nodeId=afterNodeId", "nodeId", nodeID)
	}

	currentNode, err := sv.storage.FetchNode(ctx, nodeID, condition)
	if err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	if uint64(currentNode.PrevID) == afterNodeID {
		return nil
	}

	afterNode, err := sv.storage.FetchNode(ctx, afterNodeID, condition)
	if err != nil {
		if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
			return mrordering.ErrAfterNodeNotFound.New(afterNodeID)
		}

		return sv.errorWrapper.Wrap(err)
	}

	afterNextNode := entity.Node{
		ID: uint64(afterNode.NextID),
	}

	if afterNextNode.ID > 0 {
		if afterNextNode, err = sv.storage.FetchNode(ctx, afterNextNode.ID, condition); err != nil {
			return sv.wrapErrorMustEntityExists(err)
		}
	}

	if err = sv.storage.UpdateNodeNextID(ctx, afterNode.ID, mrentity.ZeronullUint64(currentNode.ID), condition); err != nil {
		return sv.wrapErrorMustEntityExists(err)
	}

	if afterNextNode.ID > 0 {
		if err = sv.storage.UpdateNodePrevID(ctx, afterNextNode.ID, mrentity.ZeronullUint64(currentNode.ID), condition); err != nil {
			return sv.wrapErrorMustEntityExists(err)
		}
	}

	if currentNode.PrevID > 0 {
		if err = sv.storage.UpdateNodeNextID(ctx, uint64(currentNode.PrevID), currentNode.NextID, condition); err != nil {
			return sv.wrapErrorMustEntityExists(err)
		}
	}

	if currentNode.NextID > 0 {
		if err = sv.storage.UpdateNodePrevID(ctx, uint64(currentNode.NextID), currentNode.PrevID, condition); err != nil {
			return sv.wrapErrorMustEntityExists(err)
		}
	}

	currentNode.PrevID = mrentity.ZeronullUint64(afterNode.ID)
	currentNode.NextID = mrentity.ZeronullUint64(afterNextNode.ID)
	currentNode.OrderIndex = (afterNode.OrderIndex + afterNextNode.OrderIndex) / 2

	if currentNode.OrderIndex <= afterNode.OrderIndex {
		if afterNextNode.ID > 0 {
			if err := sv.storage.RecalcOrderIndex(ctx, uint64(afterNode.OrderIndex), uint64(orderIndexStep)*2, condition); err != nil {
				return sv.wrapErrorMustEntityExists(err)
			}
		}

		currentNode.OrderIndex = afterNode.OrderIndex + orderIndexStep
	}

	if err := sv.storage.UpdateNode(ctx, currentNode, condition); err != nil {
		return sv.wrapErrorMustEntityExists(err)
	}

	sv.eventEmitter.Emit(ctx, "MoveAfterId", "nodeId", nodeID, "afterNodeId", afterNodeID)

	return nil
}

// Unlink - отвязывает указанный элемент находящимся в отсортированном списке с учётом указанного условия.
func (sv *NodeMover) Unlink(ctx context.Context, nodeID uint64, condition mrstorage.SQLPartFunc) error {
	if nodeID == 0 {
		return errors.ErrIncorrectInputData.New("nodeId is zero")
	}

	currentNode, err := sv.storage.FetchNode(ctx, nodeID, condition)
	if err != nil {
		return sv.errorWrapper.Wrap(err)
	}

	if currentNode.PrevID == 0 &&
		currentNode.NextID == 0 &&
		currentNode.OrderIndex == 0 {
		return nil
	}

	if currentNode.PrevID > 0 {
		if err = sv.storage.UpdateNodeNextID(ctx, uint64(currentNode.PrevID), currentNode.NextID, condition); err != nil {
			return sv.wrapErrorMustEntityExists(err)
		}
	}

	if currentNode.NextID > 0 {
		if err = sv.storage.UpdateNodePrevID(ctx, uint64(currentNode.NextID), currentNode.PrevID, condition); err != nil {
			return sv.wrapErrorMustEntityExists(err)
		}
	}

	currentNode.PrevID = 0
	currentNode.NextID = 0
	currentNode.OrderIndex = 0

	if err = sv.storage.UpdateNode(ctx, currentNode, condition); err != nil {
		return sv.wrapErrorMustEntityExists(err)
	}

	sv.eventEmitter.Emit(ctx, "Unlink", "nodeId", nodeID)

	return nil
}

func (sv *NodeMover) wrapErrorMustEntityExists(err error) error {
	if errors.Is(err, errors.ErrEventStorageNoRecordFound) {
		return errors.WrapInternalError(err, "entity not found")
	}

	return sv.errorWrapper.Wrap(err)
}
