package mrcom_status

import (
    "encoding/json"

    "github.com/mondegor/go-webcore/mrcore"
)

const (
    _ OnlyRemoveStatus = iota
    OnlyRemoveStatusEnabled
    OnlyRemoveStatusRemoved

    enumNameOnlyRemoveStatus = "OnlyRemoveStatus"
)

type (
    OnlyRemoveStatus uint8
)

var (
    onlyRemoveStatusName = map[OnlyRemoveStatus]string{
        OnlyRemoveStatusEnabled: "ENABLED",
        OnlyRemoveStatusRemoved: "REMOVED",
    }

    onlyRemoveStatusValue = map[string]OnlyRemoveStatus{
        "ENABLED": OnlyRemoveStatusEnabled,
        "REMOVED": OnlyRemoveStatusRemoved,
    }

    OnlyRemoveStatusFlow = StatusFlow{
        OnlyRemoveStatusEnabled: {
            OnlyRemoveStatusRemoved,
        },
        OnlyRemoveStatusRemoved: {},
    }
)

func (e *OnlyRemoveStatus) ParseAndSet(value string) error {
    if parsedValue, ok := onlyRemoveStatusValue[value]; ok {
        *e = parsedValue
        return nil
    }

    return mrcore.FactoryErrInternalMapValueNotFound.New(value, enumNameOnlyRemoveStatus)
}

func (e OnlyRemoveStatus) String() string {
    return onlyRemoveStatusName[e]
}

func (e OnlyRemoveStatus) MarshalJSON() ([]byte, error) {
    return json.Marshal(e.String())
}

func (e *OnlyRemoveStatus) UnmarshalJSON(data []byte) error {
    var value string

    if err := json.Unmarshal(data, &value); err != nil {
        return err
    }

    return e.ParseAndSet(value)
}

// Scan implements the Scanner interface.
func (e *OnlyRemoveStatus) Scan(value any) error {
    if val, ok := value.(string); ok {
        return e.ParseAndSet(val)
    }

    return mrcore.FactoryErrInternalTypeAssertion.New(enumNameOnlyRemoveStatus, value)
}
