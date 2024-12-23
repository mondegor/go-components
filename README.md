# Описание GoComponents v0.8.2
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

## Подключение библиотеки
`go get -u github.com/mondegor/go-components@v0.8.2`

## Установка библиотеки для её локальной разработки
- Выбрать рабочую директорию, где должна быть расположена библиотека
- `mkdir go-components && cd go-components` // создать и перейти в директорию проекта
- `git clone git@github.com:mondegor/go-components.git .`
- `cp .env.dist .env`
- `mrcmd go-dev deps` // загрузка зависимостей проекта
- Для работы утилит `gofumpt`, `goimports`, `mockgen` необходимо в `.env` проверить
  значения переменных `GO_DEV_TOOLS_INSTALL_*` и запустить `mrcmd go-dev install-tools`

### Консольные команды используемые при разработке библиотеки

> Перед запуском консольных скриптов библиотеки необходимо скачать и установить утилиту Mrcmd.\
> Инструкция по её установке находится [здесь](https://github.com/mondegor/mrcmd#readme)

- `mrcmd go-dev help` // выводит список всех доступных go-dev команд;
- `mrcmd go-dev generate` // генерирует go файлы через встроенный механизм go:generate;
- `mrcmd go-dev gofumpt-fix` // исправляет форматирование кода (`gofumpt -l -w -extra ./`);
- `mrcmd go-dev goimports-fix` // исправляет imports, если это требуется (`goimports -d -local ${GO_DEV_IMPORTS_LOCAL_PREFIXES} ./`);
- `mrcmd golangci-lint check` // запускает линтеров для проверки кода (на основе `.golangci.yaml`);
- `mrcmd go-dev test` // запускает тесты библиотеки;
- `mrcmd go-dev test-report` // запускает тесты библиотеки с формированием отчёта о покрытии кода (`test-coverage-full.html`);
- `mrcmd plantuml build-all` // генерирует файлы изображений из `.puml` [подробнее](https://github.com/mondegor/mrcmd-plugins/blob/master/plantuml/README.md#%D1%80%D0%B0%D0%B1%D0%BE%D1%82%D0%B0-%D1%81-%D0%B4%D0%BE%D0%BA%D1%83%D0%BC%D0%B5%D0%BD%D1%82%D0%B0%D1%86%D0%B8%D0%B5%D0%B9-%D0%BF%D1%80%D0%BE%D0%B5%D0%BA%D1%82%D0%B0-markdown--plantuml);

#### Короткий вариант выше приведённых команд (Makefile)
- `make deps` // аналог `mrcmd go-dev deps`
- `make generate` // аналог `mrcmd go-dev generate`
- `make fmt` // аналог `mrcmd go-dev gofumpt-fix`
- `make fmti` // аналог `mrcmd go-dev goimports-fix`
- `make lint` // аналог `mrcmd golangci-lint check`
- `make test` // аналог `mrcmd go-dev test`
- `make test-report` // аналог `mrcmd go-dev test-report`
- `make plantuml` // аналог `mrcmd plantuml build-all`

> Чтобы расширить список команд, необходимо создать Makefile.mk и добавить
> туда дополнительные команды, все они будут добавлены в единый список команд make утилиты.

## Примеры архитектуры системы с использованием библиотеки go-components

### Пакет mrsettings
- [CacheGetter + Loader](https://github.com/mondegor/go-components/blob/master/mrsettings/component/cachegetter/cache_getter.go)
- [Setter](https://github.com/mondegor/go-components/blob/master/mrsettings/component/setter/component_setter.go)

![image](docs/resources/packages/c4/mrsettings.svg)

### Подсистема планировки задач
- [Scheduler](https://github.com/mondegor/go-webcore/blob/master/mrworker/mrschedule/scheduler.go)
- [Task](https://github.com/mondegor/go-webcore/blob/master/mrworker/mrschedule/task_shell.go)

![image](docs/resources/packages/c4/scheduler.svg)

### Сервис использующий пакет mrsettings
- [Фабрика пакета mrsettings](https://github.com/mondegor/go-sample/blob/master/app/cmd/factory/settings_manager.go)
- [Подключение пакета mrsettings](https://github.com/mondegor/go-sample/blob/bffd398fbc8cb7d3a3a8c521dc4d2babed0061ae/app/cmd/factory/app_environment.go#L137-L143)

![image](docs/resources/packages/c4/app.svg)

### Верхнеуровневая архитектура
![image](docs/resources/diagrams/c4/mrsettings_hld.svg)