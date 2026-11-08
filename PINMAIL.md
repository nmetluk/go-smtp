# Форк go-smtp для Pinmail

Ветка `pinmail` основана на rest-mail/go-smtp **v0.32.0**,
коммит `5f05598a5cd58424ad69070079458b382dd1b659`.
`main` сохраняется зеркалом rest-mail: изменения Pinmail туда не попадают.
Релизы: `v0.32.0-pinmail.N`; первый — `v0.32.0-pinmail.1`.

Путь модуля остаётся `github.com/rest-mail/go-smtp`. Pinmail подключает форк:

```go
replace github.com/rest-mail/go-smtp => github.com/nmetluk/go-smtp v0.32.0-pinmail.1
```

Задача форка — заменить локальную правку vendor и фильтрацию SMTP-байтов
явными API библиотеки (ADR-0014 в nmetluk/pinmail). Это транспортная
библиотека: SRS, маршрутизация, защита назначения и WAL остаются в Pinmail.

## Изменения Pinmail

1. `Client.StartTLS(*tls.Config)` — публичный вызов существующего `startTLS`,
   после `Hello` со своим именем. Сохраняет политику и реализацию rest-mail:
   вызывающий проверяет объявление STARTTLS и передаёт TLS-конфигурацию.
   После TLS повторный EHLO использует то же имя.
2. `DataResponse.Code` сохраняет точный успешный код SMTP после конца DATA,
   включая 251/259. `StatusText` не меняется. Ошибки остаются SMTPError.
   Как и исходная поправка Pinmail, поле заполняется SMTP-путём
   CloseWithResponse; в LMTP оно пока нулевое, LMTP в Pinmail не используется.
3. `Server.DisableCHUNKING` (по умолчанию false) отключает CHUNKING и
   BINARYMIME в EHLO/LHLO, включая дополнительные возможности из ExtraCaps
   и FeatureBackend. Приоритет выше EnableBINARYMIME. MAIL с BODY=BINARYMIME
   получает 504 5.5.4. BDAT получает **502 5.5.1** и соединение закрывается:
   непрочитанный payload не может стать SMTP-командами. Настраивать до Serve.
   Без флага поведение прежнее. Работает также после STARTTLS; сам сервер
   Pinmail включит входящий TLS отдельной задачей M2.
4. CI запускается также для ветки `pinmail`. Состав проверок rest-mail
   сохранён: vet/build, race, покрытие >=95%, RFC checklist и fuzz smoke.

Новые проверки зарегистрированы в существующем RFC checklist без изменения
числа пунктов или порога покрытия. На macOS для полного прогона используется
`TMPDIR=/tmp`: штатный длинный каталог временных файлов превышает предел
пути Unix-сокета в исходном LMTP-тесте.

Новые тесты — `pkg/smtp/client/pinmail_test.go` и
`pkg/smtp/server/pinmail_test.go`: пользовательский EHLO до/после TLS,
точный код/текст DATA, таблица default/disabled/BINARYMIME, обычный BDAT,
запрет BDAT с нулевым, ненулевым и неверным размером, защита от повторного
объявления через backend, проверка флага после реального STARTTLS.

## Ревью rest-mail относительно emersion v0.25.1

Проверено 2026-11-08 по исходникам и истории, а не только CHANGELOG:

