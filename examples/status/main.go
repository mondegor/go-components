package main

import (
    "fmt"

    mrcom_status "github.com/mondegor/go-components/mrcom/status"
)

func main() {
    status := mrcom_status.ItemStatusEnabled

    fmt.Printf("STATUS: %s\n", status.String())

    fmt.Printf("check: %#v\n", mrcom_status.ItemStatusFlow.Check(mrcom_status.ItemStatusEnabled, mrcom_status.ItemStatusDisabled))
    fmt.Printf("check: %#v\n", mrcom_status.ItemStatusFlow.Check(mrcom_status.ItemStatusRemoved, mrcom_status.ItemStatusDisabled))

    fmt.Printf("check: %#v\n", mrcom_status.OnlyRemoveStatusFlow.Check(mrcom_status.OnlyRemoveStatusEnabled, mrcom_status.OnlyRemoveStatusRemoved))
    fmt.Printf("check: %#v\n", mrcom_status.OnlyRemoveStatusFlow.Check(mrcom_status.OnlyRemoveStatusRemoved, mrcom_status.OnlyRemoveStatusEnabled))
}
