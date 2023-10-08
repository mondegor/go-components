package mrcom_status

type (
    StatusFlow map[Status][]Status

    Status interface {
        String() string
    }
)

// Exists - проверяет, что данный статус имеется в карте статусов
func (f StatusFlow) Exists(status Status) bool {
    _, ok := f[status]

    return ok
}

// Check - проверяет возможность переключения из указанного статуса в указанный статус
func (f StatusFlow) Check(statusFrom Status, statusTo Status) bool {
    statuses, ok := f[statusFrom]

    if !ok {
        return false
    }

    for _, status := range statuses {
        if statusTo == status {
            return true
        }
    }

    return false
}

// PossibleToStatuses - возвращает список статусов в которые можно переключить указанный статус
func (f StatusFlow) PossibleToStatuses(statusFrom Status) []Status {
    if statuses, ok := f[statusFrom]; ok {
        return statuses
    }

    return []Status{}
}

// PossibleFromStatuses - возвращается список статусов из которых можно переключиться в указанный статус
func (f StatusFlow) PossibleFromStatuses(statusTo Status) (statuses []Status) {
    if _, ok := f[statusTo]; !ok {
        return statuses
    }

    for statusFrom, statusesTo := range f {
        for _, status := range statusesTo {
            if statusTo == status {
                statuses = append(statuses, statusFrom)
                break
            }
        }
    }

    return statuses
}
