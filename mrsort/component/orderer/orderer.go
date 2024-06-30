package orderer

import (
	"context"
	"errors"

	"github.com/mondegor/go-storage/mrentity"
	"github.com/mondegor/go-storage/mrstorage"
	"github.com/mondegor/go-sysmess/mrmsg"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrsender"
	"github.com/mondegor/go-webcore/mrtype"

	"github.com/mondegor/go-components/mrsort"
	"github.com/mondegor/go-components/mrsort/entity"
)

const (
	orderIndexStep int64 = 1024 * 1024
)

type (
	// Component - объект, который обращаясь напрямую к служебным полям таблиц других репозиториев,
	// позволяет организовать управление порядком следования элементов этих репозиториев.
	// А именно позволяет вставлять элементы на нужную позицию, перемещать и отвязывать от их от текущих позиций.
	Component struct {
		storage      mrsort.Storage
		eventEmitter mrsender.EventEmitter
		errorWrapper mrcore.UsecaseErrorWrapper
	}
)

// New - создаёт объект Component.
func New(storage mrsort.Storage, eventEmitter mrsender.EventEmitter, errorWrapper mrcore.UsecaseErrorWrapper) *Component {
	return &Component{
		storage:      storage,
		eventEmitter: eventEmitter,
		errorWrapper: errorWrapper,
	}
}

// WithMetaData - comment method.
func (co *Component) WithMetaData(meta mrstorage.MetaGetter) mrsort.Orderer {
	return &Component{
		storage:      co.storage.WithMetaData(meta),
		eventEmitter: co.eventEmitter,
	}
}

// InsertToFirst - comment method.
func (co *Component) InsertToFirst(ctx context.Context, nodeID mrtype.KeyInt32) error {
	if nodeID < 1 {
		return mrcore.ErrUseCaseIncorrectInputData.New("node", mrmsg.Data{"nodeId": nodeID})
	}

	firstNode, err := co.storage.FetchFirstNode(ctx)
	if err != nil {
		return co.wrapErrorMustLoad(err)
	}

	if nodeID == firstNode.ID {
		return mrcore.ErrUseCaseIncorrectInputData.New("node", mrmsg.Data{"nodeId=firstNode.Id": nodeID})
	}

	if err = co.storage.UpdateNodePrevID(ctx, firstNode.ID, mrentity.ZeronullInt32(nodeID)); err != nil {
		return co.wrapErrorMustStore(err)
	}

	currentNode := entity.Node{
		ID:         nodeID,
		PrevID:     0,
		NextID:     mrentity.ZeronullInt32(firstNode.ID),
		OrderIndex: firstNode.OrderIndex / 2,
	}

	if currentNode.OrderIndex < 1 {
		if err = co.storage.RecalcOrderIndex(ctx, 0, 2*orderIndexStep); err != nil {
			return co.wrapErrorMustStore(err)
		}

		currentNode.OrderIndex = mrentity.ZeronullInt64(orderIndexStep)
	}

	if err = co.storage.UpdateNode(ctx, currentNode); err != nil {
		return co.errorWrapper.WrapErrorNotFoundOrFailed(err, entity.ModelNameNode)
	}

	co.emitEvent(ctx, "InsertToFirst", mrmsg.Data{"id": nodeID})

	return nil
}

// InsertToLast - comment method.
func (co *Component) InsertToLast(ctx context.Context, nodeID mrtype.KeyInt32) error {
	if nodeID < 1 {
		return mrcore.ErrUseCaseIncorrectInputData.New("node", mrmsg.Data{"nodeId": nodeID})
	}

	lastNode, err := co.storage.FetchLastNode(ctx)
	if err != nil {
		return co.wrapErrorMustLoad(err)
	}

	if nodeID == lastNode.ID {
		return mrcore.ErrUseCaseIncorrectInputData.New("node", mrmsg.Data{"nodeId=lastNode.Id": nodeID})
	}

	if err = co.storage.UpdateNodeNextID(ctx, lastNode.ID, mrentity.ZeronullInt32(nodeID)); err != nil {
		return co.wrapErrorMustStore(err)
	}

	currentNode := entity.Node{
		ID:         nodeID,
		PrevID:     mrentity.ZeronullInt32(lastNode.ID),
		NextID:     0,
		OrderIndex: lastNode.OrderIndex + mrentity.ZeronullInt64(orderIndexStep),
	}

	if err = co.storage.UpdateNode(ctx, currentNode); err != nil {
		return co.errorWrapper.WrapErrorNotFoundOrFailed(err, entity.ModelNameNode)
	}

	co.emitEvent(ctx, "InsertToLast", mrmsg.Data{"id": nodeID})

	return nil
}

