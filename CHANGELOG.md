# GoStorage Changelog
Все изменения библиотеки GoComponents будут документироваться на этой странице.

## 2024-06-16
### Added
- Добавлен компонент для доступа к произвольным настройкам;

### Changed
- Обновлена система формирования ошибок в связи с внедрением новой версии библиотеки `go-sysmess`:
    - изменён формат создания новых ошибок;
- Подключены линтеры с их настройками (`.golangci.yaml`);
- Добавлены комментарии для публичных объектов и методов;

## 2024-03-19
### Changed
- Поправлено форматирование документации;

## 2024-03-18
### Changed
- Внедрена новая версия библиотеки `go-sysmess`, заменены `.Caller() -> .WithCaller()`;

## 2024-03-15
### Changed
- Заменено `OrderField -> orderField` и `order_field -> order_index`;

## 2024-02-01
### Fixed
- Исправление оформления в файле README.md;

## 2024-01-30
### Changed
- Заменён устаревший интерфейс `mrcore.EventBox` на `mrsender.EventEmitter`;

## 2024-01-22
### Changed
- `FactoryErrInternalWithData` было заменено на `FactoryErrInternal.WithAttr(...)`;

## 2024-01-16
### Changed
- Переименован интерфейс `mrorderer.Component -> mrorderer.API`;

## 2023-12-10
### Changed
- Заменён `mrerr.Arg -> mrmsg.Data`;
- Доработана логика копирования объектов в `mrorderer.repository.WithMetaData`;

## 2023-12-06
### Changed
- Доработана обработка ошибок и добавлены обёртки для всех ошибок запросов;

## 2023-11-20
### Changed
- Поправлены названия переменных в сообщениях об ошибках;
- Обновлён `.editorconfig`;

## 2023-11-13
### Changed
- Переименованы некоторые переменные и функции (типа Id -> ID) в соответствии с code style языка go;
- Все файлы библиотеки были пропущены через `gofmt`;

## 2023-11-01
### Changed
- Переименован пакет `mrcom_orderer -> mrorderer`;
- В `EntityMeta` заменён метод `ForEachCond` на `Where` с логикой `SqlBuilder` из библиотеки `go-webcore`;

### Removed
- Пакет `mrcom_status` перенесён в библиотеку `go-webcore` (`ItemStatus`);

## 2023-10-08
### Added
- Добавлен статус `OnlyRemoveStatus`;
- Добавлен фильтр `ParseFilterItemStatusList`;
- Добавлен `StatusFlow`;

### Changed
- Перенос объектов "статус" из `mrcom` в `mrcom_orderer`;
- Обработка ошибок приведена к более компактному виду;

## 2023-09-20
### Changed
- Заменён адаптер `*mrpostgres.ConnAdapter` на интерфейс `mrstorage.DbConn`;
- Заменены `tabs` на пробелы в коде;

## 2023-09-16
### Add
- Добавлена модель `ChangeItemStatusRequest`;

## 2023-09-13
### Add
- Добавлен пример работы со статусами;

### Changed
- Добавлен компонент для управления порядком следования элементов;
- Добавлено описание статусов элемента и описаны варианты возможных переключений между ними;

### Fixed
- `QueryUpdate -> SqUpdate`;