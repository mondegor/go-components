package notify

import (
	"maps"
	"strings"
	"time"

	"github.com/mondegor/go-core/util/conv"

	"github.com/mondegor/go-components/mrauth"
	"github.com/mondegor/go-components/mrauth/dto"
)

type (
	// ActorProps - дополняет props уведомления данными клиента из ActorMeta:
	// временем события в часовом поясе пользователя, IP и устройством.
	ActorProps struct {
		appResolver mrauth.AppResolver
	}
)

// NewActorProps - создаёт объект ActorProps. Если appResolver не задан, устройство
// в уведомлениях остаётся пустым.
func NewActorProps(appResolver mrauth.AppResolver) *ActorProps {
	if appResolver == nil {
		appResolver = func(_ string) (string, string) {
			return "", ""
		}
	}

	return &ActorProps{
		appResolver: appResolver,
	}
}

// With - возвращает новую группу из props события, дополненную ключами occurredAt
// (текущее время в часовом поясе клиента, см. FormatTime), ip (реальный IP клиента, адрес
// из заголовков прокси не выводится) и device (приложение и устройство через запятую, пустые
// части опускаются). Исходная группа props не изменяется.
func (p *ActorProps) With(actor dto.ActorMeta, props conv.Group) conv.Group {
	group := make(conv.Group, len(props)+3)
	maps.Copy(group, props)

	group["occurredAt"] = p.FormatTime(actor, time.Now())
	group["ip"] = actor.ClientIP.Real.String()
	group["device"] = p.device(actor.UserAgent)

	return group
}

// FormatTime - форматирует момент для текста уведомления: дата и время в часовом поясе
// клиента с его названием, чтобы читатель письма не гадал, в каком поясе указано время.
func (p *ActorProps) FormatTime(actor dto.ActorMeta, tm time.Time) string {
	loc := actor.Location()

	return tm.In(loc).Format("2006-01-02 15:04") + " (" + loc.String() + ")"
}

func (p *ActorProps) device(userAgent string) string {
	appName, deviceName := p.appResolver(userAgent)
	parts := make([]string, 0, 2)

	for _, part := range [...]string{appName, deviceName} {
		if part != "" {
			parts = append(parts, part)
		}
	}

	return strings.Join(parts, ", ")
}
