package initing

import (
	"github.com/mondegor/go-core/mraccess"
	"github.com/mondegor/go-core/mrlog"

	"github.com/mondegor/go-components/mrauth/model/usergroup"
	"github.com/mondegor/go-components/wire/mrauth/config"
)

type (
	// roleRightsSource - узкий источник прав по имени роли (роль -> множество прав).
	// Удовлетворяется *filestorage.PermsProvider; объявлен здесь, чтобы не привязывать
	// wire к конкретной реализации провайдера прав.
	roleRightsSource interface {
		RoleRights(role string) (rights []string, ok bool)
	}
)

// InitRealmKindRights - создаёт объект mraccess.RightsGetter.
// Источником прав по ролям выступает roleRightsSource (роль -> множество прав),
// поверх которого для каждой пары realm/kind предвычисляется набор прав группы.
func InitRealmKindRights(logger mrlog.Logger, realms []config.UserRealm, rights roleRightsSource) (mraccess.RightsGetter, error) {
	mrlog.Info(logger, "Create and init realm kind rights")

	nKinds := 0
	for _, realm := range realms {
		nKinds += len(realm.UserKinds)
	}

	realmKinds := make([]mraccess.RoleGroup, 0, nKinds)

	for _, realm := range realms {
		for _, kind := range realm.UserKinds {
			if len(kind.Roles) == 0 {
				continue
			}

			realmKinds = append(
				realmKinds,
				mraccess.RoleGroup{
					Name:  usergroup.Build(realm.Name, kind.Name),
					Roles: kind.Roles,
				},
			)
		}
	}

	return mraccess.NewRolesGroupSet(
		realmKinds,
		rights,
	)
}
