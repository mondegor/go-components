package mrcom_orderer

import (
    "context"

    "github.com/mondegor/go-storage/mrentity"
    "github.com/mondegor/go-sysmess/mrerr"
    "github.com/mondegor/go-webcore/mrcore"
)

const (
    orderFieldStep mrentity.Int64 = 1024 * 1024
)

type (
    component struct {
        storage Storage
        eventBox mrcore.EventBox
    }
)

func NewComponent(
    storage Storage,
    eventBox mrcore.EventBox,
) *component {
    return &component{
        storage: storage,
        eventBox: eventBox,
    }
}

func (co *component) WithMetaData(meta EntityMeta) Component {
    return &component{
        storage: co.storage.WithMetaData(meta),
    }
}

func (co *component) InsertToFirst(ctx context.Context, nodeId mrentity.KeyInt32) error {
    if nodeId < 1 {
        return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"nodeId": nodeId})
    }

    firstNode := EntityNode{}
    err := co.storage.LoadFirstNode(ctx, &firstNode)

    if err != nil {
        return err
    }

    if firstNode.Id == nodeId {
        return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"nodeId": nodeId})
    }

    err = co.storage.UpdateNodePrevId(ctx, firstNode.Id, mrentity.ZeronullInt32(nodeId))

    if err != nil {
        return err
    }

    currentNode := EntityNode{
        Id: nodeId,
        PrevId: 0,
        NextId: mrentity.ZeronullInt32(firstNode.Id),
        OrderField: firstNode.OrderField / 2,
    }

    if currentNode.OrderField < 1 {
        err = co.storage.RecalcOrderField(ctx, 0, 2 * orderFieldStep)

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
        nodeId,
    )

    return nil
}

func (co *component) InsertToLast(ctx context.Context, nodeId mrentity.KeyInt32) error {
    if nodeId < 1 {
        return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"nodeId": nodeId})
    }

    lastNode := EntityNode{}
    err := co.storage.LoadLastNode(ctx, &lastNode)

    if err != nil {
        return err
    }

    if lastNode.Id == nodeId {
        return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"nodeId": nodeId})
    }

    err = co.storage.UpdateNodeNextId(ctx, lastNode.Id, mrentity.ZeronullInt32(nodeId))

    if err != nil {
        return err
    }

    currentNode := EntityNode{
        Id: nodeId,
        PrevId: mrentity.ZeronullInt32(lastNode.Id),
        NextId: 0,
        OrderField: lastNode.OrderField + mrentity.ZeronullInt64(orderFieldStep),
    }

    err = co.storage.UpdateNode(ctx, &currentNode)

    if err != nil {
        return err
    }

    co.eventBox.Emit(
        "%s::InsertToLast: id=%d",
        ModelNameEntityOrderer,
        nodeId,
    )

    return nil
}

func (co *component) MoveToFirst(ctx context.Context, nodeId mrentity.KeyInt32) error {
    if nodeId < 1 {
        return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"nodeId": nodeId})
    }

    currentNode := EntityNode{Id: nodeId}

    firstNode := EntityNode{}
    err := co.storage.LoadFirstNode(ctx, &firstNode)

    if err != nil {
        return err
    }

    if firstNode.Id == currentNode.Id {
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

    if mrentity.KeyInt32(currentNode.NextId) == firstNode.Id {
        return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"currentNode.Id": currentNode.Id, "currentNode.NextId": currentNode.NextId})
    }

    err = co.storage.UpdateNodePrevId(ctx, firstNode.Id, mrentity.ZeronullInt32(currentNode.Id))

    if err != nil {
        return err
    }

    if currentNode.PrevId > 0 {
        err = co.storage.UpdateNodeNextId(ctx, mrentity.KeyInt32(currentNode.PrevId), currentNode.NextId)

        if err != nil {
            return err
        }
    }

    if currentNode.NextId > 0 {
        err = co.storage.UpdateNodePrevId(ctx, mrentity.KeyInt32(currentNode.NextId), currentNode.PrevId)

        if err != nil {
            return err
        }
    }

    currentNode.PrevId = 0
    currentNode.NextId = mrentity.ZeronullInt32(firstNode.Id)
    currentNode.OrderField = firstNode.OrderField / 2

    if currentNode.OrderField < 1 {
        err = co.storage.RecalcOrderField(ctx, 0, 2 * orderFieldStep)

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
        nodeId,
    )

    return nil
}

