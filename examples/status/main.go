package main

import (
    "fmt"

    "github.com/mondegor/go-components/mrcom"
)

func main() {
    status := mrcom.ItemStatusEnabled

    fmt.Printf("STATUS: %s\n", status.String())

    fmt.Printf("check: %v\n", mrcom.ItemStatusFlowDefault.Check(mrcom.ItemStatusEnabled, mrcom.ItemStatusDisabled))
    fmt.Printf("check: %v\n", mrcom.ItemStatusFlowDefault.Check(mrcom.ItemStatusRemoved, mrcom.ItemStatusDisabled))
}
