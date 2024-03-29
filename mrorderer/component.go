package mrorderer

import (
	"context"

	"github.com/mondegor/go-storage/mrentity"
	"github.com/mondegor/go-sysmess/mrmsg"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrsender"
	"github.com/mondegor/go-webcore/mrtype"
)

const (
	orderIndexStep int64 = 1024 * 1024
)

type (
	Component struct {
		storage      Storage
		eventEmitter mrsender.EventEmitter
	}
)

func NewComponent(
	storage Storage,
	eventEmitter mrsender.EventEmitter,
) *Component {
	return &Component{
		storage:      storage,
		eventEmitter: eventEmitter,
	}
}

func (co *Component) WithMetaData(meta EntityMeta) API {
	return &Component{
		storage:      co.storage.WithMetaData(meta),
		eventEmitter: co.eventEmitter,
	}
}

func (co *Component) InsertToFirst(ctx context.Context, nodeID mrtype.KeyInt32) error {
	if nodeID < 1 {
		return mrcore.FactoryErrUseCaseIncorrectInputData.New("node", mrmsg.Data{"nodeId": nodeID})
	}

	firstNode := EntityNode{}

	if err := co.storage.LoadFirstNode(ctx, &firstNode); err != nil {
		return co.wrapErrorMustLoad(err)
	}

	if nodeID == firstNode.ID {
		return mrcore.FactoryErrUseCaseIncorrectInputData.New("node", mrmsg.Data{"nodeId=firstNode.Id": nodeID})
	}

	if err := co.storage.UpdateNodePrevID(ctx, firstNode.ID, mrentity.ZeronullInt32(nodeID)); err != nil {
		return co.wrapErrorMustStore(err)
	}

	currentNode := EntityNode{
		ID:         nodeID,
		PrevID:     0,
		NextID:     mrentity.ZeronullInt32(firstNode.ID),
		OrderIndex: firstNode.OrderIndex / 2,
	}

	if currentNode.OrderIndex < 1 {
		if err := co.storage.RecalcOrderIndex(ctx, 0, 2*orderIndexStep); err != nil {
			return co.wrapErrorMustStore(err)
		}

		currentNode.OrderIndex = mrentity.ZeronullInt64(orderIndexStep)
	}

	if err := co.storage.UpdateNode(ctx, &currentNode); err != nil {
		return co.wrapErrorNotFound(err)
	}

	co.emitEvent(ctx, "InsertToFirst", mrmsg.Data{"id": nodeID})

	return nil
}

func (co *Component) InsertToLast(ctx context.Context, nodeID mrtype.KeyInt32) error {
	if nodeID < 1 {
		return mrcore.FactoryErrUseCaseIncorrectInputData.New("node", mrmsg.Data{"nodeId": nodeID})
	}

	lastNode := EntityNode{}

	if err := co.storage.LoadLastNode(ctx, &lastNode); err != nil {
		return co.wrapErrorMustLoad(err)
	}

	if nodeID == lastNode.ID {
		return mrcore.FactoryErrUseCaseIncorrectInputData.New("node", mrmsg.Data{"nodeId=lastNode.Id": nodeID})
	}

	if err := co.storage.UpdateNodeNextID(ctx, lastNode.ID, mrentity.ZeronullInt32(nodeID)); err != nil {
		return co.wrapErrorMustStore(err)
	}

	currentNode := EntityNode{
		ID:         nodeID,
		PrevID:     mrentity.ZeronullInt32(lastNode.ID),
		NextID:     0,
		OrderIndex: lastNode.OrderIndex + mrentity.ZeronullInt64(orderIndexStep),
	}

	if err := co.storage.UpdateNode(ctx, &currentNode); err != nil {
		return co.wrapErrorNotFound(err)
	}

	co.emitEvent(ctx, "InsertToLast", mrmsg.Data{"id": nodeID})

	return nil
}

