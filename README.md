# Описание GoComponents v0.9.0
Этот репозиторий содержит описание библиотеки GoComponents.

## Статус библиотеки
Библиотека находится в стадии разработки.

## Описание библиотеки
Библиотека содержит набор компонентов повторного использования:
- Компонент `mrsettings` для хранения и получения произвольных настроек с различными вариантами, в том числе и с использованием кэша;
- Компонент `mrordering` упорядочивания записей на основе двусвязного списка,
  позволяет встраиваться в произвольные таблицы БД;
- Очередь элементов `mrqueue` основанной на БД с возможностями:
  - захвата ограниченного кол-ва элементов для их обработки;
  - повторной обработки элементов при возникновении ошибок;
  - отложенной обработки элементов;
- Компонент `mrmailer` для массовой отправки сообщений различными провайдерами.
  Основан на очереди элементов `mrqueue`, которая даёт все её преимущества;
- Компонент `mrnotifier` для отправки персонализированных уведомлений на основе шаблонов.
  Также основан на очереди элементов `mrqueue`;
- Компонент `mrauth` для аутентификации пользователей: двухфакторная аутентификация (2FA),
  сессии, JWT, защищённые операции, журнал безопасности. HTTP контракт компонента описан
  в [contracts/mrauth](contracts/mrauth), спецификация 2FA — в [docs/analytics/2fa-spec.md](docs/analytics/2fa-spec.md);

Компоненты встраиваются в таблицы БД приложения: репозитории получают имя таблицы и первичного ключа
от вызывающего кода, примеры миграций находятся в `_sample/migrations` компонентов.
Фабрики, собирающие компоненты из их зависимостей, находятся в пакете [wire](wire).

## Подключение библиотеки
`go get -u github.com/mondegor/go-components@v0.9.0`

## Установка библиотеки для её локальной разработки
- Выбрать рабочую директорию, где должна быть расположена библиотека
- `mkdir go-components && cd go-components` // создать и перейти в директорию проекта
- `git clone git@github.com:mondegor/go-components.git .`
- `cp .env.dist .env`
- `mrcmd go-dev deps` // загрузка зависимостей проекта
- Для работы утилит `gofumpt`, `goimports`, `gci`, `golangci-lint`, `mockgen` необходимо запустить
  `mrcmd go-dev install-tools`. По умолчанию `gofumpt`, `goimports`, `gci`, `golangci-lint` устанавливаются
  последних версий; чтобы закрепить версию, раскомментируйте переменную `GO_DEV_TOOLS_INSTALL_*` в `.env`.
  `mockgen` в go-dev по умолчанию выключен — его версия задана в `.env.dist`

### Консольные команды используемые при разработке библиотеки

> Перед запуском консольных скриптов библиотеки необходимо скачать и установить утилиту Mrcmd.\
> Инструкция по её установке находится [здесь](https://github.com/mondegor/mrcmd#readme)

- `mrcmd go-dev help` // выводит список всех доступных go-dev команд;
- `mrcmd go-dev generate` // генерирует go файлы через встроенный механизм go:generate;
- `mrcmd go-dev gofumpt-fix` // исправляет форматирование кода (`gofumpt -l -w -extra ./`);
- `mrcmd go-dev goimports-fix` // исправляет imports, если это требуется (`goimports -l -w -local ${GO_DEV_IMPORTS_LOCAL_PREFIXES}` для всех go файлов, кроме сгенерированных);
- `mrcmd go-dev gci-fix` // упорядочивает imports (`gci`);
- `mrcmd go-dev lint` // запускает линтеры для проверки кода (на основе `.golangci.yaml`);
- `mrcmd go-dev test` // запускает тесты библиотеки;
- `mrcmd go-dev test-report` // запускает тесты библиотеки с формированием отчёта о покрытии кода (`test-coverage-full.html`);
- `mrcmd plantuml build-all` // генерирует файлы изображений из `.puml` [подробнее](https://github.com/mondegor/mrcmd-plugins/blob/master/plantuml/README.md#%D1%80%D0%B0%D0%B1%D0%BE%D1%82%D0%B0-%D1%81-%D0%B4%D0%BE%D0%BA%D1%83%D0%BC%D0%B5%D0%BD%D1%82%D0%B0%D1%86%D0%B8%D0%B5%D0%B9-%D0%BF%D1%80%D0%BE%D0%B5%D0%BA%D1%82%D0%B0-markdown--plantuml);

#### Короткий вариант выше приведённых команд (Makefile)
- `make deps` // аналог `mrcmd go-dev deps`
- `make deps-upgrade` // аналог `mrcmd go-dev get -u ./...` + `mrcmd go-dev tidy`
- `make generate` // аналог `mrcmd go-dev generate`
- `make lint` // аналог `mrcmd go-dev gofumpt-fix` + `goimports-fix` + `gci-fix` + `lint`
- `make test` // аналог `mrcmd go-dev test`
- `make test-report` // аналог `mrcmd go-dev test-report`
- `make plantuml` // аналог `mrcmd plantuml build-all`

> Чтобы расширить список команд, необходимо создать Makefile.mk и добавить
> туда дополнительные команды, все они будут добавлены в единый список команд make утилиты.
> В текущем Makefile.mk: `make check-and-fix` (generate + форматирование + lint + test + plantuml),
> `make full` (deps + check-and-fix).

## Архитектура компонентов
- [mrsettings](mrsettings/README.md) - хранение и получение настроек с кэшированием;
- [mrmailer](mrmailer/README.md) - массовая отправка сообщений через очередь;
- [mrnotifier](mrnotifier/README.md) - персонализированные уведомления на основе шаблонов;
- [mrauth](mrauth/README.md) - аутентификация, 2FA, сессии и журнал безопасности;
