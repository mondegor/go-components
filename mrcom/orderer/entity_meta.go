package mrcom_orderer

type (
    entityMeta struct {
        tableName string
        primaryName string
        conds []any
    }
)

func NewEntityMeta(tableName string, primaryName string, conds ...any) *entityMeta {
    return &entityMeta{
        tableName: tableName,
        primaryName: primaryName,
        conds: conds,
    }
}

func (it *entityMeta) TableName() string {
    return it.tableName
}

func (it *entityMeta) PrimaryName() string {
    return it.primaryName
}

func (it *entityMeta) ForEachCond(fn func (cond any)) {
    for _, cond := range it.conds {
        fn(cond)
    }
}
