package initing

import (
	"github.com/mondegor/go-sysmess/mraccess"
	"github.com/mondegor/go-sysmess/mrlog"

	"github.com/mondegor/go-components/wire/mrauth/config"
)

// InitRealmKindRights - создаёт объект mraccess.RightsGetter.
func InitRealmKindRights(logger mrlog.Logger, realms []config.UserRealm, rights mraccess.RightsSource) (mraccess.RightsGetter, error) {
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
					Name:  realm.Name + "/" + kind.Name, // realm/kind
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
