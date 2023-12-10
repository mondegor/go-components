package mrorderer

import (
	"context"

	"github.com/mondegor/go-storage/mrentity"
	"github.com/mondegor/go-sysmess/mrmsg"
	"github.com/mondegor/go-webcore/mrcore"
	"github.com/mondegor/go-webcore/mrtype"
)

const (
	orderFieldStep int64 = 1024 * 1024
)

type (
	component struct {
		storage  Storage
		eventBox mrcore.EventBox
	}
)

func NewComponent(
	storage Storage,
	eventBox mrcore.EventBox,
) *component {
	return &component{
		storage:  storage,
		eventBox: eventBox,
	}
}

func (co *component) WithMetaData(meta EntityMeta) Component {
	return &component{
		storage:  co.storage.WithMetaData(meta),
		eventBox: co.eventBox,
	}
}

func (co *component) InsertToFirst(ctx context.Context, nodeID mrtype.KeyInt32) error {
	if nodeID < 1 {
		return mrcore.FactoryErrServiceIncorrectInputData.New("node", mrmsg.Data{"nodeId": nodeID})
	}

	firstNode := EntityNode{}

	if err := co.storage.LoadFirstNode(ctx, &firstNode); err != nil {
		return co.wrapErrorMustLoad(err)
	}

	if nodeID == firstNode.ID {
		return mrcore.FactoryErrServiceIncorrectInputData.New("node", mrmsg.Data{"nodeId=firstNode.Id": nodeID})
	}

	if err := co.storage.UpdateNodePrevID(ctx, firstNode.ID, mrentity.ZeronullInt32(nodeID)); err != nil {
		return co.wrapErrorMustStore(err)
	}

	currentNode := EntityNode{
		ID:         nodeID,
		PrevID:     0,
		NextID:     mrentity.ZeronullInt32(firstNode.ID),
		OrderField: firstNode.OrderField / 2,
	}

	if currentNode.OrderField < 1 {
		if err := co.storage.RecalcOrderField(ctx, 0, 2*orderFieldStep); err != nil {
			return co.wrapErrorMustStore(err)
		}

		currentNode.OrderField = mrentity.ZeronullInt64(orderFieldStep)
	}

	if err := co.storage.UpdateNode(ctx, &currentNode); err != nil {
		return co.wrapErrorNotFound(err)
	}

	co.eventBox.Emit(
		"%s::InsertToFirst: id=%d",
		ModelNameEntityOrderer,
		nodeID,
	)

	return nil
}

func (co *component) InsertToLast(ctx context.Context, nodeID mrtype.KeyInt32) error {
	if nodeID < 1 {
		return mrcore.FactoryErrServiceIncorrectInputData.New("node", mrmsg.Data{"nodeId": nodeID})
	}

	lastNode := EntityNode{}

	if err := co.storage.LoadLastNode(ctx, &lastNode); err != nil {
		return co.wrapErrorMustLoad(err)
	}

	if nodeID == lastNode.ID {
		return mrcore.FactoryErrServiceIncorrectInputData.New("node", mrmsg.Data{"nodeId=lastNode.Id": nodeID})
	}

	if err := co.storage.UpdateNodeNextID(ctx, lastNode.ID, mrentity.ZeronullInt32(nodeID)); err != nil {
		return co.wrapErrorMustStore(err)
	}

	currentNode := EntityNode{
		ID:         nodeID,
		PrevID:     mrentity.ZeronullInt32(lastNode.ID),
		NextID:     0,
		OrderField: lastNode.OrderField + mrentity.ZeronullInt64(orderFieldStep),
	}

	if err := co.storage.UpdateNode(ctx, &currentNode); err != nil {
		return co.wrapErrorNotFound(err)
	}

	co.eventBox.Emit(
		"%s::InsertToLast: id=%d",
		ModelNameEntityOrderer,
		nodeID,
	)

	return nil
}

