import subprocess
import time
import argparse
import sys
import os

def spawn_ffmpeg(video_file, target_url):
    """Запускает процесс ffmpeg для одного потока"""
    # -re : чтение в реальном времени
    # -stream_loop -1 : зациклить видео бесконечно
    # -c copy : без перекодирования (чтобы не грузить CPU генератора)
    cmd = [
        "ffmpeg", "-hide_banner", "-loglevel", "error",
        "-re", "-stream_loop", "-1",
        "-i", video_file,
        "-c", "copy",
        "-f", "rtsp",
        target_url
    ]
    return subprocess.Popen(cmd, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

def main():
    parser = argparse.ArgumentParser(description="RUSEON Core: RTSP Load Generator")
    parser.add_argument("--video", type=str, required=True, help="Путь к тестовому MP4/MKV файлу")
    parser.add_argument("--host", type=str, default="127.0.0.1:8554", help="RTSP Хост и порт сервера")
    parser.add_argument("--streams", type=int, default=10, help="Количество генерируемых потоков (камер)")
    
    args = parser.parse_args()

    if not os.path.exists(args.video):
        print(f"❌ Файл {args.video} не найден!")
        sys.exit(1)

    print(f"🚀 Запуск нагрузочного тестирования RUSEON Core")
    print(f"🎥 Файл: {args.video}")
    print(f"📡 Целевой сервер: rtsp://{args.host}/<имя_потока>")
    print(f"🔥 Количество потоков: {args.streams}")
    print("-" * 40)

    processes = []
    try:
        for i in range(1, args.streams + 1):
            stream_name = f"cam_{i}"
            target_url = f"rtsp://{args.host}/{stream_name}"
            
            p = spawn_ffmpeg(args.video, target_url)
            processes.append(p)
            print(f"✅ Запущен поток: {stream_name} (PID: {p.pid})")
            
            # Небольшая пауза, чтобы не обрушить сеть шквалом одновременных подключений
            time.sleep(0.1)

        print("-" * 40)
        print(f"🎉 Все {args.streams} потоков успешно запущены!")
        print("Нажмите Ctrl+C для остановки всех трансляций...")
        
        while True:
            time.sleep(1)

    except KeyboardInterrupt:
        print("\n🛑 Остановка нагрузочного тестирования...")
    finally:
        for p in processes:
            p.terminate()
        print("👋 Все процессы ffmpeg завершены.")

if __name__ == "__main__":
    main()
