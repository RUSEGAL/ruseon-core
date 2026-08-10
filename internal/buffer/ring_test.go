package buffer

import (
	"bytes"
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


