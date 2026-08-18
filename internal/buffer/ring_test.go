package buffer

import (
	"bytes"
	"context"
	"sync"
	"testing"
	"time"
)

func TestRingBuffer_WriteAndRead(t *testing.T) {
	rb := NewRingBuffer(5)
	defer rb.Close()

	f1 := &Frame{IsKeyFrame: true, Timestamp: 1}
	f2 := &Frame{IsKeyFrame: false, Timestamp: 2}

	rb.Write(f1)
	rb.Write(f2)

	reader := rb.NewReader()
	defer reader.Close()

	read1 := reader.Read()
	if read1.Timestamp != 1 {
		t.Errorf("expected Timestamp 1, got %v", read1.Timestamp)
	}

	read2 := reader.Read()
	if read2.Timestamp != 2 {
		t.Errorf("expected Timestamp 2, got %v", read2.Timestamp)
	}
}

func TestRingBuffer_Overwrite(t *testing.T) {
	rb := NewRingBuffer(3)
	defer rb.Close()

	// Пишем 5 кадров в буфер размером 3
	for i := 1; i <= 5; i++ {
		rb.Write(&Frame{IsKeyFrame: i == 1 || i == 4, Timestamp: time.Duration(i)})
	}

	// В буфере должны остаться кадры 3, 4, 5.
	// Кадр 4 - I-frame.
	reader := rb.NewReader()
	defer reader.Close()

	// Новый читатель должен найти ближайший прошлый I-frame (кадр 4).
	firstRead := reader.Read()
	if firstRead == nil {
		t.Fatal("expected frame, got nil")
	}
	if firstRead.Timestamp != 4 {
		t.Errorf("expected Timestamp 4 (latest I-frame), got %v", firstRead.Timestamp)
	}

	secondRead := reader.Read()
	if secondRead.Timestamp != 5 {
		t.Errorf("expected Timestamp 5, got %v", secondRead.Timestamp)
	}
}

func TestRingBuffer_GetSetParams(t *testing.T) {
	rb := NewRingBuffer(10)
	defer rb.Close()

	vps := []byte{0x01}
	sps := []byte{0x02}
	pps := []byte{0x03}

	rb.SetParams(vps, sps, pps)

	gotVps, gotSps, gotPps := rb.GetParams()

	if !bytes.Equal(gotVps, vps) {
		t.Errorf("VPS mismatch")
	}
	if !bytes.Equal(gotSps, sps) {
		t.Errorf("SPS mismatch")
	}
	if !bytes.Equal(gotPps, pps) {
		t.Errorf("PPS mismatch")
	}
}

func TestRingBuffer_Close(t *testing.T) {
	rb := NewRingBuffer(5)

	reader := rb.NewReader()
	// не делаем defer reader.Close(), так как мы тестируем rb.Close()

	// Запускаем горутину, которая закроет буфер через 50 мс
	go func() {
		time.Sleep(50 * time.Millisecond)
		rb.Close()
	}()

	// Чтение должно разблокироваться и вернуть nil после Close()
	f := reader.Read()
	if f != nil {
		t.Errorf("expected nil after close, got %v", f)
	}
}

