[Назад](../README.md)
---

# Пакет mrnotifier
- [NoteProducer](notifier/service/note_producer.go) - сохраняет уведомления и ставит их в очередь на сборку;
- [SendNotice](notifier/infra/handler/send_notice.go) - обработчик, собирающий уведомления и передающий их на отправку;
- [BuildNotice](notifier/usecase/build_notice.go) - сборка уведомления по шаблону и переменным;
- [Template](template/service/template.go) - получение шаблонов уведомлений;
- [NoticeSender](notifier.go) - интерфейс передачи собранных уведомлений на отправку, реализуется приложением;
- [Пример реализации NoticeSender через mrmailer](../wire/mrmailer/notice_adapter.go);
- [Фабрики](../wire/mrnotifier): producer, processor (обработка очереди), scheduler (обслуживание очереди);

## Верхнеуровневая архитектура
![image](../docs/resources/diagrams/c4/mrnotifier_hld.svg)