func (co *Component) MoveToFirst(ctx context.Context, nodeID mrtype.KeyInt32) error {
	if nodeID < 1 {
		return mrcore.FactoryErrUseCaseIncorrectInputData.New("node", mrmsg.Data{"nodeId": nodeID})
	}

	currentNode := EntityNode{ID: nodeID}

	firstNode := EntityNode{}

	if err := co.storage.LoadFirstNode(ctx, &firstNode); err != nil {
		return co.wrapErrorMustLoad(err)
	}

	if firstNode.ID == currentNode.ID {
		if firstNode.OrderIndex == 0 {
			currentNode.OrderIndex = mrentity.ZeronullInt64(orderIndexStep)

			if err := co.storage.UpdateNode(ctx, &currentNode); err != nil {
				return co.wrapErrorMustStore(err)
			}
		}

		return nil
	}

	if err := co.storage.LoadNode(ctx, &currentNode); err != nil {
		return co.wrapErrorNotFound(err)
	}

	if mrtype.KeyInt32(currentNode.NextID) == firstNode.ID {
		return mrcore.FactoryErrInternal.WithAttr(
			"node",
			mrmsg.Data{
				"currentNode.Id":                  currentNode.ID,
				"currentNode.NextId=firstNode.Id": currentNode.NextID,
			},
		).New()
	}

	if err := co.storage.UpdateNodePrevID(ctx, firstNode.ID, mrentity.ZeronullInt32(currentNode.ID)); err != nil {
		return co.wrapErrorMustStore(err)
	}

	if currentNode.PrevID > 0 {
		if err := co.storage.UpdateNodeNextID(ctx, mrtype.KeyInt32(currentNode.PrevID), currentNode.NextID); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	if currentNode.NextID > 0 {
		if err := co.storage.UpdateNodePrevID(ctx, mrtype.KeyInt32(currentNode.NextID), currentNode.PrevID); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	currentNode.PrevID = 0
	currentNode.NextID = mrentity.ZeronullInt32(firstNode.ID)
	currentNode.OrderIndex = firstNode.OrderIndex / 2

	if currentNode.OrderIndex < 1 {
		if err := co.storage.RecalcOrderIndex(ctx, 0, 2*orderIndexStep); err != nil {
			return co.wrapErrorMustStore(err)
		}

		currentNode.OrderIndex = mrentity.ZeronullInt64(orderIndexStep)
	}

	if err := co.storage.UpdateNode(ctx, &currentNode); err != nil {
		return co.wrapErrorMustStore(err)
	}

	co.emitEvent(ctx, "MoveToFirst", mrmsg.Data{"id": nodeID})

	return nil
}

func (co *Component) MoveToLast(ctx context.Context, nodeID mrtype.KeyInt32) error {
	if nodeID < 1 {
		return mrcore.FactoryErrUseCaseIncorrectInputData.New("node", mrmsg.Data{"nodeId": nodeID})
	}

	currentNode := EntityNode{ID: nodeID}

	lastNode := EntityNode{}

	if err := co.storage.LoadLastNode(ctx, &lastNode); err != nil {
		return co.wrapErrorMustLoad(err)
	}

	if lastNode.ID == currentNode.ID {
		if lastNode.OrderIndex == 0 {
			currentNode.OrderIndex = mrentity.ZeronullInt64(orderIndexStep)

			if err := co.storage.UpdateNode(ctx, &currentNode); err != nil {
				return co.wrapErrorMustStore(err)
			}
		}

		return nil
	}

	if err := co.storage.LoadNode(ctx, &currentNode); err != nil {
		return co.wrapErrorNotFound(err)
	}

	if lastNode.ID > 0 {
		if mrtype.KeyInt32(currentNode.PrevID) == lastNode.ID {
			return mrcore.FactoryErrInternal.WithAttr(
				"node",
				mrmsg.Data{
					"currentNode.Id":                 currentNode.ID,
					"currentNode.PrevId=lastNode.Id": currentNode.PrevID,
				},
			).New()
		}

		if err := co.storage.UpdateNodeNextID(ctx, lastNode.ID, mrentity.ZeronullInt32(currentNode.ID)); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	if currentNode.PrevID > 0 {
		if err := co.storage.UpdateNodeNextID(ctx, mrtype.KeyInt32(currentNode.PrevID), currentNode.NextID); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	if currentNode.NextID > 0 {
		if err := co.storage.UpdateNodePrevID(ctx, mrtype.KeyInt32(currentNode.NextID), currentNode.PrevID); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	currentNode.PrevID = mrentity.ZeronullInt32(lastNode.ID)
	currentNode.NextID = 0
	currentNode.OrderIndex = lastNode.OrderIndex + mrentity.ZeronullInt64(orderIndexStep)

	if err := co.storage.UpdateNode(ctx, &currentNode); err != nil {
		return co.wrapErrorMustStore(err)
	}

	co.emitEvent(ctx, "MoveToLast", mrmsg.Data{"id": nodeID})

	return nil
}

func (co *Component) MoveAfterID(ctx context.Context, nodeID mrtype.KeyInt32, afterNodeID mrtype.KeyInt32) error {
	if afterNodeID < 1 {
		return co.MoveToFirst(ctx, nodeID)
	}

	if nodeID < 1 {
		return mrcore.FactoryErrUseCaseIncorrectInputData.New("node", mrmsg.Data{"nodeId": nodeID})
	}

	if nodeID == afterNodeID {
		return mrcore.FactoryErrUseCaseIncorrectInputData.New("node", mrmsg.Data{"nodeId=afterNodeId": nodeID})
	}

	currentNode := EntityNode{ID: nodeID}

	if err := co.storage.LoadNode(ctx, &currentNode); err != nil {
		return co.wrapErrorNotFound(err)
	}

	if mrtype.KeyInt32(currentNode.PrevID) == afterNodeID {
		return nil
	}

	afterNode := EntityNode{ID: afterNodeID}

	if err := co.storage.LoadNode(ctx, &afterNode); err != nil {
		return co.wrapErrorAfterNodeNotFound(err, afterNode.ID)
	}

	afterNextNode := EntityNode{ID: mrtype.KeyInt32(afterNode.NextID)}

	if afterNextNode.ID > 0 {
		if err := co.storage.LoadNode(ctx, &afterNextNode); err != nil {
			return co.wrapErrorMustLoad(err)
		}
	}

	if err := co.storage.UpdateNodeNextID(ctx, afterNode.ID, mrentity.ZeronullInt32(currentNode.ID)); err != nil {
		return co.wrapErrorMustStore(err)
	}

	if afterNextNode.ID > 0 {
		if err := co.storage.UpdateNodePrevID(ctx, afterNextNode.ID, mrentity.ZeronullInt32(currentNode.ID)); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	if currentNode.PrevID > 0 {
		if err := co.storage.UpdateNodeNextID(ctx, mrtype.KeyInt32(currentNode.PrevID), currentNode.NextID); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	if currentNode.NextID > 0 {
		if err := co.storage.UpdateNodePrevID(ctx, mrtype.KeyInt32(currentNode.NextID), currentNode.PrevID); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	currentNode.PrevID = mrentity.ZeronullInt32(afterNode.ID)
	currentNode.NextID = mrentity.ZeronullInt32(afterNextNode.ID)
	currentNode.OrderIndex = (afterNode.OrderIndex + afterNextNode.OrderIndex) / 2

	if currentNode.OrderIndex <= afterNode.OrderIndex {
		if afterNextNode.ID > 0 {
			if err := co.storage.RecalcOrderIndex(ctx, int64(afterNode.OrderIndex), 2*orderIndexStep); err != nil {
				return co.wrapErrorMustStore(err)
			}
		}

		currentNode.OrderIndex = afterNode.OrderIndex + mrentity.ZeronullInt64(orderIndexStep)
	}

	if err := co.storage.UpdateNode(ctx, &currentNode); err != nil {
		return co.wrapErrorMustStore(err)
	}

	co.emitEvent(ctx, "MoveAfterId", mrmsg.Data{"id": nodeID, "afterId": afterNodeID})

	return nil
}

func (co *Component) Unlink(ctx context.Context, nodeID mrtype.KeyInt32) error {
	if nodeID < 1 {
		return co.MoveToFirst(ctx, nodeID)
	}

	currentNode := EntityNode{ID: nodeID}

	if err := co.storage.LoadNode(ctx, &currentNode); err != nil {
		return co.wrapErrorNotFound(err)
	}

	if currentNode.PrevID == 0 &&
		currentNode.NextID == 0 &&
		currentNode.OrderIndex == 0 {
		return nil
	}

	if currentNode.PrevID > 0 {
		if err := co.storage.UpdateNodeNextID(ctx, mrtype.KeyInt32(currentNode.PrevID), currentNode.NextID); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	if currentNode.NextID > 0 {
		if err := co.storage.UpdateNodePrevID(ctx, mrtype.KeyInt32(currentNode.NextID), currentNode.PrevID); err != nil {
			return co.wrapErrorMustStore(err)
		}
	}

	currentNode.PrevID = 0
	currentNode.NextID = 0
	currentNode.OrderIndex = 0

	if err := co.storage.UpdateNode(ctx, &currentNode); err != nil {
		return co.wrapErrorMustStore(err)
	}

	co.emitEvent(ctx, "Unlink", mrmsg.Data{"id": nodeID})

	return nil
}

func (co *Component) wrapErrorNotFound(err error) error {
	if mrcore.FactoryErrStorageNoRowFound.Is(err) ||
		mrcore.FactoryErrStorageRowsNotAffected.Is(err) {
		return mrcore.FactoryErrUseCaseEntityNotFound.Wrap(err)
	}

	return co.wrapErrorFailed(err)
}

func (co *Component) wrapErrorAfterNodeNotFound(err error, afterNodeID mrtype.KeyInt32) error {
	if mrcore.FactoryErrStorageNoRowFound.Is(err) {
		return FactoryErrAfterNodeNotFound.Wrap(err, afterNodeID)
	}

	return co.wrapErrorFailed(err)
}

func (co *Component) wrapErrorMustLoad(err error) error {
	if mrcore.FactoryErrStorageNoRowFound.Is(err) {
		return mrcore.FactoryErrInternal.WithCaller(1).Wrap(err)
	}

	return co.wrapErrorFailed(err)
}

func (co *Component) wrapErrorMustStore(err error) error {
	if mrcore.FactoryErrStorageNoRowFound.Is(err) ||
		mrcore.FactoryErrStorageRowsNotAffected.Is(err) {
		return mrcore.FactoryErrInternal.WithCaller(1).Wrap(err)
	}

	return co.wrapErrorFailed(err)
}

func (co *Component) wrapErrorFailed(err error) error {
	if mrcore.FactoryErrStorageQueryFailed.Is(err) {
		return mrcore.FactoryErrUseCaseOperationFailed.WithCaller(2).Wrap(err)
	}

	return mrcore.FactoryErrUseCaseTemporarilyUnavailable.WithCaller(2).Wrap(err)
}

func (co *Component) emitEvent(ctx context.Context, eventName string, object mrmsg.Data) {
	co.eventEmitter.EmitWithSource(
		ctx,
		eventName,
		ModelNameEntityOrderer,
		object,
	)
}