func TestRingBuffer_DropsAndRecovery(t *testing.T) {
	rb := NewRingBuffer(2)
	defer rb.Close()

	reader := rb.NewReader()
	defer reader.Close()

	// Пишем 3 кадра, канал читателя имеет размер 2.
	// Канал забьется, и 3-й кадр будет сброшен (Drop).
	rb.Write(&Frame{IsKeyFrame: true, Timestamp: 1})
	rb.Write(&Frame{IsKeyFrame: false, Timestamp: 2})
	rb.Write(&Frame{IsKeyFrame: false, Timestamp: 3}) // DROPPED

	// Читаем из канала всё что там есть (кадры 1 и 2)
	f1 := reader.Read()
	if f1.Timestamp != 1 {
		t.Errorf("expected 1, got %v", f1.Timestamp)
	}
	f2 := reader.Read()
	if f2.Timestamp != 2 {
		t.Errorf("expected 2, got %v", f2.Timestamp)
	}

	// Теперь читатель требует I-Frame (NeedsIFrame == true)
	// Пишем P-Frame (не I-Frame). Он должен быть проигнорирован.
	rb.Write(&Frame{IsKeyFrame: false, Timestamp: 4})

	// Пишем I-Frame (ключевой). Он должен быть доставлен.
	rb.Write(&Frame{IsKeyFrame: true, Timestamp: 5})

	// Читаем из канала. Там должен быть кадр 5, так как кадр 4 был проигнорирован.
	f5 := reader.Read()
	if f5.Timestamp != 5 {
		t.Errorf("expected 5 after recovery, got %v", f5.Timestamp)
	}
}

func TestReader_ReadContext(t *testing.T) {
	rb := NewRingBuffer(5)
	defer rb.Close()

	reader := rb.Subscribe()
	defer reader.Close()

	// 1. Test Cancellation
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(20 * time.Millisecond)
		cancel()
	}()

	f, err := reader.ReadContext(ctx)
	if f != nil || err != context.Canceled {
		t.Errorf("expected context.Canceled, got f=%v, err=%v", f, err)
	}

	// 2. Test Timeout
	ctxTimeout, cancelTimeout := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancelTimeout()

	f, err = reader.ReadContext(ctxTimeout)
	if f != nil || err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got f=%v, err=%v", f, err)
	}

	// 3. Test Success with Frame
	ctxValid := context.Background()
	rb.Write(&Frame{IsKeyFrame: true, Timestamp: 100})

	f, err = reader.ReadContext(ctxValid)
	if err != nil || f == nil || f.Timestamp != 100 {
		t.Errorf("expected frame with timestamp 100, got f=%v, err=%v", f, err)
	}
}

