# Дорожная карта (Roadmap) RUSEON Core

## Завершенные этапы (Фундамент MVP)
- [x] **Этап 1-27: Базовая архитектура, HLS, fMP4, Web UI, BadgerDB, Zero-Copy RingBuffer, i18n** (Подробнее в истории коммитов).
- [x] **Этапы 28-41: Защита от OOM/Thundering Herd, WebRTC, Event Bus, Live API + Swagger, CLI, MQTT Publisher, E2E тесты** (Подробнее в истории коммитов).

## RUSEON Enterprise (ruseon-enterprise)
Помимо базового ядра, следующие функции выделены в проприетарную ветку:
- [ ] **Global High Availability (Clustering)** (Синхронизация стейта через Raft, автоматический Failover (миграция) камер при падении сервера-ноды).
- [ ] **GPU Transcoding (NVENC)** (Перекодирование адаптивного битрейта для мобильных клиентов через аппаратное ускорение Nvidia CGO).
- [ ] **S3 Cold Storage Tiering** (Прозрачная и автоматическая выгрузка старых fMP4-записей (старше 7 дней) с быстрого NVMe в Amazon S3 / Minio).

## Экстренные фиксы (Urgent Fixes)
- [x] Внедрение защиты от медленных клиентов (OOM Mitigation, WriteTimeout, IdleTimeout, Non-blocking broadcast).
- [x] Исправление бага с чтением архива отключенных камер.

## Тесты
Покрытие кода тестами критически важных узлов.
- [x] **Performance CI/CD**: Написание и автоматизация бенчмарков `k6` в GitHub Actions для защиты ядра от регрессий пропускной способности при Merge Request'ах.
- [x] **Unit-тесты AI-хендлеров**: Написание моков для gRPC-взаимодействия с `ruseon-yolo`.
