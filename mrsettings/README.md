[Назад](../README.md)
---

# Пакет mrsettings
- [SettingsGetter](service/cacheget/settings_getter.go) - отдаёт значения настроек из кэша;
- [SettingsGetter.Reload](service/cacheget/settings_reloader.go) - загружает обновлённые настройки из БД в кэш;
- [SettingsSetter](service/settings_setter.go) - сохраняет значения настроек в БД;
- [Фабрика SettingsGetter с периодическим обновлением кэша](../wire/mrsettings/cacheget/getter.go);

## Верхнеуровневая архитектура
![image](../docs/resources/diagrams/c4/mrsettings_hld.svg)
