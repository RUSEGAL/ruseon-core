package eventbus

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"

	"github.com/RUSEGAL/ruseon-core/v2/pkg/config"
)

// matchesTopic проверяет, подписан ли вебхук на данный топик
func matchesTopic(allowedTopics []string, topic string) bool {
	if len(allowedTopics) == 0 {
		return true // По умолчанию разрешены все, если фильтр не задан
	}
	for _, t := range allowedTopics {
		if t == "*" || t == topic {
			return true
		}
	}
	return false
}

// sendWebhook выполняет HTTP POST запрос
func sendWebhook(client *http.Client, wh config.WebhookConfig, payload []byte) error {
	req, err := http.NewRequest("POST", wh.URL, bytes.NewBuffer(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	
	// Если задан секрет, подписываем payload
	if wh.Secret != "" {
		h := hmac.New(sha256.New, []byte(wh.Secret))
		h.Write(payload)
		signature := hex.EncodeToString(h.Sum(nil))
		req.Header.Set("X-Signature", signature)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	
	// Вычитываем Body до конца (EOF), чтобы TCP-соединение могло вернуться в пул Keep-Alive
	_, _ = io.Copy(io.Discard, resp.Body)

	// Считаем успешными только 2xx коды
	// Можно расширить для обработки 4xx и 5xx по-разному
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Здесь можно возвращать кастомную ошибку
		return http.ErrServerClosed // Условная ошибка для триггера Circuit Breaker
	}

	return nil
}
