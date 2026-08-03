# Gritprof Media Server

Gritprof Media Server — это высокопроизводительный, современный сервер ретрансляции (transmuxing) RTSP в HLS и записи архива, написанный на Go и React. Он обеспечивает потоковое вещание в реальном времени, запись архива в формате fMP4, таймшифт (просмотр прошлого) и продвинутую панель управления с аналитикой.

## Возможности

- **Молниеносная ретрансляция (Zero-Copy)**: Использует Go с `gortsplib` и `mediacommon` для получения RTSP потоков и их "на лету" упаковки в HLS прямо в оперативной памяти (без промежуточного перекодирования). Реализован Memory Pooling для минимизации нагрузки на сборщик мусора.
- **Умная запись (fMP4 Архивация)**: Непрерывная запись потоков напрямую в fragmented MP4 с поддержкой ротации (очистка по дням). Уникальная система бесшовной нарезки файлов без потери кадров при ротации.
- **Timeshift плеер (Плеер Архива)**: Позволяет просматривать архивные записи через веб-интерфейс, выбирая нужный временной отрезок на графическом таймлайне, с возможностью скачивания отдельных кусков в виде готового `.mp4`.
- **Современная архитектура БД (BadgerDB)**: Встроенная сверхбыстрая NoSQL база данных на основе LSM-tree хранит конфигурации камер, теги и статистику, обеспечивая миллисекундный отклик. База тонко настроена для работы с минимальным потреблением оперативной памяти.
- **Динамическое управление**: Добавление, редактирование, управление записью и удаление камер "на горячую". Система тегов, поиск и фильтрация. Приостановка камер с сохранением истории причин (биллинг/техобслуживание).
- **Система Бэкапов**: Регулярный авто-бэкап базы данных (BadgerDB Snapshots) и возможность выгрузки/загрузки конфигураций в JSON через интерфейс администратора.
- **Современный Дашборд**: Фронтенд на React 19 (Vite, TypeScript, Zustand) в потрясающем стиле Glassmorphism с поддержкой JWT-авторизации, SSE (Server-Sent Events) для логов в реальном времени и графиками.

## Архитектура

- **Backend**: Go 1.23+, фреймворк Gin (REST API + HLS).
- **Frontend**: React 19, TypeScript, Vite, Lucide Icons, Vanilla CSS (Flex/Grid).
- **Видеопротокол**: RTSP вход -> H.264/H.265 (HEVC) -> RingBuffer -> HLS Muxer (live) / fMP4 Recorder (archive).
- **Хранение данных**: 
  - `data/` — База данных BadgerDB.
  - `recordings/` — MP4-архивы камер.
  - `backups/` — Автоматические бинарные бэкапы БД.

## Требования

- [Go](https://go.dev/) 1.23 или новее
- [Node.js](https://nodejs.org/) 18+ (для сборки фронтенда)
- Любая RTSP камера, поддерживающая H.264 или H.265.

## Быстрый старт

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

### 3. Доступ к дашборду

Откройте браузер и перейдите по адресу: [http://localhost:8080](http://localhost:8080)

*Учетные данные для входа по умолчанию (хранятся в `config.yaml`):*
- **Username**: admin
- **Password**: admin

## Установка в Production (Автозагрузка)

### Windows
Для надежной работы на Windows рекомендуется запускать сервер как системную службу и открыть порт в брандмауэре.

**1. Настройка брандмауэра (в PowerShell от имени администратора):**
```powershell
New-NetFirewallRule -DisplayName "Gritprof Media Server" -Direction Inbound -LocalPort 8080 -Protocol TCP -Action Allow
```

**2. Автозагрузка (с помощью NSSM):**
1. Скачайте утилиту [NSSM](http://nssm.cc/) (Non-Sucking Service Manager).
2. В командной строке от имени администратора запустите установку:
   ```cmd
   nssm install GritprofMediaServer
   ```
3. В открывшемся GUI-окне укажите:
   - **Path**: полный путь к `GritprofMediaServer.exe`.
   - **Directory**: папка, в которой лежит `.exe` (очень важно для сохранения файлов базы `data/` и архива `recordings/`).
4. Нажмите **Install service**.
5. Запустите службу: `nssm start GritprofMediaServer`.

### Linux (Systemd)
На Linux для обеспечения бесперебойной работы используйте `systemd`.

**1. Настройка файрвола (UFW):**
```bash
sudo ufw allow 8080/tcp
```

**2. Автозагрузка (Systemd):**
Создайте файл конфигурации службы (замените `/opt/gritprof` на реальный путь к вашей папке с сервером):
```bash
sudo nano /etc/systemd/system/gritprof.service
```
Вставьте конфигурацию:
```ini
[Unit]
Description=Gritprof Media Server
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/gritprof
ExecStart=/opt/gritprof/GritprofMediaServer-linux
Restart=on-failure
RestartSec=5
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
```
Примените изменения и запустите сервер:
```bash
sudo systemctl daemon-reload
sudo systemctl enable gritprof
sudo systemctl start gritprof
```

## Конфигурация


Глобальные настройки сервера генерируются в файле `config.yaml` при первом запуске:
```yaml
server:
  port: 8080
  debug: true
  record_retention_days: 14   # Срок хранения видеоархива (дней)
  gc_percent: 50              # Тюнинг сборщика мусора для уменьшения потребления RAM
  gc_memory_limit_mb: 2048    # Жесткий лимит памяти (OOM protection)
auth:
  username: admin
  password: mysecretpassword
  secret: "generated_jwt_secret_here"
```

Настройки камер и тегов больше не хранятся в файле — они динамически управляются через интерфейс пользователя и сохраняются во внутреннюю базу `BadgerDB`.

## Документация проекта

Подробные материалы по архитектуре и техническим решениям:
- `docs/ARCHITECTURE.md` - Архитектура и дизайн системы
- `docs/DECISIONS.md` - Журнал принятых архитектурных решений (ADR)
- `docs/INFRASTRUCTURE.MD` - Анализ требований к оборудованию
- `docs/ROADMAP.MD` - Дорожная карта развития проекта
