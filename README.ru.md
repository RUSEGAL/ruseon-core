<p align="center">
  <img src="web/public/favicon.svg" width="150" alt="RUSEON Logo" />
</p>

<h1 align="center">RUSEON Core</h1>

<p align="center">
  <strong>Высокопроизводительная видеоинфраструктура и конвейер данных для ИИ</strong><br>
  <em>RTSP → HLS / WebRTC / fMP4 архив напрямую в RAM без перекодирования</em>
</p>

<p align="center">
  <a href="https://github.com/RUSEGAL/ruseon-core/actions/workflows/test.yml"><img src="https://img.shields.io/github/actions/workflow/status/RUSEGAL/ruseon-core/test.yml?branch=main&style=flat-square" alt="CI Status"></a>
  <a href="https://goreportcard.com/report/github.com/RUSEGAL/ruseon-core"><img src="https://goreportcard.com/badge/github.com/RUSEGAL/ruseon-core?style=flat-square" alt="Go Report Card"></a>
  <a href="https://github.com/RUSEGAL/ruseon-core/releases/latest"><img src="https://img.shields.io/github/v/release/RUSEGAL/ruseon-core?style=flat-square" alt="Latest Release"></a>
  <a href="bench-results/bench-baseline.md"><img src="https://img.shields.io/badge/Бенчмарк-600%20Камер%20%7C%2018k%20FPS%20%7C%200%20Дропов-brightgreen?style=flat-square" alt="Стресс-тест"></a>
  <img src="https://img.shields.io/badge/Покрытие%20CI-14%20Пакетов%20Проверено-blue?style=flat-square" alt="Покрытие CI">
  <img src="https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square" alt="License">
</p>

<p align="center">
  <em>Читать на других языках: <a href="README.md">English</a>, <a href="README.ru.md">Русский</a>.</em>
</p>

<hr>

<p align="center">
  <em>Большинство open-source видеосерверов решают задачу простого стриминга.<br>RUSEON решает задачу полного жизненного цикла видеоданных.<br><strong>Ingest. Route. Record. Analyze. Export.</strong> Один движок.</em>
</p>

**RUSEON Core** — это высокопроизводительная платформа для видеоинфраструктуры, оптимизированная для IP-камер, Edge-вычислений и облачных задач. Работая исключительно по принципу **трансмуксинга** (трансляции медиаконтейнеров без декодирования и повторного сжатия видеопотока), движок репакует H.264/H.265 видео в форматы HLS и WebRTC прямо в оперативной памяти.

Такой **zero-allocation подход** обеспечивает около-нулевую нагрузку на CPU, отдачу master-плейлистов за доли миллисекунды (< 0.5 мс) и горизонтальную масштабируемость, выступая надежным связующим мостом между CCTV-камерами и системами видеоаналитики на базе ИИ.

---

## 🚀 Ключевые возможности

* ⚡ **Zero-Copy Трансмуксинг**: Сверхнизкая задержка при преобразовании RTSP в HLS/WebRTC напрямую в RAM без перекодирования.
* ⚡ **Субмиллисекундные Master-плейлисты**: Неблокирующая генерация и отдача HLS master-плейлистов (< 0.5 мс) с дедупликацией через `singleflight`.
* 🎥 **Ultra-Low Latency (WebRTC WHEP)**: Поддержка протокола WebRTC на базе Pion для воспроизведения с субсекундной задержкой.
* 🧠 **Zero-Transcoding AI Metadata**: Проброс метаданных от нейросетей (Bounding Boxes) напрямую в видеопотоки зрителей через **HLS WebVTT** и **WebRTC DataChannels** без нагрузки на процессор.
* 💾 **Smart I/O Archiver**: Запись видеоархива в fMP4 с системными вызовами ядра Linux (`FADV_DONTNEED`, скользящее окно `sync_file_range`), предотвращающими вымывание дискового кэша ОС (Page Cache) и I/O-фризы.
* 🗄 **Отказоустойчивое хранилище конфигураций**: Встроенная NoSQL-СУБД на базе BadgerDB с синхронной записью на диск (`SyncWrites = true`), атомарными миграциями и гарантированной сохранностью данных при сбоях питания.
* 🛡 **Честные пробы состояния и здоровья**: Строгие пробы `/livez` и `/readyz`, в реальном времени отслеживающие доступность базы данных, файлового хранилища и менеджера потоков.
* 📡 **Событийная архитектура и IoT**: Поддержка вебхуков с Circuit Breaker и публикация событий в MQTT через неблокирующий ring-buffer с контролируемыми таймаутами.
* 🎨 **Современный Edge-дашборд**: Интерфейс на React 19 (TypeScript) с JWT-авторизацией, живой SSE-телеметрией и интерактивным 24-часовым таймлайном fMP4 архива.

