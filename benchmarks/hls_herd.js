import http from 'k6/http';
import { check, sleep } from 'k6';

export const options = {
    stages: [
        { duration: '10s', target: 1000 },
        { duration: '50s', target: 1000 },
        { duration: '10s', target: 0 },
    ],
    thresholds: {
        http_req_duration: ['p(95)<500'],
        http_req_failed: ['rate<0.01'],
    },
};

export default function () {
    const baseUrl = 'http://127.0.0.1:8080/stream/hls/cam_1';
    const fakeIp = `10.0.${Math.floor(__VU / 255)}.${__VU % 255}`;
    
    const params = {
        headers: {
            'User-Agent': `k6-load-test-vu-${__VU}`,
            'X-Forwarded-For': fakeIp,
            'X-Real-IP': fakeIp,
        },
    };

    // 1. Скачиваем плейлист
    const res = http.get(`${baseUrl}/index.m3u8`, params);
    
    check(res, {
        'playlist status 200': (r) => r.status === 200,
    });

    if (res.status === 200) {
        // Отладочный вывод только для первого юзера при первой итерации
        if (__VU === 1 && __ITER === 0) {
            console.log("=== MANIFEST PREVIEW ===\n" + res.body + "\n========================");
        }

        const lines = res.body.split('\n');
        let segmentName = '';
        for (let i = lines.length - 1; i >= 0; i--) {
            if (lines[i].trim().endsWith('.ts')) {
                segmentName = lines[i].trim();
                break;
            }
        }

        if (segmentName !== '') {
            // ВАЖНО: responseType: 'none' предотвращает попытки k6 распаковывать бинарное видео в текст, 
            // что съедает весь процессор и память тестовой машины.
            const segParams = Object.assign({}, params, { responseType: 'none' });
            const segRes = http.get(`${baseUrl}/${segmentName}`, segParams);
            
            check(segRes, {
                'segment status 200': (r) => r.status === 200,
            });
        } else if (__VU === 1 && __ITER === 0) {
            console.warn("⚠️ Плейлист пуст! Muxer не нарезает сегменты. Возможно, в исходном видео (sample.mp4) отсутствуют ключевые кадры (I-frames), либо слишком большой GOP.");
        }
    }

    sleep(2);
}