func (co *component) MoveToFirst(ctx context.Context, nodeID mrtype.KeyInt32) error {
	if nodeID < 1 {
		return mrcore.FactoryErrServiceIncorrectInputData.New("node", mrmsg.Data{"nodeId": nodeID})
	}

	currentNode := EntityNode{ID: nodeID}

	firstNode := EntityNode{}

	if err := co.storage.LoadFirstNode(ctx, &firstNode); err != nil {
		return co.wrapErrorMustLoad(err)
	}

	if firstNode.ID == currentNode.ID {
		if firstNode.OrderField == 0 {
			currentNode.OrderField = mrentity.ZeronullInt64(orderFieldStep)

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
		return mrcore.FactoryErrInternalWithData.New(
			"node",
			mrmsg.Data{
				"currentNode.Id":                  currentNode.ID,
				"currentNode.NextId=firstNode.Id": currentNode.NextID,
			},
		)
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
	currentNode.OrderField = firstNode.OrderField / 2

	if currentNode.OrderField < 1 {
		if err := co.storage.RecalcOrderField(ctx, 0, 2*orderFieldStep); err != nil {
			return co.wrapErrorMustStore(err)
		}

		currentNode.OrderField = mrentity.ZeronullInt64(orderFieldStep)
	}

	if err := co.storage.UpdateNode(ctx, &currentNode); err != nil {
		return co.wrapErrorMustStore(err)
	}

	co.eventBox.Emit(
		"%s::MoveToFirst: id=%d",
		ModelNameEntityOrderer,
		nodeID,
	)

	return nil
}

func (co *component) MoveToLast(ctx context.Context, nodeID mrtype.KeyInt32) error {
	if nodeID < 1 {
		return mrcore.FactoryErrServiceIncorrectInputData.New("node", mrmsg.Data{"nodeId": nodeID})
	}

	currentNode := EntityNode{ID: nodeID}

	lastNode := EntityNode{}

	if err := co.storage.LoadLastNode(ctx, &lastNode); err != nil {
		return co.wrapErrorMustLoad(err)
	}

	if lastNode.ID == currentNode.ID {
		if lastNode.OrderField == 0 {
			currentNode.OrderField = mrentity.ZeronullInt64(orderFieldStep)

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
			return mrcore.FactoryErrInternalWithData.New(
				"node",
				mrmsg.Data{
					"currentNode.Id":                 currentNode.ID,
					"currentNode.PrevId=lastNode.Id": currentNode.PrevID,
				},
			)
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
	currentNode.OrderField = lastNode.OrderField + mrentity.ZeronullInt64(orderFieldStep)

	if err := co.storage.UpdateNode(ctx, &currentNode); err != nil {
		return co.wrapErrorMustStore(err)
	}

	co.eventBox.Emit(
		"%s::MoveToLast: id=%d",
		ModelNameEntityOrderer,
		nodeID,
	)

	return nil
}

func (co *component) MoveAfterID(ctx context.Context, nodeID mrtype.KeyInt32, afterNodeID mrtype.KeyInt32) error {
	if afterNodeID < 1 {
		return co.MoveToFirst(ctx, nodeID)
	}

	if nodeID < 1 {
		return mrcore.FactoryErrServiceIncorrectInputData.New("node", mrmsg.Data{"nodeId": nodeID})
	}

	if nodeID == afterNodeID {
		return mrcore.FactoryErrServiceIncorrectInputData.New("node", mrmsg.Data{"nodeId=afterNodeId": nodeID})
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
	currentNode.OrderField = (afterNode.OrderField + afterNextNode.OrderField) / 2

	if currentNode.OrderField <= afterNode.OrderField {
		if afterNextNode.ID > 0 {
			if err := co.storage.RecalcOrderField(ctx, int64(afterNode.OrderField), 2*orderFieldStep); err != nil {
				return co.wrapErrorMustStore(err)
			}
		}

		currentNode.OrderField = afterNode.OrderField + mrentity.ZeronullInt64(orderFieldStep)
	}

	if err := co.storage.UpdateNode(ctx, &currentNode); err != nil {
		return co.wrapErrorMustStore(err)
	}

	co.eventBox.Emit(
		"%s::MoveAfterId: id=%d, afterId=%d",
		ModelNameEntityOrderer,
		nodeID,
		afterNodeID,
	)

	return nil
}

func (co *component) Unlink(ctx context.Context, nodeID mrtype.KeyInt32) error {
	if nodeID < 1 {
		return co.MoveToFirst(ctx, nodeID)
	}

	currentNode := EntityNode{ID: nodeID}

	if err := co.storage.LoadNode(ctx, &currentNode); err != nil {
		return co.wrapErrorNotFound(err)
	}

	if currentNode.PrevID == 0 &&
		currentNode.NextID == 0 &&
		currentNode.OrderField == 0 {
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
	currentNode.OrderField = 0

	if err := co.storage.UpdateNode(ctx, &currentNode); err != nil {
		return co.wrapErrorMustStore(err)
	}

	co.eventBox.Emit(
		"%s::Unlink: id=%d",
		ModelNameEntityOrderer,
		nodeID,
	)

	return nil
}

func (co *component) wrapErrorNotFound(err error) error {
	if mrcore.FactoryErrStorageNoRowFound.Is(err) ||
		mrcore.FactoryErrStorageRowsNotAffected.Is(err) {
		return mrcore.FactoryErrServiceEntityNotFound.Wrap(err)
	}

	return co.wrapErrorFailed(err)
}

func (co *component) wrapErrorAfterNodeNotFound(err error, afterNodeID mrtype.KeyInt32) error {
	if mrcore.FactoryErrStorageNoRowFound.Is(err) {
		return FactoryErrAfterNodeNotFound.Wrap(err, afterNodeID)
	}

	return co.wrapErrorFailed(err)
}

func (co *component) wrapErrorMustLoad(err error) error {
	if mrcore.FactoryErrStorageNoRowFound.Is(err) {
		return mrcore.FactoryErrInternal.Caller(1).Wrap(err)
	}

	return co.wrapErrorFailed(err)
}

func (co *component) wrapErrorMustStore(err error) error {
	if mrcore.FactoryErrStorageNoRowFound.Is(err) ||
		mrcore.FactoryErrStorageRowsNotAffected.Is(err) {
		return mrcore.FactoryErrInternal.Caller(1).Wrap(err)
	}

	return co.wrapErrorFailed(err)
}

func (co *component) wrapErrorFailed(err error) error {
	if mrcore.FactoryErrStorageQueryFailed.Is(err) {
		return mrcore.FactoryErrServiceOperationFailed.Wrap(err)
	}

	return mrcore.FactoryErrServiceTemporarilyUnavailable.Caller(2).Wrap(err)
}
