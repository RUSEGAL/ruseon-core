# Контекст Проекта: RUSEON Core (на август 2026)

Этот файл создан для передачи полного контекста текущего состояния проекта в новый чат с ИИ-ассистентом.

## О проекте
**RUSEON Core** — это высокопроизводительный сервер ретрансляции RTSP-потоков в HLS и fMP4-архив.
- **Backend:** Go 1.26+, фреймворк Gin (REST API + HLS).
- **Frontend:** React 19, TypeScript, Vite, Vanilla CSS, Lucide Icons. Архитектура UI использует стиль Glassmorphism.
- **База данных:** BadgerDB (LSM-tree, key-value), хранит настройки камер, теги, статистику.
- **Главная фишка:** Zero-copy transmuxing. Потоки не перекодируются (отсутствует FFmpeg). H.264/H.265 NALU-юниты извлекаются из RTSP (через `gortsplib`) и напрямую упаковываются в TS (для HLS) или fMP4 (для архива) через кастомный `RingBuffer`.

## Что уже сделано (Платформа RUSEON Core полностью готова)
- **Ядро и Ingest:** Прием RTSP, RingBuffer, HLS Muxer, MP4 Recorder (с ротацией по дням). Оптимизирован I/O сброс через `sync_file_range` и `FADV_DONTNEED`.
- **Конфигурация:** Полностью динамическое Live API на базе BadgerDB. Добавление камер без рестарта через Swagger или `ruseon-cli`. Реализован Value Log GC для защиты SSD от износа.
- **WebRTC & AI Meta:** Внедрен WebRTC (WHEP) для ультра-низкой задержки. Реализован транзит AI-метаданных (Bounding Boxes) поверх WebRTC DataChannels и HLS WebVTT с отрисовкой на клиенте.
- **Интеграция (IoT & Push):** Встроены Event Bus (Webhooks) и MQTT Publisher для экспорта аналитики во внешние системы (Home Assistant, Node-RED). Защита через MPSC Lock-Free кольцевые буферы.
- **Безопасность и Тесты:** Token-Based Authentication для HLS, Fuzz-тесты кольцевого буфера, k6 нагрузочное тестирование и Testcontainers E2E тесты.

## Что предстоит сделать (RUSEON Enterprise)
Согласно `engineering/ROADMAP.MD`, базовое ядро завершено. Мы переходим к Enterprise-фичам:
1. **Global High Availability (Clustering)** (Синхронизация стейта через Raft, автоматический Failover).
2. **GPU Transcoding (NVENC)**.
3. **S3 Cold Storage Tiering**.

## Структура проекта (Важное)
- `cmd/server/main.go` — Основной демон сервера (API, HLS, WebRTC, Ingest).
- `cmd/ruseon-cli/main.go` — CLI инструмент (`spf13/cobra`) для управления сервером из консоли.
- `internal/stream/` — Управление потоками (`stream.go`, `manager.go`), биллинг (`billing.go`), трекер HLS клиентов (`tracker.go`).
- `internal/hls/muxer.go` — Упаковка NALU в TS-сегменты. Именно здесь предстоит делать Lazy Muxing.
- `internal/archive/` — Запись MP4 (`recorder.go`), отдача HLS из архива (`hls.go`), экспорт MP4 (`export.go`).
- `internal/storage/` — Слой работы с BadgerDB.
- `internal/api/` — Gin-обработчики (`handler.go`, `router.go`).
- `web/src/` — Фронтенд (React). Модалки лежат в `web/src/components/modals/`.

## Инструкции для ИИ (System Prompt)
- Придерживайся принципа Zero-Copy и минимального потребления памяти.
- В интерфейсе строго поддерживай визуальный стиль Glassmorphism и Vanilla CSS. Не используй Tailwind.
- Перед внесением масштабных изменений сверяйся с `docs/ARCHITECTURE.md` и `docs/DECISIONS.md`.