// MoveToFirst - comment method.
func (co *Component) MoveToFirst(ctx context.Context, nodeID mrtype.KeyInt32) error {
	if nodeID < 1 {
		return mrcore.ErrUseCaseIncorrectInputData.New("node", mrmsg.Data{"nodeId": nodeID})
	}

	firstNode, err := co.storage.FetchFirstNode(ctx)
	if err != nil {
		return co.wrapErrorMustLoad(err)
	}

	if firstNode.ID == nodeID {
		if firstNode.OrderIndex == 0 {
			currentNode := entity.Node{
				ID:         nodeID,
				OrderIndex: mrentity.ZeronullInt64(orderIndexStep),
			}

			if err = co.storage.UpdateNode(ctx, currentNode); err != nil {
				return co.wrapErrorMustStore(err)
			}
		}

		return nil
	}

	currentNode, err := co.storage.FetchNode(ctx, nodeID)
	if err != nil {
		return co.errorWrapper.WrapErrorNotFoundOrFailed(err, entity.ModelNameNode)
	}

	if mrtype.KeyInt32(currentNode.NextID) == firstNode.ID {
		return mrcore.ErrInternal.New().WithAttr(
			"node",
			mrmsg.Data{
				"currentNode.Id":                  currentNode.ID,
				"currentNode.NextId=firstNode.Id": currentNode.NextID,
			},
		)
	}

	if err = co.storage.UpdateNodePrevID(ctx, firstNode.ID, mrentity.ZeronullInt32(currentNode.ID)); err != nil {
		return co.wrapErrorMustStore(err)
	}

	if currentNode.PrevID > 0 {
		if err = co.storage.UpdateNodeNextID(ctx, mrtype.KeyInt32(currentNode.PrevID), currentNode.NextID); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	if currentNode.NextID > 0 {
		if err = co.storage.UpdateNodePrevID(ctx, mrtype.KeyInt32(currentNode.NextID), currentNode.PrevID); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	currentNode.PrevID = 0
	currentNode.NextID = mrentity.ZeronullInt32(firstNode.ID)
	currentNode.OrderIndex = firstNode.OrderIndex / 2

	if currentNode.OrderIndex < 1 {
		if err = co.storage.RecalcOrderIndex(ctx, 0, 2*orderIndexStep); err != nil {
			return co.wrapErrorMustStore(err)
		}

		currentNode.OrderIndex = mrentity.ZeronullInt64(orderIndexStep)
	}

	if err = co.storage.UpdateNode(ctx, currentNode); err != nil {
		return co.wrapErrorMustStore(err)
	}

	co.emitEvent(ctx, "MoveToFirst", mrmsg.Data{"id": nodeID})

	return nil
}

// MoveToLast - comment method.
func (co *Component) MoveToLast(ctx context.Context, nodeID mrtype.KeyInt32) error {
	if nodeID < 1 {
		return mrcore.ErrUseCaseIncorrectInputData.New("node", mrmsg.Data{"nodeId": nodeID})
	}

	lastNode, err := co.storage.FetchLastNode(ctx)
	if err != nil {
		return co.wrapErrorMustLoad(err)
	}

	if lastNode.ID == nodeID {
		if lastNode.OrderIndex == 0 {
			currentNode := entity.Node{
				ID:         nodeID,
				OrderIndex: mrentity.ZeronullInt64(orderIndexStep),
			}

			if err = co.storage.UpdateNode(ctx, currentNode); err != nil {
				return co.wrapErrorMustStore(err)
			}
		}

		return nil
	}

	currentNode, err := co.storage.FetchNode(ctx, nodeID)
	if err != nil {
		return co.errorWrapper.WrapErrorNotFoundOrFailed(err, entity.ModelNameNode)
	}

	if lastNode.ID > 0 {
		if mrtype.KeyInt32(currentNode.PrevID) == lastNode.ID {
			return mrcore.ErrInternal.New().WithAttr(
				"node",
				mrmsg.Data{
					"currentNode.Id":                 currentNode.ID,
					"currentNode.PrevId=lastNode.Id": currentNode.PrevID,
				},
			)
		}

		if err = co.storage.UpdateNodeNextID(ctx, lastNode.ID, mrentity.ZeronullInt32(currentNode.ID)); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	if currentNode.PrevID > 0 {
		if err = co.storage.UpdateNodeNextID(ctx, mrtype.KeyInt32(currentNode.PrevID), currentNode.NextID); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	if currentNode.NextID > 0 {
		if err = co.storage.UpdateNodePrevID(ctx, mrtype.KeyInt32(currentNode.NextID), currentNode.PrevID); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	currentNode.PrevID = mrentity.ZeronullInt32(lastNode.ID)
	currentNode.NextID = 0
	currentNode.OrderIndex = lastNode.OrderIndex + mrentity.ZeronullInt64(orderIndexStep)

	if err = co.storage.UpdateNode(ctx, currentNode); err != nil {
		return co.wrapErrorMustStore(err)
	}

	co.emitEvent(ctx, "MoveToLast", mrmsg.Data{"id": nodeID})

	return nil
}

// MoveAfterID - comment method.
func (co *Component) MoveAfterID(ctx context.Context, nodeID, afterNodeID mrtype.KeyInt32) error {
	if afterNodeID < 1 {
		return co.MoveToFirst(ctx, nodeID)
	}

	if nodeID < 1 {
		return mrcore.ErrUseCaseIncorrectInputData.New("node", mrmsg.Data{"nodeId": nodeID})
	}

	if nodeID == afterNodeID {
		return mrcore.ErrUseCaseIncorrectInputData.New("node", mrmsg.Data{"nodeId=afterNodeId": nodeID})
	}

	currentNode, err := co.storage.FetchNode(ctx, nodeID)
	if err != nil {
		return co.errorWrapper.WrapErrorNotFoundOrFailed(err, entity.ModelNameNode)
	}

	if mrtype.KeyInt32(currentNode.PrevID) == afterNodeID {
		return nil
	}

	afterNode, err := co.storage.FetchNode(ctx, afterNodeID)
	if err != nil {
		return co.wrapErrorAfterNodeNotFound(err, afterNodeID)
	}

	afterNextNode := entity.Node{
		ID: mrtype.KeyInt32(afterNode.NextID),
	}

	if afterNextNode.ID > 0 {
		if afterNextNode, err = co.storage.FetchNode(ctx, afterNextNode.ID); err != nil {
			return co.wrapErrorMustLoad(err)
		}
	}

	if err = co.storage.UpdateNodeNextID(ctx, afterNode.ID, mrentity.ZeronullInt32(currentNode.ID)); err != nil {
		return co.wrapErrorMustStore(err)
	}

	if afterNextNode.ID > 0 {
		if err = co.storage.UpdateNodePrevID(ctx, afterNextNode.ID, mrentity.ZeronullInt32(currentNode.ID)); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	if currentNode.PrevID > 0 {
		if err = co.storage.UpdateNodeNextID(ctx, mrtype.KeyInt32(currentNode.PrevID), currentNode.NextID); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	if currentNode.NextID > 0 {
		if err = co.storage.UpdateNodePrevID(ctx, mrtype.KeyInt32(currentNode.NextID), currentNode.PrevID); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	currentNode.PrevID = mrentity.ZeronullInt32(afterNode.ID)
	currentNode.NextID = mrentity.ZeronullInt32(afterNextNode.ID)
	currentNode.OrderIndex = (afterNode.OrderIndex + afterNextNode.OrderIndex) / 2

	if currentNode.OrderIndex <= afterNode.OrderIndex {
		if afterNextNode.ID > 0 {
			if err := co.storage.RecalcOrderIndex(ctx, int64(afterNode.OrderIndex), orderIndexStep*2); err != nil {
				return co.wrapErrorMustStore(err)
			}
		}

		currentNode.OrderIndex = afterNode.OrderIndex + mrentity.ZeronullInt64(orderIndexStep)
	}

	if err := co.storage.UpdateNode(ctx, currentNode); err != nil {
		return co.wrapErrorMustStore(err)
	}

	co.emitEvent(ctx, "MoveAfterId", mrmsg.Data{"id": nodeID, "afterId": afterNodeID})

	return nil
}

// Unlink - comment method.
func (co *Component) Unlink(ctx context.Context, nodeID mrtype.KeyInt32) error {
	if nodeID < 1 {
		return co.MoveToFirst(ctx, nodeID)
	}

	currentNode, err := co.storage.FetchNode(ctx, nodeID)
	if err != nil {
		return co.errorWrapper.WrapErrorNotFoundOrFailed(err, entity.ModelNameNode)
	}

	if currentNode.PrevID == 0 &&
		currentNode.NextID == 0 &&
		currentNode.OrderIndex == 0 {
		return nil
	}

	if currentNode.PrevID > 0 {
		if err = co.storage.UpdateNodeNextID(ctx, mrtype.KeyInt32(currentNode.PrevID), currentNode.NextID); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	if currentNode.NextID > 0 {
		if err = co.storage.UpdateNodePrevID(ctx, mrtype.KeyInt32(currentNode.NextID), currentNode.PrevID); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	currentNode.PrevID = 0
	currentNode.NextID = 0
	currentNode.OrderIndex = 0

	if err = co.storage.UpdateNode(ctx, currentNode); err != nil {
		return co.wrapErrorMustStore(err)
	}

	co.emitEvent(ctx, "Unlink", mrmsg.Data{"id": nodeID})

	return nil
}

func (co *Component) wrapErrorAfterNodeNotFound(err error, afterNodeID mrtype.KeyInt32) error {
	if errors.Is(err, mrcore.ErrStorageNoRowFound) {
		return mrsort.ErrAfterNodeNotFound.Wrap(err, afterNodeID)
	}

	return co.errorWrapper.WrapErrorFailed(err, entity.ModelNameNode)
}

func (co *Component) wrapErrorMustLoad(err error) error {
	if errors.Is(err, mrcore.ErrStorageNoRowFound) {
		return mrcore.ErrInternal.Wrap(err)
	}

	return co.errorWrapper.WrapErrorFailed(err, entity.ModelNameNode)
}

func (co *Component) wrapErrorMustStore(err error) error {
	if co.errorWrapper.IsNotFoundError(err) {
		return mrcore.ErrInternal.Wrap(err)
	}

	return co.errorWrapper.WrapErrorFailed(err, entity.ModelNameNode)
}

func (co *Component) emitEvent(ctx context.Context, eventName string, object mrmsg.Data) {
	co.eventEmitter.EmitWithSource(
		ctx,
		eventName,
		entity.ModelNameNode,
		object,
	)
}
