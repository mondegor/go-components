package mrcom

import "github.com/mondegor/go-storage/mrentity"

type (
    ChangeItemStatusRequest struct {
        Version mrentity.Version `json:"version"`
        Status  ItemStatus `json:"status" validate:"required,max=16"`
    }
)
