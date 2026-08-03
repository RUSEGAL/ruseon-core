package buffer

import (
	"bytes"
	"testing"
	"time"
)

func TestRingBuffer_WriteAndRead(t *testing.T) {
	rb := NewRingBuffer(5)
	
	f1 := &Frame{IsKeyFrame: true, Timestamp: 1}
	f2 := &Frame{IsKeyFrame: false, Timestamp: 2}
	
	rb.Write(f1)
	rb.Write(f2)
	
	reader := rb.NewReader()
	
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
	
	// Пишем 5 кадров в буфер размером 3
	for i := 1; i <= 5; i++ {
		rb.Write(&Frame{IsKeyFrame: i == 1 || i == 4, Timestamp: time.Duration(i)})
	}
	
	// В буфере должны остаться кадры 3, 4, 5. 
	// Кадр 4 - I-frame.
	reader := rb.NewReader()
	
	// Новый читатель должен найти ближайший прошлый I-frame.
	// Последний I-frame был под номером 4.
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

func TestRingBuffer_Overrun(t *testing.T) {
	rb := NewRingBuffer(3)
	
	rb.Write(&Frame{IsKeyFrame: true, Timestamp: 1})
	reader := rb.NewReader()
	
	// Пишем еще 5 кадров, перезаписывая старые. Читатель "отстал".
	for i := 2; i <= 6; i++ {
		rb.Write(&Frame{IsKeyFrame: i == 5, Timestamp: time.Duration(i)})
	}
	
	// Читатель должен обнаружить overrun (буфер переполнен новыми данными)
	// и прыгнуть на ближайший доступный I-frame (кадр 5)
	f := reader.Read()
	if f == nil {
		t.Fatal("expected frame, got nil")
	}
	if f.Timestamp != 5 {
		t.Errorf("expected Timestamp 5 after overrun, got %v", f.Timestamp)
	}
}
