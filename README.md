# TripBuilder — Backend

Backend-часть TripBuilder — сервиса для создания маршрутов, планирования
групповых поездок, общения и обмена материалами путешествий.

## Архитектура

Backend организован как набор Go-сервисов:

- gateway;
- auth;
- user;
- route;
- search;
- event;
- notification;
- media;
- chat;
- task;
- shared.

Каждый сервис имеет собственный Go-модуль. Общие middleware, конфигурация и
логирование вынесены в `shared`.

## Технологии

- Go;
- REST API;
- Docker Compose;
- Nginx;
- PostgreSQL/Redis и сервисные зависимости;
- GitHub Actions для dev/test workflow.

## Запуск

1. Скопируйте нужные `.env.example` в локальные `.env`.
2. Укажите собственные значения баз данных и внешних зависимостей.
3. Запустите compose-конфигурацию из `trip-builder`.

Тестовые учётные данные не хранятся в README. Для локальной проверки создайте
собственные тестовые аккаунты.

## Проверка

Backend состоит из нескольких Go-модулей, поэтому тесты запускаются отдельно для
сервисов, а не одной командой из корня:

```bash
go test ./services/auth/...
go test ./services/user/...
```

## Связанные репозитории

- [TripBuilder](https://github.com/Sun1tAr/TripBuilder) — продуктовая документация;
- [B4I-TripBuilder-Frontend](https://github.com/Sun1tAr/B4I-TripBuilder-Frontend) — frontend;
- [MIREA-TripBuilder-FrontEnd](https://github.com/Sun1tAr/MIREA-TripBuilder-FrontEnd) — preview с mock-данными.

> Статус: проектный прототип. Основные сервисы, локальный compose-контур и сценарии
> взаимодействия описаны в репозитории; состав функциональности может расширяться.