- [emersion v0.25.1](https://github.com/emersion/go-smtp/tree/v0.25.1):
  `0ed1f057d5110fb8071dcfb7b8d2e72ec05067a6`;
- [rest-mail v0.32.0](https://github.com/rest-mail/go-smtp/tree/v0.32.0):
  `5f05598a5cd58424ad69070079458b382dd1b659`;
- общий предок: `24034bd94c58db6d68dae5d8f44c1fee361cd0b4`;
- `git rev-list --left-right --count upstream-v0.25.1...v0.32.0`: **7 / 96**.
  Это числа уникальных коммитов, включая тесты, документацию и CI.

| Область | Коммиты rest-mail | Изменение и значение для Pinmail |
| --- | --- | --- |
| Структура API | [94f58de](https://github.com/rest-mail/go-smtp/commit/94f58de), [5f05598](https://github.com/rest-mail/go-smtp/commit/5f05598) | Путь модуля rest-mail и разделение на pkg/smtp, client, server; исходный корневой пакет удалён |
| Гонки TLS/остановки | [b2761b4](https://github.com/rest-mail/go-smtp/commit/b2761b4), [083ce65](https://github.com/rest-mail/go-smtp/commit/083ce65) | Замена соединения STARTTLS синхронизирована с Close; доступ к pipe BDAT защищён при остановке |
| BDAT и границы данных | [4023a1c](https://github.com/rest-mail/go-smtp/commit/4023a1c), [f54df46](https://github.com/rest-mail/go-smtp/commit/f54df46), [909aea7](https://github.com/rest-mail/go-smtp/commit/909aea7), [42033cb](https://github.com/rest-mail/go-smtp/commit/42033cb) | Граница команд после BDAT, корректное отбрасывание чанков; при повреждении framing соединение закрывается, payload не становится командами |
| Парсинг MAIL/RCPT и остановка | [7939ad6](https://github.com/rest-mail/go-smtp/commit/7939ad6), [a8d9725](https://github.com/rest-mail/go-smtp/commit/a8d9725), [09297fc](https://github.com/rest-mail/go-smtp/commit/09297fc) | Проверка разделителей и параметров, управляющих символов/повторного @; сервисный 421 при Close вместо тихого обрыва |
| DATA и размеры | [900ee33](https://github.com/rest-mail/go-smtp/commit/900ee33), [40a0752](https://github.com/rest-mail/go-smtp/commit/40a0752), [d338709](https://github.com/rest-mail/go-smtp/commit/d338709), [9ec7a47](https://github.com/rest-mail/go-smtp/commit/9ec7a47) | Включительная граница MaxMessageBytes, лимит строки только для команд, CRLF framing, отдельный таймаут сетевых записей DATA |
| Ответы и клиент | [342a1d2](https://github.com/rest-mail/go-smtp/commit/342a1d2), [3c4f9e9](https://github.com/rest-mail/go-smtp/commit/3c4f9e9), [f1c4132](https://github.com/rest-mail/go-smtp/commit/f1c4132) | Пользовательские успешные ответы обработчика, обработка классов SMTP-кодов, проверка TLS handshake и сброс сведений EHLO |
| Дополнительные API | [50d883f](https://github.com/rest-mail/go-smtp/commit/50d883f), [5a11280](https://github.com/rest-mail/go-smtp/commit/5a11280), [14d5359](https://github.com/rest-mail/go-smtp/commit/14d5359), [776f852](https://github.com/rest-mail/go-smtp/commit/776f852) | Context, FeatureBackend, XCLIENT, клиентский PIPELINING; Pinmail не включает их автоматически |
| Проверяемые протоколы | [866dbb1](https://github.com/rest-mail/go-smtp/commit/866dbb1), [5b756a3](https://github.com/rest-mail/go-smtp/commit/5b756a3), [2ec81e6](https://github.com/rest-mail/go-smtp/commit/2ec81e6), [90aed53](https://github.com/rest-mail/go-smtp/commit/90aed53) | RFC checklist, fuzz и порог покрытия; отдельные границы AUTH-строк |

Семь коммитов только на стороне emersion не следует считать семью
неперенесёнными ошибками. Сопоставлены следующие изменения:

| emersion | Соответствие в rest-mail |
| --- | --- |
| f02fdf9: AUTH как данные | 78869ba — соответствующая поправка |
| ddebd6a: сброс списка RCPT клиента | 0d04e1d — соответствующая поправка |
| 785ce2a: дочитывание oversized LMTP | f5ff635 и тесты LMTP/size limit |
| 402a482: discard отвергнутого BDAT | 909aea7, 42033cb: явное дочитывание и отказ при обрыве |
| 642c374, 0ed1f05: восстановление line limit | 4023a1c/f54df46; лимит команд передаётся при каждом чтении, payload читается отдельно. Регрессионный тест 642c374 перенесён в 5f05598 |
| 6a37d8b | Исправление опечатки в комментарии |

Вывод ревью: rest-mail выбран из-за нужных транспортных исправлений и
контроля точных ответов. Изменений существенно больше двух добавок Pinmail,
поэтому это не доказательство отсутствия иных ошибок или полный аудит
безопасности. Расширения, которыми Pinmail не пользуется, не включаются
без отдельной постановки; обязательны тесты обеих сторон перед обновлением.

## Квартальная сверка

Ответственный — сопровождающий Pinmail; первая следующая сверка — февраль
2027, далее не реже раза в квартал. При исправлении уязвимости — вне очереди.

1. Обновить ссылки rest-mail и emersion, сохранить проверенные теги/коммиты.
2. Сравнить историю и runtime-diff с предыдущей базой; отдельно разобрать
   TLS/Close, framing DATA/BDAT, MAIL/RCPT, отмену/таймауты и security fixes.
3. Перенести нужные исправления в `pinmail`, сохранив минимальный diff.
   Обновление базы/тегов не должно переписывать уже опубликованный релиз.
4. Прогнать тесты форка с race/coverage, checklist/fuzz CI и Pinmail
   (`make vendor-check`, `make ci`, `make lint`, `make vuln`).
5. Предложить публичный StartTLS/Code и выключатель CHUNKING upstream после
   согласования. Удалить локальные добавки, если появился эквивалентный API.
6. Выпустить новый тег, обновить replace/go.sum/vendor в Pinmail и записать
   ревью с источниками. `main` форка обновлять только как зеркало rest-mail.

Этот документ задаёт регламент, но не создаёт автоматическое расписание.