func TestRingBuffer_LongGOP_RequiresIFrame(t *testing.T) {
	rb := NewRingBuffer(5)
	defer rb.Close()

	// 1. Пишем 1 I-кадр и 6 P-кадров (I-кадр вымывается из буфера емкостью 5)
	rb.Write(&Frame{IsKeyFrame: true, Timestamp: 1})
	for i := 2; i <= 7; i++ {
		rb.Write(&Frame{IsKeyFrame: false, Timestamp: time.Duration(i)})
	}

	// 2. Новый подписчик подключается к потоку, в истории которого нет I-кадра (Long GOP > capacity)
	reader := rb.Subscribe()
	defer reader.Close()

	if !reader.NeedsIFrame.Load() {
		t.Errorf("expected reader to require I-frame when historical GOP exceeds buffer capacity")
	}

	// 3. Пишем P-кадр -> читатель должен его пропустить
	rb.Write(&Frame{IsKeyFrame: false, Timestamp: 8})
	select {
	case f := <-reader.C:
		t.Fatalf("expected reader to ignore delta frame before I-frame, got frame: %v", f)
	default:
		// OK
	}

	// 4. Пишем I-кадр -> читатель должен его получить
	rb.Write(&Frame{IsKeyFrame: true, Timestamp: 9})
	select {
	case f := <-reader.C:
		if f.Timestamp != 9 || !f.IsKeyFrame {
			t.Errorf("expected keyframe 9, got %v", f)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatalf("expected reader to receive keyframe 9")
	}
}

func TestRingBuffer_ReplayLiveBoundary_NoGapsOrDups(t *testing.T) {
	rb := NewRingBuffer(100)
	defer rb.Close()

	// 1. Предзаполняем буфер: I-кадр на 10, P-кадры 11..20
	for i := 1; i <= 20; i++ {
		rb.Write(&Frame{
			IsKeyFrame: i%10 == 0,
			Timestamp:  time.Duration(i),
			NALUs:      [][]byte{{byte(i)}},
		})
	}

	var reader *Reader
	var received []*Frame
	var readerWg sync.WaitGroup

	readerWg.Add(1)
	go func() {
		defer readerWg.Done()
		// Подключаемся прямо во время параллельной записи
		reader = rb.Subscribe()
		defer reader.Close()

		for {
			select {
			case f, ok := <-reader.C:
				if !ok {
					return
				}
				received = append(received, f)
				if f.Timestamp == 100 {
					return
				}
			case <-time.After(500 * time.Millisecond):
				return
			}
		}
	}()

	// Параллельно пишем кадры 21..100
	for i := 21; i <= 100; i++ {
		rb.Write(&Frame{
			IsKeyFrame: i%10 == 0,
			Timestamp:  time.Duration(i),
			NALUs:      [][]byte{{byte(i)}},
		})
		time.Sleep(100 * time.Microsecond)
	}

	readerWg.Wait()

	if len(received) == 0 {
		t.Fatalf("expected to receive frames, got 0")
	}

	// Первый кадр обязан быть ключевым
	if !received[0].IsKeyFrame {
		t.Errorf("expected first replayed frame to be keyframe, got ts=%v isKey=%v", received[0].Timestamp, received[0].IsKeyFrame)
	}

	// Проверяем строгую монотонность: 0 дубликатов, 0 пропусков
	for i := 1; i < len(received); i++ {
		prev := received[i-1].Timestamp
		curr := received[i].Timestamp
		if curr != prev+1 {
			t.Fatalf("sequence discontinuity at index %d: received ts=%v immediately after prev ts=%v (gap or duplicate)", i, curr, prev)
		}
	}
}

func TestRingBuffer_SlowSubscriber_NoGlobalStall(t *testing.T) {
	rb := NewRingBuffer(5)
	defer rb.Close()

	// 1. Медленный подписчик - канал емкостью 5, не вычитываем из него ничего
	slowSub := rb.Subscribe()
	defer slowSub.Close()

	// 2. Быстрый подписчик - читаем непрерывно в фоне
	fastSub := rb.Subscribe()
	defer fastSub.Close()

	fastReceived := 0
	fastDone := make(chan struct{})
	go func() {
		for {
			select {
			case _, ok := <-fastSub.C:
				if !ok {
					close(fastDone)
					return
				}
				fastReceived++
			case <-fastDone:
				return
			}
		}
	}()

	// 3. Быстро пишем 1000 кадров и измеряем время выполнения
	start := time.Now()
	for i := 1; i <= 1000; i++ {
		rb.Write(&Frame{
			IsKeyFrame: i%10 == 0,
			Timestamp:  time.Duration(i),
			NALUs:      [][]byte{{byte(i)}},
		})
	}
	elapsed := time.Since(start)

	close(fastDone)

	// Запись 1000 кадров в non-blocking кольцо должна занимать не более 50 мс
	if elapsed > 100*time.Millisecond {
		t.Errorf("Write() stalled due to slow subscriber, elapsed: %v", elapsed)
	}

	// Медленный подписчик должен зафиксировать сбросы (Drops > 0)
	if slowSub.Drops == 0 {
		t.Errorf("expected slow subscriber to have drops > 0, got %d", slowSub.Drops)
	}
	if !slowSub.NeedsIFrame.Load() {
		t.Errorf("expected slow subscriber to require I-Frame after drops")
	}
}

func TestRingBuffer_DefensiveCapacityGuard(t *testing.T) {
	rb0 := NewRingBuffer(0)
	if rb0.capacity != 100 || len(rb0.frames) != 100 {
		t.Errorf("expected capacity 100 for NewRingBuffer(0), got %d", rb0.capacity)
	}

	rbNeg := NewRingBuffer(-15)
	if rbNeg.capacity != 100 || len(rbNeg.frames) != 100 {
		t.Errorf("expected capacity 100 for NewRingBuffer(-15), got %d", rbNeg.capacity)
	}
}


