package mrcom_status

import (
    "net/http"

    "github.com/mondegor/go-webcore/mrreq"
)

func ParseFilterItemStatusList(r *http.Request, key string, def ItemStatus, items *[]ItemStatus) error {
    enums, err := mrreq.EnumList(r, key)

    if err != nil {
        return err
    }

    tmpItems, err := ParseItemStatusList(enums)

    if err != nil {
        return err
    }

    if len(tmpItems) == 0 {
        tmpItems = []ItemStatus{def}
    }

    *items = tmpItems

    return nil
}