[**Подробнее о возможностях в документации**](https://docs.ruseon.tech/guide/features)

---

## 🏗 Архитектура и Конвейер Данных ИИ

RUSEON использует архитектуру неблокирующего кольцевого буфера (**Lock-Free Ring Buffer**). Видеокадры, поступающие по RTSP, сохраняются в памяти один раз и параллельно передаются всем потребителям (HLS, WebRTC, fMP4-архиватор, gRPC-конвейер ИИ) без лишних аллокаций памяти.

```mermaid
flowchart TD
    CAMERA["IP-камера<br/>RTSP (H.264 / H.265)"] -->|TCP / UDP| CORE["Медиа-движок RUSEON"]

    CORE --> RING["Lock-Free RingBuffer"]

    RING --> HLS["HLS Муксер<br/>fMP4 & TS"]
    RING --> WEBRTC["WebRTC / WHEP<br/>Движок Pion"]
    RING --> RECORDER["fMP4 Архиватор<br/>Direct I/O"]
    RING --> AI["ИИ Конвейер<br/>gRPC Приемник"]

    HLS --> BROWSER["Браузеры и плееры"]
    WEBRTC --> CLIENTS["Real-Time операторы"]
    RECORDER --> ARCHIVE["Дисковый архив (BlobStore)"]
    AI --> META["Шина метаданных<br/>WebVTT / DataChannel / MQTT"]
```

[**Архитектурный обзор системы**](https://docs.ruseon.tech/architecture/system)

---

## 🧪 Тестирование, надежность и верификация в CI

В кодовой базе RUSEON Core действует многоуровневая автоматизированная система проверки качества на каждый Pull Request:

```
+-----------------------------------------------------------------------------+
|                       СИСТЕМА ТЕСТИРОВАНИЯ RUSEON                           |
+-----------------------------------------------------------------------------+
|  1. Go Native Fuzzing        | Фаззинг Lock-Free буфера на гонки и переполнение |
|  2. Testcontainers E2E Suite | MediaMTX 1.19.2 + FFmpeg 8 (RTSP -> HLS .ts) |
|  3. Нагрузочные тесты k6     | Аутентифицированные HLS, WHEP, Архив, Пробы  |
|  4. Тесты фронтенда (React)  | 36 тестов на Vitest + React Testing Library  |
|  5. Контроль порогов в CI    | Строгие quality gates для 14 пакетов ядра    |
|  6. Статический аудит        | golangci-lint (0 замечаний) + gosec SAST     |
+-----------------------------------------------------------------------------+
```

### 1. Go Native Fuzz-тестирование (`internal/buffer`)
- Непрерывный фаззинг Lock-Free кольцевого буфера (`fuzz_test.go`), проверяющий поведение при циклическом заполнении, конкурентном чтении/записи, переполнении и соблюдении инварианта нулевых аллокаций.

### 2. Сквозные интеграционные тесты с Testcontainers (`tests/e2e`)
- Тесты выполняются в изолированной Docker bridge-сети с зафиксированными версиями образов:
  - **MediaMTX `1.19.2`** (RTSP-сервер)
  - **FFmpeg `8-alpine`** (генератор живого H.264 видеопотока 10 кадров/сек, GOP = 10)
- Проверяется весь реальный конвейер от начала до конца:
  `FFmpeg Stream -> MediaMTX -> RUSEON RTSP Ingest -> RingBuffer -> /livez & /readyz -> HLS Master Playlist (index.m3u8) -> Video Playlist (stream.m3u8) -> Скачивание и бинарная валидация MPEG-TS сегмента (байт синхронизации 0x47)`.

### 3. Нагрузочные тесты производительности медиа-путей (`tests/performance`)
- Комплексные нагрузочные тесты на **k6** (`k6_load.js`) с JWT-аутентификацией:
  - **Live HLS Stream**: согласование плейлистов и отдача сегментов.
  - **WebRTC WHEP**: рукопожатие SDP offer/answer и ICE-подключение.
  - **Timeshift Archive**: перемотка и воспроизведение fMP4 архива.
  - **Control Plane**: вызовы REST API управления камерами.
  - **Системные пробы**: мониторинг `/livez`, `/readyz` и `/metrics`.

### 4. Тестирование фронтенда (`web/`)
- Набор компонентных и модульных тестов на **Vitest** + **React Testing Library** + **jsdom** (36 тестов в 7 файлах):
  - Математика 24-часового таймлайна, зуммирование и попадание по сегментам архива (`timeline-math.test.ts`).
  - Координатор переподключения потоков с экспоненциальным backoff и джиттером (`reconnect-coordinator.test.ts`).
  - Определение медиа-возможностей браузера (`capabilities.test.ts`).
  - Стейт-машина плеера и иерархия переключения протоколов WebRTC $\to$ HLS (`orchestrator.test.ts`).
  - Тесты формы логина, обработки 401 Unauthorized и переключателя языков.
  - 100% чистый прогон линтера OxLint.

### 5. Автоматический контроль порогов покрытия в CI (`scripts/check_coverage.go`)
- Обязательная проверка порогов покрытия тестами в GitHub Actions для всех ключевых пакетов:

| Пакет | Требуемый порог | Фактическое покрытие | Статус |
| :--- | :---: | :---: | :---: |
| `pkg/registry` | $\ge 90.0\%$ | **100.0%** | **PASS** |
| `pkg/logger` | $\ge 90.0\%$ | **97.0%** | **PASS** |
| `pkg/eventbus` | $\ge 90.0\%$ | **94.5%** | **PASS** |
| `pkg/storage/localfs` | $\ge 85.0\%$ | **90.9%** | **PASS** |
| `pkg/auth` | $\ge 80.0\%$ | **90.2%** | **PASS** |
| `internal/archive` | $\ge 80.0\%$ | **87.8%** | **PASS** |
| `internal/mqtt` | $\ge 80.0\%$ | **84.1%** | **PASS** |
| `pkg/storage` | $\ge 75.0\%$ | **81.9%** | **PASS** |
| `internal/buffer` | $\ge 70.0\%$ | **74.0%** | **PASS** |
| `internal/backup` | $\ge 70.0\%$ | **72.9%** | **PASS** |
| `pkg/config` | $\ge 70.0\%$ | **72.0%** | **PASS** |
| `internal/stream` | $\ge 65.0\%$ | **70.4%** | **PASS** |
| `internal/grpc` | $\ge 65.0\%$ | **70.3%** | **PASS** |
| `internal/recorder` | $\ge 65.0\%$ | **69.7%** | **PASS** |
| **Общее покрытие ядра** | $\ge 50.0\%$ | **62.3%** | **PASS** |

---

## 📊 Результаты нагрузочного тестирования

При стандартизированном полнофункциональном нагрузочном тестировании (**600 одновременных камер 30 FPS**, **1 800 активных HLS зрителей**, **240 WebRTC WHEP клиентов** и **двунаправленный gRPC ИИ-конвейер** на 12-ядерном процессоре):

| Компонент | Метрика | Фактический результат | Латентность / Примечания |
| :--- | :--- | :--- | :--- |
| **Входящий поток (Ingest)** | **18 139 FPS (1 339.9 Mbps)** | 600 камер @ 30 FPS | `0` потерянных кадров |
| **Отдача HLS fMP4** | **500.2 MB/s** (1 098 000 сегментов) | 1 800 одновременных зрителей | `p50: 50.1 ms` / `p95: 277.1 ms` |
| **REST API Роутер** | **375 RPS** (675k запросов, 0 ошибок) | Полная JWT валидация | `p50: 0.63 ms` / `p95: 18.9 ms` |
| **WebRTC WHEP** | **51 793 RTP пакетов** | 240 живых пир-соединений | Хэндшейк Non-Trickle: `568 ms` |
| **gRPC ИИ-конвейер** | **18 136 FPS / 1 900 RPS** | Передача кадров и метаданных | Задержка стрима: `32.4 ms` |
| **Шина событий EventBus** | **582 476 доставленных вебхуков** | 100% успешная доставка | `0` дропов (0.0% потерь) |
| **Потребление памяти** | **471 MB RSS** (140 MB Heap Alloc) | 2 448 активных горутин | Макс. пауза GC: `24.98 ms` |

👉 **[Полный отчет о нагрузочном тестировании (bench-results/bench-baseline.md)](bench-results/bench-baseline.md)**

---

## 🛠 Руководство по локальному запуску тестов

Все проверки качества можно запустить локально перед отправкой кода:

```bash
# 1. Запуск всех тестов бэкенда с детектором гонок и замером покрытия
go test -v -race -timeout=15m -coverprofile=coverage.out ./...

# 2. Запуск Fuzz-тестов кольцевого буфера
go test -fuzz=FuzzRingBuffer -fuzztime=30s ./internal/buffer

# 3. Запуск E2E тестов с Testcontainers (требуется Docker)
go test -v -run=TestE2E ./tests/e2e/...

# 4. Проверка соблюдения порогов покрытия тестами
go run scripts/check_coverage.go coverage.out

# 5. Запуск линтеров и SAST сканера безопасности
golangci-lint run ./...
gosec -exclude-dir=pkg/grpc/pb ./...

# 6. Запуск тестов и линтера фронтенда
cd web
npm run lint
npm test
npm run build
```

---

## 🏎 Быстрый старт

### 1. Запуск через Docker
Запустите RUSEON Core одной командой. При первом старте система сгенерирует надежный пароль администратора и выведет его в консоль.

```bash
docker run -p 8080:8080 -v data:/app/data -v recordings:/app/recordings ghcr.io/rusegal/ruseon-core:latest
```

### 2. Вход в дашборд
Откройте браузер по адресу [http://localhost:8080](http://localhost:8080). Войдите с логином `admin` и сгенерированным паролем.

### 3. Добавление камеры через REST API
Добавляйте и управляйте потоками на лету без перезагрузки сервера:
```bash
curl -X POST http://localhost:8080/api/cameras \
  -H 'Authorization: Bearer ВАШ_JWT_ТОКЕН' \
  -H 'Content-Type: application/json' \
  -d '{"id":"cam1","url":"rtsp://user:pass@192.168.1.100:554/stream","record":true}'
```

[**Полное руководство по быстрому старту**](https://docs.ruseon.tech/guide/quick-start)

---

## 📖 Документация

Официальная документация является единым источником информации о возможностях RUSEON Core:

- **[Начало работы](https://docs.ruseon.tech/guide/introduction)**
- **[Конфигурация](https://docs.ruseon.tech/reference/configuration)**
- **[Архитектура системы](https://docs.ruseon.tech/architecture/overview)**
- **[Стриминг и WebRTC](https://docs.ruseon.tech/streaming/overview)**
- **[Архив и Smart I/O](https://docs.ruseon.tech/archive/overview)**
- **[Справочник REST API](https://docs.ruseon.tech/api/overview)**
- **[Развертывание](https://docs.ruseon.tech/deployment/overview)**
- **[Устранение неполадок](https://docs.ruseon.tech/troubleshooting/overview)**

---

## ⚠️ Известные ограничения

RUSEON Core спроектирован для максимальной производительности при маршрутизации и хранении видео. Движок работает по принципу трансмуксинга и **не производит ресурсоемкое серверное перекодирование видео**. Если ваши камеры выдают форматы, не поддерживаемые напрямую браузерами на целевых устройствах (например, H.265 в старых браузерах), воспроизведение осуществляется через встроенный клиентский WebCodecs/Canvas плеер либо требует внешнего транскодера.

[**Все известные ограничения**](https://docs.ruseon.tech/reference/known-limitations)

---

## 💎 Редакции RUSEON

RUSEON развивается по модели Open Core. Начните бесплатно с Community Edition и переходите на Pro или Enterprise при масштабировании видеоинфраструктуры для получения расширенных возможностей (SSO/OIDC, облачный архив S3, отказоустойчивый кластер High Availability).

> **Готовы к масштабированию?** Свяжитесь с нами: [rusegal.dev@yahoo.com](mailto:rusegal.dev@yahoo.com)

---

## 📄 Лицензия

RUSEON Core (Community Edition) распространяется под лицензией [MIT License](LICENSE).
