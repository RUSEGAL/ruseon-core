import requests
import argparse
import sys

def main():
    parser = argparse.ArgumentParser(description="RUSEON Core: Camera Setup for Load Testing")
    parser.add_argument("--api", type=str, default="http://127.0.0.1:8080", help="URL сервера RUSEON Core")
    parser.add_argument("--user", type=str, default="admin", help="Логин")
    parser.add_argument("--password", type=str, default="password123", help="Пароль")
    parser.add_argument("--host", type=str, default="127.0.0.1:8554", help="RTSP Хост (например MediaMTX)")
    parser.add_argument("--streams", type=int, default=10, help="Количество добавляемых камер")
    
    args = parser.parse_args()

    # 1. Логин
    print(f"🔑 Авторизация в {args.api}...")
    login_resp = requests.post(f"{args.api}/api/login", json={
        "username": args.user,
        "password": args.password
    })
    
    if login_resp.status_code != 200:
        print(f"❌ Ошибка авторизации: {login_resp.text}")
        sys.exit(1)
        
    token = login_resp.json().get("token")
    headers = {"Authorization": f"Bearer {token}"}
    print("✅ Успешно!")

    # 2. Добавление камер
    print(f"📡 Добавление {args.streams} камер...")
    for i in range(1, args.streams + 1):
        cam_id = f"cam_{i}"
        cam_url = f"rtsp://{args.host}/{cam_id}"
        
        cam_data = {
            "id": cam_id,
            "url": cam_url,
            "record": False, # Для теста Ingest записи можно пока выключить, или включить позже
            "lazyHLS": True,
            "transport": "tcp"
        }
        
        resp = requests.post(f"{args.api}/api/cameras", json=cam_data, headers=headers)
        if resp.status_code == 200:
            print(f"  [+] Добавлена {cam_id} -> {cam_url}")
        else:
            print(f"  [-] Ошибка добавления {cam_id}: {resp.text}")

    print(f"🎉 Готово! {args.streams} камер добавлены в RUSEON Core.")

if __name__ == "__main__":
    main()
