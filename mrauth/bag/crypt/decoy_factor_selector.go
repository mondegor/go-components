package crypt

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/google/uuid"

	"github.com/mondegor/go-components/mrauth/enum/auth2fatype"
)

const (
	percentBase          = 100
	defaultTOTPPercent   = 50
	decoyFactorHashBytes = 4

	// minTOTPPercent, maxTOTPPercent - границы доли подставного TOTP. Края диапазона исключены
	// намеренно: и 0, и 100 раздали бы один и тот же подставной тип всем аккаунтам без 2FA,
	// а вырожденное распределение выдаёт заглушку не хуже расхождения с реальным.
	minTOTPPercent = 1
	maxTOTPPercent = 99

	// minSaltLength - минимальная длина соли (256 бит, размер выхода SHA-256). Проверки
	// на непустоту недостаточно: короткая соль перебирается, а подобравший его вычисляет
	// ожидаемый подставной тип и по расхождению читает, включена ли у аккаунта 2FA.
	minSaltLength = 32
)

type (
	// DecoyFactorSelector - выбирает подставной тип второго фактора для аккаунта, у которого
	// 2FA выключена: вход по аварийному коду обязан выглядеть одинаково при любом состоянии
	// аккаунта, а для этого у подставной цепочки должен быть какой-то тип фактора.
	//
	// Выбор считается HMAC'ом от идентификатора пользователя, а не открытым хешем: алгоритм
	// известен (библиотека открытая), и без секретной соли тот, кто знает userID, вычислил бы
	// ожидаемый тип - а несовпадение однозначно выдавало бы аккаунт с включённой 2FA.
	// Ключ обязан быть постоянным для инсталляции: при его смене подставной тип у одних и тех же
	// аккаунтов поменяется, и это тоже наблюдаемо.
	//
	// Долю totpPercent имеет смысл выставлять близкой к реальному распределению вторых факторов в инсталляции:
	// при заметном расхождении сам тип становится статистическим признаком подставной цепочки.
	DecoyFactorSelector struct {
		salt        []byte
		totpPercent uint32
	}
)

// NewDecoyFactorSelector - создаёт объект DecoyFactorSelector.
func NewDecoyFactorSelector(salt []byte, totpPercent uint32) (*DecoyFactorSelector, error) {
	if len(salt) < minSaltLength {
		return nil, fmt.Errorf("decoy factor selector salt is too short: minimum is %d bytes", minSaltLength)
	}

	if totpPercent < minTOTPPercent || totpPercent > maxTOTPPercent {
		return nil, fmt.Errorf(
			"decoy factor selector totpPercent is out of range: allowed %d..%d", minTOTPPercent, maxTOTPPercent,
		)
	}

	return &DecoyFactorSelector{
		salt:        salt,
		totpPercent: totpPercent,
	}, nil
}

// DefaultTOTPPercent - доля подставного TOTP, используемая, когда реальное распределение
// вторых факторов в инсталляции неизвестно.
func DefaultTOTPPercent() uint32 {
	return defaultTOTPPercent
}

// Select - возвращает подставной тип второго фактора указанного пользователя.
// Результат постоянен для одного userID и предсказуем только при знании соли.
func (s *DecoyFactorSelector) Select(userID uuid.UUID) auth2fatype.Enum {
	mac := hmac.New(sha256.New, s.salt)
	mac.Write(userID[:])

	value := binary.BigEndian.Uint32(mac.Sum(nil)[:decoyFactorHashBytes])

	if value%percentBase < s.totpPercent {
		return auth2fatype.TOTP
	}

	return auth2fatype.Password
}
