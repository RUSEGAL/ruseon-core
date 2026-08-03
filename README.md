<p align="center">
  <img src="web/public/favicon.svg" width="150" alt="REA Stream Engine Logo" />
</p>

<h1 align="center">REA Stream Engine</h1>

<p align="center">
  <strong>Высокопроизводительный сервер ретрансляции RTSP в HLS и записи архива.</strong><br>
  <em>(Zero-Copy Transmuxing, fMP4 Archive, React 19 Dashboard)</em>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/Go-1.23+-00ADD8?style=flat-square&logo=go" alt="Go Version">
  <img src="https://img.shields.io/badge/React-19-61DAFB?style=flat-square&logo=react" alt="React">
  <img src="https://img.shields.io/badge/License-MIT-blue.svg?style=flat-square" alt="License">
  <img src="https://img.shields.io/badge/Architecture-Zero--Copy-success?style=flat-square" alt="Zero-Copy">
</p>

<hr>

**REA Stream Engine** — это современное Enterprise-решение для видеонаблюдения и потокового вещания, написанное на Go и React. Он обеспечивает захват RTSP потоков и их "на лету" переупаковку в HLS прямо в оперативной памяти, запись видеоархива без потери кадров и продвинутую систему таймшифта.

## 📑 Оглавление
- [✨ Ключевые возможности](#-ключевые-возможности)
- [🏗 Архитектура](#-архитектура)
- [🚀 Быстрый старт](#-быстрый-старт)
- [🛠 Установка (Production)](#-установка-production)
- [⚙️ Конфигурация](#️-конфигурация)
- [📚 Документация](#-документация)

---

## ✨ Ключевые возможности

* ⚡ **Молниеносная ретрансляция (Zero-Copy)**: Никакого FFmpeg и промежуточного перекодирования. Использование чистого Go (`gortsplib` и `mediacommon`) для получения RTSP и упаковки в HLS "на лету" прямо в RAM.
* 🛡 **Thundering Herd Protection**: Защита от лавинных нагрузок при одновременном подключении тысяч клиентов.
* 📦 **Умная запись (fMP4)**: Непрерывная запись потоков в fragmented MP4. Уникальная система бесшовной нарезки файлов при ротации без потери единого кадра.
* ⏪ **Timeshift (Плеер Архива)**: Мгновенный просмотр архива через веб-интерфейс, графический таймлайн и скачивание готовых `.mp4` отрезков.
* 🗄 **BadgerDB (NoSQL)**: Встроенная сверхбыстрая LSM-tree база данных для конфигураций и статистики (миллисекундный отклик, минимальное потребление RAM).
* 🔄 **Горячее управление и Бэкапы**: Добавление/удаление камер без перезагрузки сервера. Автоматические бинарные бэкапы БД и экспорт/импорт в JSON.
* 🎨 **Glassmorphism UI**: Фронтенд на React 19 (Vite, TypeScript) с JWT-авторизацией, SSE логами реального времени и локализацией (i18n).

---

## 🏗 Архитектура

```mermaid
graph LR
  subgraph Data Sources
    Cam1[RTSP Camera]
    Cam2[RTSP Camera]
  end

  subgraph REA Stream Engine (Go)
    Demux[Zero-Copy Demuxer]
    Pool[Memory Pool]
    HLS[HLS Muxer]
    Rec[fMP4 Recorder]
    DB[(BadgerDB)]
  end

  subgraph Clients
    Browser[Web Dashboard]
    Player[HLS / MP4 Player]
  end

  Cam1 & Cam2 -->|H.264/H.265| Demux
  Demux --> Pool
  Pool --> HLS
  Pool --> Rec
  DB -.->|Configs & Stats| Demux
  HLS -->|Live Stream| Player
  Rec -->|Archive| Player
  Browser <-->|REST API & SSE| Demux
```

---

## 🚀 Быстрый старт

### Требования
- [Go](https://go.dev/) 1.23+
- [Node.js](https://nodejs.org/) 18+

### 1. Сборка фронтенда
```bash
cd web
npm install
npm run build
cd ..
```

### 2. Запуск бэкенда
```bash
go mod tidy
go run ./cmd/server
```

### 3. Доступ к панели управления
Откройте браузер по адресу: [http://localhost:8080](http://localhost:8080)

*Учетные данные по умолчанию:*
- **Логин**: admin
- **Пароль**: admin

---

## 🛠 Установка (Production)

Для надежной работы сервер следует запускать как системную службу.

### Windows
**1. Настройка брандмауэра:**
```powershell
New-NetFirewallRule -DisplayName "REA Stream Engine" -Direction Inbound -LocalPort 8080 -Protocol TCP -Action Allow
```

**2. Автозагрузка (NSSM):**
1. Скачайте [NSSM](http://nssm.cc/).
2. В CMD (от администратора): `nssm install REAStreamEngine`
3. Укажите `Path` (путь к `.exe`) и `Directory` (путь к рабочей папке).
4. Запустите: `nssm start REAStreamEngine`

### Linux (Systemd)
**1. Настройка файрвола (UFW):**
```bash
sudo ufw allow 8080/tcp
```

**2. Автозагрузка (Systemd):**
Создайте `/etc/systemd/system/reastream.service`:
```ini
[Unit]
Description=REA Stream Engine
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/reastream
ExecStart=/opt/reastream/REAStreamEngine-linux
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```
```bash
sudo systemctl daemon-reload
sudo systemctl enable reastream
sudo systemctl start reastream
```

---

## ⚙️ Конфигурация

Глобальные настройки сервера генерируются в `config.yaml` при первом запуске:
```yaml
server:
  port: 8080
  debug: true
  record_retention_days: 14   # Срок хранения архива
  gc_percent: 50              # Тюнинг сборщика мусора
  gc_memory_limit_mb: 2048    # Защита от OOM
auth:
  username: admin
  password: mysecretpassword
  secret: "generated_jwt_secret"
```
> **Примечание:** Настройки камер и тегов динамически управляются через веб-интерфейс и хранятся в БД `BadgerDB`.

---

## 📚 Документация

Подробные материалы по архитектуре и техническим решениям:
- 🏗 [Архитектура (ARCHITECTURE.md)](docs/ARCHITECTURE.md)
- 📝 [Журнал решений (DECISIONS.md)](docs/DECISIONS.md)
- 🖥 [Инфраструктура (INFRASTRUCTURE.MD)](docs/INFRASTRUCTURE.MD)
- 🗺 [Дорожная карта (ROADMAP.MD)](docs/ROADMAP.MD)
