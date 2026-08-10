<p align="center">
  <img src="web/public/favicon.svg" width="150" alt="RUSEON Logo" />
</p>

<h1 align="center">RUSEON Core</h1>

<p align="center">
  <strong>Video Data Infrastructure & AI Pipeline</strong><br>
  <em>Облачно-ориентированная, высокопроизводительная платформа для видеоданных</em>
</p>

<p align="center">
  <a href="https://github.com/RUSEON/ruseon-core/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/RUSEON/ruseon-core/ci.yml?branch=main&style=flat-square" alt="CI Status"></a>
  <a href="https://goreportcard.com/report/github.com/RUSEON/ruseon-core"><img src="https://goreportcard.com/badge/github.com/RUSEON/ruseon-core?style=flat-square" alt="Go Report Card"></a>
  <a href="https://github.com/RUSEON/ruseon-core/releases/latest"><img src="https://img.shields.io/github/v/release/RUSEON/ruseon-core?style=flat-square" alt="Latest Release"></a>
  <img src="https://img.shields.io/docker/pulls/ruseon/ruseon-core?style=flat-square" alt="Docker Pulls">
  <img src="https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square" alt="License">
</p>

<p align="center">
  <em>Читать на других языках: <a href="README.md">English</a>, <a href="README.ru.md">Русский</a>.</em>
</p>

<hr>

<p align="center">
  <em>Большинство open-source видеосерверов решают задачу стриминга.<br>RUSEON решает задачу полного жизненного цикла видеоданных.<br>Ingest. Route. Record. Analyze. Export.<br>Один движок.</em>
</p>

**RUSEON Core** — это open-source Edge-платформа для работы с видеоинфраструктурой, написанная на Go. Разработанная для облачных и Edge-сред, она обеспечивает zero-copy ретрансляцию RTSP в HLS/WebRTC, запись архива в fMP4 и закладывает фундаментальный API для систем видеоаналитики на базе ИИ.

Вместо того чтобы пытаться делать всё подряд (например, ИИ-инференс или GPU-перекодирование) внутри одного монолита, RUSEON фокусируется исключительно на максимально эффективной маршрутизации и хранении байтов видео. Он спроектирован как центральный роутер и хранилище, в то время как тяжелые аналитические задачи (YOLO, распознавание лиц) работают как отдельные модули (например, `ruseon-yolo`, `ruseon-lpr`).

## 🚀 Ключевые возможности

* ⚡ **Zero-Copy Ретрансляция**: Сверхнизкая задержка при преобразовании RTSP в HLS/WebRTC напрямую в оперативной памяти. Обходит промежуточное перекодирование для максимальной эффективности.
* 🎥 **Ultra-Low Latency (WebRTC WHEP)**: Поддержка протокола WebRTC для воспроизведения с около-нулевой задержкой, что критически важно для PTZ-камер и real-time аналитики.
* 🧠 **Zero-Transcoding AI Metadata**: Проброс метаданных от нейросетей (Bounding Boxes) напрямую в видеопотоки зрителей через **HLS WebVTT** и **WebRTC DataChannels**, экономя 100% CPU (без ре-энкодинга).
* 💾 **Smart I/O Archiver**: Использование механизмов ядра Linux (`FADV_DONTNEED`, скользящее окно `sync_file_range`) для защиты оперативной памяти (OS Page Cache) от вымывания при записи гигабайтов fMP4 видеоархива. Защищает от I/O stalls.
* 🛡 **Cloud-Native Архитектура и Безопасность**: Встроенная защита от проблемы "Thundering Herd", жесткий OOM-контроль, а также защита API от атак типа Path Traversal (проверено через CodeQL).
* 📦 **Высокопроизводительный архив (fMP4)**: Непрерывная запись без потерь в формат fragmented MP4. Оптимизировано для хранения на Edge-устройствах и быстрой синхронизации с облаком.
* ⏪ **Продвинутый Timeshift-конвейер**: Воспроизведение архивных данных в формате HLS в реальном времени с возможностью бесшовного экспорта датасетов для обучения ИИ.
* 🗄 **Встроенная NoSQL-СУБД**: Работает на базе BadgerDB, обеспечивая субмиллисекундное сохранение конфигураций и метрик без необходимости во внешних зависимостях.
* 🎨 **Современный UI-мониторинг**: Включает Edge-дашборд на React 19 (TypeScript) с JWT-авторизацией, телеметрией SSE в реальном времени и удобной визуализацией таймлайна.
* 🧪 **Надежность (Enterprise Reliability)**: Кодовая база защищена автоматизированными CI-пайплайнами производительности k6, непрерывным профилированием (pprof) и тестами Chaos Engineering для надежного предотвращения регрессий.

---

## ⚔️ Философия и Сравнение

Философия RUSEON Core сильно отличается от других популярных инструментов в экосистеме видео. Вместо прямой конкуренции, он решает совершенно иную архитектурную задачу:

