package session

const (
	defaultSessionMax   = 4  // максимум одновременных сессий, если для realm/kind не задан лимит
	minSessionThreshold = -4 // минимальное отклонение от лимита для soft сигнала
	maxSessionThreshold = 16 // максимальное отклонение от лимита для hard сигнала
)

type (
	// LimitRealm - лимиты одновременных сессий по видам пользователя (kind) внутри realm.
	LimitRealm struct {
		ID         uint16
		KindLimits []UserKindLimit
	}

	// UserKindLimit - лимит одновременных сессий для вида пользователя (kind).
	UserKindLimit struct {
		Kind       string
		SessionMax int
	}

	// realmKindKey - составной ключ "realm + вид пользователя" для мапы лимитов сессий.
	realmKindKey struct {
		realmID uint16
		kind    string
	}

	// limitResolver - резолвер лимита одновременных сессий по realm/kind. Используется
	// в List (обрезка списка), где нужны только лимиты без порогов soft/hard.
	limitResolver struct {
		limits map[realmKindKey]int
	}

	// sessionLimiter - limitResolver, дополненный производными порогами soft/hard.
	// Используется в OpenSession, где нужны и лимит, и пороги входа.
	sessionLimiter struct {
		*limitResolver
		softThreshold int
		hardThreshold int
	}
)

// newLimitResolver - создаёт резолвер лимитов из конфигурации realm'ов: не заданный (0)
// лимит вида пользователя заменяется значением по умолчанию.
func newLimitResolver(realms []LimitRealm) *limitResolver {
	limits := make(map[realmKindKey]int)

	for _, realm := range realms {
		for _, kind := range realm.KindLimits {
			sessionMax := kind.SessionMax
			if sessionMax < 1 {
				sessionMax = defaultSessionMax
			}

			limits[realmKindKey{realmID: realm.ID, kind: kind.Kind}] = sessionMax
		}
	}

	return &limitResolver{limits: limits}
}

// newSessionLimiter - создаёт резолвер лимитов с порогами: строит limitResolver и
// нормализует отклонения soft/hard порогов (см. correctThresholds).
func newSessionLimiter(realms []LimitRealm, soft, hard int) *sessionLimiter {
	soft, hard = correctThresholds(soft, hard)

	return &sessionLimiter{
		limitResolver: newLimitResolver(realms),
		softThreshold: soft,
		hardThreshold: hard,
	}
}

// Limit - максимум одновременных сессий для realm/kind. Для не сконфигурированной пары
// (в т.ч. пустого kind) возвращает значение по умолчанию.
func (l *limitResolver) Limit(realmID uint16, kind string) int {
	if limit, ok := l.limits[realmKindKey{realmID: realmID, kind: kind}]; ok {
		if limit > 0 {
			return limit
		}
	}

	return defaultSessionMax
}

// thresholds - лимит L и производные пороги для realm/kind:
//   - soft - при достижении ставим пользователя в очередь на фоновую чистку;
//   - hard - при достижении вход временно отклоняется.
//
// Пороги получаются прибавлением настраиваемого отклонения к лимиту и зажимаются снизу
// единицей: при малом лимите и отрицательном отклонении сумма не должна уходить в ноль/минус.
func (l *sessionLimiter) thresholds(realmID uint16, kind string) (limit, soft, hard int) {
	limit = l.Limit(realmID, kind)

	return limit, max(1, limit+l.softThreshold), max(1, limit+l.hardThreshold)
}

// correctThresholds - ограничивает отклонения soft и hard диапазоном
// [minSessionThreshold, maxSessionThreshold] и при необходимости меняет их местами,
// гарантируя soft <= hard.
func correctThresholds(soft, hard int) (softThreshold, hardThreshold int) {
	if soft < minSessionThreshold {
		soft = minSessionThreshold
	}

	if soft > maxSessionThreshold {
		soft = maxSessionThreshold
	}

	if hard < minSessionThreshold {
		hard = minSessionThreshold
	}

	if hard > maxSessionThreshold {
		hard = maxSessionThreshold
	}

	if soft > hard {
		soft, hard = hard, soft
	}

	return soft, hard
}
