# GoStorage Changelog
Все изменения библиотеки GoComponents будут документироваться на этой странице.

## 2023-11-01
### Changed
- Обновлены зависимости библиотеки;
- Переименован пакет `mrcom_orderer` -> `mrorderer`;
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
- Обновлены зависимости библиотеки;
- Обработка ошибок приведена к более компактному виду;

## 2023-09-20
### Changed
- Заменён адаптер `*mrpostgres.ConnAdapter` на интерфейс `mrstorage.DbConn`;
- Обновлены зависимости библиотеки;
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
- `QueryUpdate` -> `SqUpdate`;