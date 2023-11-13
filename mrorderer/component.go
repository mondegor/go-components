package mrorderer

import (
	"context"

	"github.com/mondegor/go-storage/mrentity"
	"github.com/mondegor/go-sysmess/mrerr"
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
		return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"nodeId": nodeID})
	}

	firstNode := EntityNode{}
	err := co.storage.LoadFirstNode(ctx, &firstNode)

	if err != nil {
		return err
	}

	if firstNode.ID == nodeID {
		return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"nodeId": nodeID})
	}

	err = co.storage.UpdateNodePrevID(ctx, firstNode.ID, mrentity.ZeronullInt32(nodeID))

	if err != nil {
		return err
	}

	currentNode := EntityNode{
		ID:         nodeID,
		PrevID:     0,
		NextID:     mrentity.ZeronullInt32(firstNode.ID),
		OrderField: firstNode.OrderField / 2,
	}

	if currentNode.OrderField < 1 {
		err = co.storage.RecalcOrderField(ctx, 0, 2*orderFieldStep)

		if err != nil {
			return err
		}

		currentNode.OrderField = mrentity.ZeronullInt64(orderFieldStep)
	}

	err = co.storage.UpdateNode(ctx, &currentNode)

	if err != nil {
		return err
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
		return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"nodeId": nodeID})
	}

	lastNode := EntityNode{}
	err := co.storage.LoadLastNode(ctx, &lastNode)

	if err != nil {
		return err
	}

	if lastNode.ID == nodeID {
		return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"nodeId": nodeID})
	}

	err = co.storage.UpdateNodeNextID(ctx, lastNode.ID, mrentity.ZeronullInt32(nodeID))

	if err != nil {
		return err
	}

	currentNode := EntityNode{
		ID:         nodeID,
		PrevID:     mrentity.ZeronullInt32(lastNode.ID),
		NextID:     0,
		OrderField: lastNode.OrderField + mrentity.ZeronullInt64(orderFieldStep),
	}

	err = co.storage.UpdateNode(ctx, &currentNode)

	if err != nil {
		return err
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
		return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"nodeId": nodeID})
	}

	currentNode := EntityNode{ID: nodeID}

	firstNode := EntityNode{}
	err := co.storage.LoadFirstNode(ctx, &firstNode)

	if err != nil {
		return err
	}

	if firstNode.ID == currentNode.ID {
		if firstNode.OrderField == 0 {
			currentNode.OrderField = mrentity.ZeronullInt64(orderFieldStep)
			err = co.storage.UpdateNode(ctx, &currentNode)

			if err != nil {
				return err
			}
		}

		return nil
	}

	err = co.storage.LoadNode(ctx, &currentNode)

	if err != nil {
		return err
	}

	if mrtype.KeyInt32(currentNode.NextID) == firstNode.ID {
		return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"currentNode.Id": currentNode.ID, "currentNode.NextId": currentNode.NextID})
	}

	err = co.storage.UpdateNodePrevID(ctx, firstNode.ID, mrentity.ZeronullInt32(currentNode.ID))

	if err != nil {
		return err
	}

	if currentNode.PrevID > 0 {
		err = co.storage.UpdateNodeNextID(ctx, mrtype.KeyInt32(currentNode.PrevID), currentNode.NextID)

		if err != nil {
			return err
		}
	}

	if currentNode.NextID > 0 {
		err = co.storage.UpdateNodePrevID(ctx, mrtype.KeyInt32(currentNode.NextID), currentNode.PrevID)

		if err != nil {
			return err
		}
	}

	currentNode.PrevID = 0
	currentNode.NextID = mrentity.ZeronullInt32(firstNode.ID)
	currentNode.OrderField = firstNode.OrderField / 2

	if currentNode.OrderField < 1 {
		err = co.storage.RecalcOrderField(ctx, 0, 2*orderFieldStep)

		if err != nil {
			return err
		}

		currentNode.OrderField = mrentity.ZeronullInt64(orderFieldStep)
	}

	err = co.storage.UpdateNode(ctx, &currentNode)

	if err != nil {
		return err
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
		return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"nodeId": nodeID})
	}

	currentNode := EntityNode{ID: nodeID}

	lastNode := EntityNode{}
	err := co.storage.LoadLastNode(ctx, &lastNode)

	if err != nil {
		return err
	}

	if lastNode.ID == currentNode.ID {
		if lastNode.OrderField == 0 {
			currentNode.OrderField = mrentity.ZeronullInt64(orderFieldStep)
			err = co.storage.UpdateNode(ctx, &currentNode)

			if err != nil {
				return err
			}
		}

		return nil
	}

	err = co.storage.LoadNode(ctx, &currentNode)

	if err != nil {
		return err
	}

	if lastNode.ID > 0 {
		if mrtype.KeyInt32(currentNode.PrevID) == lastNode.ID {
			return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"currentNode.Id": currentNode.ID, "currentNode.PrevId": currentNode.PrevID})
		}

		err = co.storage.UpdateNodeNextID(ctx, lastNode.ID, mrentity.ZeronullInt32(currentNode.ID))

		if err != nil {
			return err
		}
	}

	if currentNode.PrevID > 0 {
		err = co.storage.UpdateNodeNextID(ctx, mrtype.KeyInt32(currentNode.PrevID), currentNode.NextID)

		if err != nil {
			return err
		}
	}

	if currentNode.NextID > 0 {
		err = co.storage.UpdateNodePrevID(ctx, mrtype.KeyInt32(currentNode.NextID), currentNode.PrevID)

		if err != nil {
			return err
		}
	}

	currentNode.PrevID = mrentity.ZeronullInt32(lastNode.ID)
	currentNode.NextID = 0
	currentNode.OrderField = lastNode.OrderField + mrentity.ZeronullInt64(orderFieldStep)

	err = co.storage.UpdateNode(ctx, &currentNode)

	if err != nil {
		return err
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
		return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"nodeId": nodeID})
	}

	if nodeID == afterNodeID {
		return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"nodeId": nodeID, "afterNodeId": afterNodeID})
	}

	currentNode := EntityNode{ID: nodeID}
	err := co.storage.LoadNode(ctx, &currentNode)

	if err != nil {
		return err
	}

	if mrtype.KeyInt32(currentNode.PrevID) == afterNodeID {
		return nil
	}

	afterNode := EntityNode{ID: afterNodeID}
	err = co.storage.LoadNode(ctx, &afterNode)

	if err != nil {
		return err
	}

	afterNextNode := EntityNode{ID: mrtype.KeyInt32(afterNode.NextID)}

	if afterNextNode.ID > 0 {
		err = co.storage.LoadNode(ctx, &afterNextNode)

		if err != nil {
			return err
		}
	}

	err = co.storage.UpdateNodeNextID(ctx, afterNode.ID, mrentity.ZeronullInt32(currentNode.ID))

	if err != nil {
		return err
	}

	if afterNextNode.ID > 0 {
		err = co.storage.UpdateNodePrevID(ctx, afterNextNode.ID, mrentity.ZeronullInt32(currentNode.ID))

		if err != nil {
			return err
		}
	}

	if currentNode.PrevID > 0 {
		err = co.storage.UpdateNodeNextID(ctx, mrtype.KeyInt32(currentNode.PrevID), currentNode.NextID)

		if err != nil {
			return err
		}
	}

	if currentNode.NextID > 0 {
		err = co.storage.UpdateNodePrevID(ctx, mrtype.KeyInt32(currentNode.NextID), currentNode.PrevID)

		if err != nil {
			return err
		}
	}

	currentNode.PrevID = mrentity.ZeronullInt32(afterNode.ID)
	currentNode.NextID = mrentity.ZeronullInt32(afterNextNode.ID)
	currentNode.OrderField = (afterNode.OrderField + afterNextNode.OrderField) / 2

	if currentNode.OrderField <= afterNode.OrderField {
		if afterNextNode.ID > 0 {
			err = co.storage.RecalcOrderField(ctx, int64(afterNode.OrderField), 2*orderFieldStep)

			if err != nil {
				return err
			}
		}

		currentNode.OrderField = afterNode.OrderField + mrentity.ZeronullInt64(orderFieldStep)
	}

	err = co.storage.UpdateNode(ctx, &currentNode)

	if err != nil {
		return err
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
	err := co.storage.LoadNode(ctx, &currentNode)

	if err != nil {
		return err
	}

	if currentNode.PrevID == 0 &&
		currentNode.NextID == 0 &&
		currentNode.OrderField == 0 {
		return nil
	}

	if currentNode.PrevID > 0 {
		err = co.storage.UpdateNodeNextID(ctx, mrtype.KeyInt32(currentNode.PrevID), currentNode.NextID)

		if err != nil {
			return err
		}
	}

	if currentNode.NextID > 0 {
		err = co.storage.UpdateNodePrevID(ctx, mrtype.KeyInt32(currentNode.NextID), currentNode.PrevID)

		if err != nil {
			return err
		}
	}

	currentNode.PrevID = 0
	currentNode.NextID = 0
	currentNode.OrderField = 0

	err = co.storage.UpdateNode(ctx, &currentNode)

	if err != nil {
		return err
	}

	co.eventBox.Emit(
		"%s::Unlink: id=%d",
		ModelNameEntityOrderer,
		nodeID,
	)

	return nil
}
