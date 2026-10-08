[Назад](../README.md)
---

# Пакет mrauth
- [HTTP контроллеры](infra/pub/controller/httpv1) и [их контракт](../contracts/mrauth);
- [UseCases](usecase) и [сервисы](service) компонента;
- [UserProvider](infra/adapter/tokenauth/user_provider.go) - пользователь и его права по access токену;
- [Сборщики журналов активности и защищённых операций](infra/adapter/collect);
- [Фабрики](../wire/mrauth): HTTP модуль, провайдеры пользователя, планировщик очистки, сборщики журналов;

## Верхнеуровневая архитектура
![image](../docs/resources/diagrams/c4/mrauth_hld.svg)