func (co *component) MoveToLast(ctx context.Context, nodeId mrentity.KeyInt32) error {
    if nodeId < 1 {
        return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"nodeId": nodeId})
    }

    currentNode := EntityNode{Id: nodeId}

    lastNode := EntityNode{}
    err := co.storage.LoadLastNode(ctx, &lastNode)

    if err != nil {
        return err
    }

    if lastNode.Id == currentNode.Id {
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

    if mrentity.KeyInt32(currentNode.PrevId) == lastNode.Id {
        return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"currentNode.Id": currentNode.Id, "currentNode.PrevId": currentNode.PrevId})
    }

    err = co.storage.UpdateNodeNextId(ctx, lastNode.Id, mrentity.ZeronullInt32(currentNode.Id))

    if err != nil {
        return err
    }

    if currentNode.PrevId > 0 {
        err = co.storage.UpdateNodeNextId(ctx, mrentity.KeyInt32(currentNode.PrevId), currentNode.NextId)

        if err != nil {
            return err
        }
    }

    if currentNode.NextId > 0 {
        err = co.storage.UpdateNodePrevId(ctx, mrentity.KeyInt32(currentNode.NextId), currentNode.PrevId)

        if err != nil {
            return err
        }
    }

    currentNode.PrevId = mrentity.ZeronullInt32(lastNode.Id)
    currentNode.NextId = 0
    currentNode.OrderField = lastNode.OrderField + mrentity.ZeronullInt64(orderFieldStep)

    err = co.storage.UpdateNode(ctx, &currentNode)

    if err != nil {
        return err
    }

    co.eventBox.Emit(
        "%s::MoveToLast: id=%d",
        ModelNameEntityOrderer,
        nodeId,
    )

    return nil
}

func (co *component) MoveAfterId(ctx context.Context, nodeId mrentity.KeyInt32, afterNodeId mrentity.KeyInt32) error {
    if afterNodeId < 1 {
        return co.MoveToFirst(ctx, nodeId)
    }

    if nodeId < 1 {
        return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"nodeId": nodeId})
    }

    if nodeId == afterNodeId {
        return mrcore.FactoryErrServiceIncorrectInputData.New(mrerr.Arg{"nodeId": nodeId, "afterNodeId": afterNodeId})
    }

    currentNode := EntityNode{Id: nodeId}
    err := co.storage.LoadNode(ctx, &currentNode)

    if err != nil {
        return err
    }

    if mrentity.KeyInt32(currentNode.PrevId) == afterNodeId {
        return nil
    }

    afterNode := EntityNode{Id: afterNodeId}
    err = co.storage.LoadNode(ctx, &afterNode)

    if err != nil {
        return err
    }

    afterNextNode := EntityNode{Id: mrentity.KeyInt32(afterNode.NextId)}

    if afterNextNode.Id > 0 {
        err = co.storage.LoadNode(ctx, &afterNextNode)

        if err != nil {
            return err
        }
    }

    err = co.storage.UpdateNodeNextId(ctx, afterNode.Id, mrentity.ZeronullInt32(currentNode.Id))

    if err != nil {
        return err
    }

    if afterNextNode.Id > 0 {
        err = co.storage.UpdateNodePrevId(ctx, afterNextNode.Id, mrentity.ZeronullInt32(currentNode.Id))

        if err != nil {
            return err
        }
    }

    if currentNode.PrevId > 0 {
        err = co.storage.UpdateNodeNextId(ctx, mrentity.KeyInt32(currentNode.PrevId), currentNode.NextId)

        if err != nil {
            return err
        }
    }

    if currentNode.NextId > 0 {
        err = co.storage.UpdateNodePrevId(ctx, mrentity.KeyInt32(currentNode.NextId), currentNode.PrevId)

        if err != nil {
            return err
        }
    }

    currentNode.PrevId = mrentity.ZeronullInt32(afterNode.Id)
    currentNode.NextId = mrentity.ZeronullInt32(afterNextNode.Id)
    currentNode.OrderField = (afterNode.OrderField + afterNextNode.OrderField) / 2

    if currentNode.OrderField <= afterNode.OrderField {
        if afterNextNode.Id > 0 {
            err = co.storage.RecalcOrderField(ctx, mrentity.Int64(afterNode.OrderField), 2 * orderFieldStep)

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
        nodeId,
        afterNodeId,
    )

    return nil
}

func (co *component) Unlink(ctx context.Context, nodeId mrentity.KeyInt32) error {
    if nodeId < 1 {
        return co.MoveToFirst(ctx, nodeId)
    }

    currentNode := EntityNode{Id: nodeId}
    err := co.storage.LoadNode(ctx, &currentNode)

    if err != nil {
        return err
    }

    if currentNode.PrevId == 0 &&
        currentNode.NextId == 0 &&
        currentNode.OrderField == 0 {
        return nil
    }

    if currentNode.PrevId > 0 {
        err = co.storage.UpdateNodeNextId(ctx, mrentity.KeyInt32(currentNode.PrevId), currentNode.NextId)

        if err != nil {
            return err
        }
    }

    if currentNode.NextId > 0 {
        err = co.storage.UpdateNodePrevId(ctx, mrentity.KeyInt32(currentNode.NextId), currentNode.PrevId)

        if err != nil {
            return err
        }
    }

    currentNode.PrevId = 0
    currentNode.NextId = 0
    currentNode.OrderField = 0

    err = co.storage.UpdateNode(ctx, &currentNode)

    if err != nil {
        return err
    }

    co.eventBox.Emit(
        "%s::Unlink: id=%d",
        ModelNameEntityOrderer,
        nodeId,
    )

    return nil
}