- **MediaMTX**: Великолепный **Медиа-маршрутизатор**. Фокусируется на трансляции огромного числа протоколов (RTSP, WebRTC, RTMP, SRT). RUSEON, напротив, сфокусирован на **жизненном цикле видеоданных** (архив, timeshift, экспорт датасетов).
- **FFmpeg**: Ультимативный **Медиа-инструментарий**. Идеален для обработки видео, но требует сложного скриптинга для создания надежного демона, управляемого по API.
- **Flussonic**: Комплексная **IPTV-платформа**, созданная для телекома и броадкастинга.
- **RUSEON Core**: Инфраструктурная **Edge-видеоплатформа**. Создана специально для приема потоков с CCTV камер, их надежного сохранения на диск и эффективной раздачи операторам и конвейерам ИИ.

## 🏗 Архитектура и Конвейер Данных ИИ

RUSEON Core выступает в роли критически важного связующего звена между Edge-оборудованием и вашими Облачными/ИИ нагрузками.

```mermaid
graph LR
  subgraph Edge [Edge Devices / Cameras]
    Cam[RTSP Streams]
  end

  subgraph Engine [RUSEON Core]
    API[Live REST / SSE API]
    gRPC[gRPC AI Receiver]
    Demux[Zero-Copy Demuxer]
    Pool[RingBuffer / Pool]
    Meta[Metadata Bus]
    HLS[HLS Muxer + WebVTT]
    RTC[WebRTC WHEP + DataChannel]
    Rec[fMP4 Storage 'Direct I/O']
    Storage[(Video Archive / BlobStore)]
    MQTT[MQTT Publisher]
    DB[(BadgerDB Control Plane)]
  end

  subgraph Clients [Clients & Ecosystem]
    Dashboard[React Dashboard]
    CLI[ruseon-cli / Swagger]
    Player[Video Clients]
    IoT[IoT Broker / Home Assistant]
    AI[AI / CV Models]
  end

  %% Ingest Pipeline
  Cam -->|H.264 / HEVC| Demux
  Demux --> Pool
  
  %% Delivery Pipeline
  Pool --> HLS
  Pool --> RTC
  Pool --> Rec
  
  %% State Management
  DB -.->|Config & State| API
  API -.-> Demux
  API <--> CLI
  
  %% AI Metadata Loop
  AI -->|Push Bounding Boxes| gRPC
  gRPC -->|Metadata| Meta[Metadata Bus]
  Meta --> HLS
  Meta --> RTC
  Meta --> MQTT
  
  %% Outputs
  HLS -->|M3U8 / TS| Player
  RTC -->|Sub-second Latency| Player
  Rec -->|fMP4 Archive| Storage
  Storage -->|Dataset Export| AI
  MQTT -->|JSON Telemetry| IoT
  
  %% Observability
  Dashboard <-->|Metrics & Config| API
```

---

## 📊 Производительность

RUSEON Core спроектирован для максимальной эффективности. В его основе лежит маршрутизатор, который обеспечивает **zero-copy / low-copy frame distribution with allocation minimization on the hot path**.

Архитектура продемонстрировала высокую стабильность в наших бенчмарк-сценариях. RUSEON Core эффективно утилизировал локальные сетевые интерфейсы 10G, достигая пропускной способности **~8.8 Gbit/s end-to-end** для раздачи HLS.

Для просмотра полных результатов автоматизированных бенчмарков, стресс-тестов и отчетов Chaos Engineering, пожалуйста, обращайтесь к нашему единому источнику правды:
👉 **[benchmarks/RESULTS.md](benchmarks/RESULTS.md)**

---

## 🏎 Быстрый старт

### Требования
- [Docker](https://www.docker.com/) (Рекомендуется для быстрого развертывания)
- [Go](https://go.dev/) 1.26+ (Для сборки из исходников)

### Развертывание через Docker (GHCR) 🐳

Самый быстрый способ развернуть RUSEON Core — использовать наш официальный multi-arch Docker-образ:

```bash
docker run -d \
  --network host \
  -v ruseon-data:/data \
  --name ruseon-core \
  ghcr.io/ruseon/ruseon-core:latest
# Примечание: --network host обязателен для корректной маршрутизации WebRTC по UDP.
# В противном случае пробросьте -p 8080:8080 и ваш настроенный ICE UDP порт.
```

RUSEON Edge-дашборд будет доступен по адресу `http://localhost:8080`.

### Сборка из исходников

Для разработчиков и контрибьюторов:

```bash
# 1. Клонирование репозитория
git clone https://github.com/RUSEON/ruseon-core.git
cd ruseon-core

# 2. Сборка Edge-дашборда (React)
cd web && npm install && npm run build && cd ..

# 3. Запуск ядра (Core Engine)
go mod tidy
go run ./cmd/server
```
*(On first startup, RUSEON generates a random administrator password and prints it once to the server console.)*

---

## 🤝 Участие в разработке

Мы верим в силу open-source и приветствуем любой вклад от сообщества.
Будь то баг-репорт, новая фича или улучшение документации, пожалуйста, ознакомьтесь с нашим [Руководством для контрибьюторов](CONTRIBUTING.ru.md) для начала работы.

Убедитесь, что ваши коммиты соответствуют спецификации [Conventional Commits](https://www.conventionalcommits.org/).

## 📄 Лицензия

RUSEON Core (Community Edition) распространяется под лицензией [MIT](LICENSE).
