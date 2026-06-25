package session

import (
	"context"
	"crypto/rand"
	"math"
	"math/big"

	"github.com/mondegor/go-sysmess/errors"

	"github.com/mondegor/go-components/mrauth/entity"
	"github.com/mondegor/go-components/mrauth/repository"
)

const (
	// maxInsertAttempts - число попыток подобрать свободный session_id при вставке сессии.
	maxInsertAttempts = 3
)

type (
	// Issuer - выдаёт новую сессию: генерирует уникальный session_id и вставляет её строку,
	// повторяя вставку при коллизии первичного ключа. Скрывает ретраи подбора id от вызывающего.
	Issuer struct {
		storage sessionStorage
	}

	sessionStorage interface {
		Insert(ctx context.Context, row entity.Session) error
	}
)

// NewIssuer - создаёт объект Issuer.
func NewIssuer(storage sessionStorage) *Issuer {
	return &Issuer{
		storage: storage,
	}
}

// Issue - вставляет строку новой сессии, генерируя уникальный session_id и повторяя
// вставку при коллизии PK. Возвращает фактически записанный session_id.
func (s *Issuer) Issue(ctx context.Context, session entity.Session) (sessionID uint32, err error) {
	for attempt := 1; ; attempt++ {
		session.SessionID, err = genSessionID()
		if err != nil {
			return 0, err
		}

		if err = s.storage.Insert(ctx, session); err != nil {
			// session_id занят и попытки не закончились - перегенерируем id и повторяем вставку
			if errors.Is(err, repository.ErrSessionIDCollision) && attempt < maxInsertAttempts {
				continue
			}

			return 0, err
		}

		return session.SessionID, nil
	}
}

// genSessionID - генерирует случайный идентификатор сессии в диапазоне [1, math.MaxUint32].
func genSessionID() (uint32, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(math.MaxUint32))
	if err != nil {
		return 0, err
	}

	// n принадлежит [0, math.MaxUint32), результат [1, math.MaxUint32] помещается в uint32
	return uint32(n.Uint64()) + 1, nil //nolint:gosec
}
