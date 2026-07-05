package realm

type (
	// Realm - пара числового идентификатора realm и его имени.
	Realm struct {
		ID   uint16
		Name string
	}

	// Registry - неизменяемый in-memory реестр соответствия id <-> name realm'ов.
	Registry struct {
		idByName map[string]uint16
		nameByID map[uint16]string
	}
)

// New - создаёт реестр realm'ов из заданного списка.
func New(realms []Realm) *Registry {
	idByName := make(map[string]uint16, len(realms))
	nameByID := make(map[uint16]string, len(realms))

	for _, r := range realms {
		idByName[r.Name] = r.ID
		nameByID[r.ID] = r.Name
	}

	return &Registry{
		idByName: idByName,
		nameByID: nameByID,
	}
}

// IDByName - возвращает идентификатор realm по его имени.
func (re *Registry) IDByName(name string) (uint16, bool) {
	id, ok := re.idByName[name]

	return id, ok
}

// NameByID - возвращает имя realm по его идентификатору.
func (re *Registry) NameByID(id uint16) (string, bool) {
	name, ok := re.nameByID[id]

	return name, ok
}
