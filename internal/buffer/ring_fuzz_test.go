package buffer

import (
	"bytes"
	"encoding/binary"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// FuzzRingBuffer тестирует базовые инварианты кольцевого буфера на произвольных последовательностях операций со строгим эталонным оракулом (Oracle).
func FuzzRingBuffer(f *testing.F) {
	// Начальный корпус
	f.Add(int(10), uint8(20), uint64(0x5555555555555555), []byte{0x00, 0x00, 0x00, 0x01, 0x67})
	f.Add(int(1), uint8(5), uint64(0), []byte{})
	f.Add(int(50), uint8(100), uint64(0xFFFFFFFFFFFFFFFF), []byte("test-nalu-payload"))
	f.Add(int(0), uint8(10), uint64(1), []byte{1, 2, 3})
	f.Add(int(256), uint8(250), uint64(0x123456789ABCDEF0), []byte{0x27, 0x42, 0x00})
	f.Add(int(5), uint8(25), uint64(0b1000000000000000000000000), []byte{0xAA, 0xBB})

	f.Fuzz(func(t *testing.T, capacity int, numWrites uint8, keyframeBitmask uint64, payload []byte) {
		if capacity > 500 {
			capacity = 500
		}
		if numWrites > 150 {
			numWrites = 150
		}

		rb := NewRingBuffer(capacity)
		defer rb.Close()

		effectiveCap := capacity
		if effectiveCap <= 0 {
			effectiveCap = 100
		}

		// 1. Проверяем установку и чтение параметров кодека
		if len(payload) >= 3 {
			vps := payload[:1]
			sps := payload[1:2]
			pps := payload[2:3]
			rb.SetParams(vps, sps, pps)

			gotV, gotS, gotP := rb.GetParams()
			if !bytes.Equal(gotV, vps) || !bytes.Equal(gotS, sps) || !bytes.Equal(gotP, pps) {
				t.Fatalf("Codec parameters mismatch: got (%v, %v, %v), want (%v, %v, %v)", gotV, gotS, gotP, vps, sps, pps)
			}
		}

		// 2. Выполняем запись кадров и сохраняем их в эталонный срез
		var written []*Frame
		for i := 0; i < int(numWrites); i++ {
			isKey := (keyframeBitmask & (1 << (uint(i) % 64))) != 0
			frame := &Frame{
				NALUs:      [][]byte{append([]byte(nil), payload...)},
				IsKeyFrame: isKey,
				Timestamp:  time.Duration(i+1) * time.Millisecond,
			}
			written = append(written, frame)
			rb.Write(frame)
		}

		// 3. Вычисляем эталонное состояние истории (Oracle)
		startWindow := 0
		if len(written) > effectiveCap {
			startWindow = len(written) - effectiveCap
		}
		historyWindow := written[startWindow:]

		// Ищем в окне истории ближайший прошлый I-Frame (от самого свежего к началу окна)
		foundIFrameIdx := -1
		for i := len(historyWindow) - 1; i >= 0; i-- {
			if historyWindow[i].IsKeyFrame {
				foundIFrameIdx = startWindow + i
				break
			}
		}

		// 4. Подключаем нового читателя после всех записей
		lateReader := rb.Subscribe()
		defer lateReader.Close()

		// Вычитываем все кадры из канала читателя
		var receivedFromHistory []*Frame
		for {
			select {
			case f, ok := <-lateReader.C:
				if !ok {
					break
				}
				receivedFromHistory = append(receivedFromHistory, f)
			default:
				goto historyDrained
			}
		}
	historyDrained:

		// 5. Строгая проверка истории против эталона (Oracle Assertions)
		if foundIFrameIdx == -1 {
			if len(receivedFromHistory) != 0 {
				t.Fatalf("Expected 0 historical frames when no I-frame in window, got %d", len(receivedFromHistory))
			}
		} else {
			expectedFrames := written[foundIFrameIdx:]
			if len(receivedFromHistory) != len(expectedFrames) {
				t.Fatalf("Historical frames count mismatch: got %d, want %d (from index %d of %d writes, cap %d)",
					len(receivedFromHistory), len(expectedFrames), foundIFrameIdx, len(written), effectiveCap)
			}

			if !receivedFromHistory[0].IsKeyFrame {
				t.Fatalf("First historical frame must be keyframe, got: %+v", receivedFromHistory[0])
			}

			for idx, rec := range receivedFromHistory {
				exp := expectedFrames[idx]
				if rec.Timestamp != exp.Timestamp {
					t.Fatalf("Frame [%d] timestamp mismatch: got %v, want %v", idx, rec.Timestamp, exp.Timestamp)
				}
				if rec.IsKeyFrame != exp.IsKeyFrame {
					t.Fatalf("Frame [%d] keyframe flag mismatch: got %v, want %v", idx, rec.IsKeyFrame, exp.IsKeyFrame)
				}
			}
		}
	})
}

// FuzzRingBuffer_Concurrency тестирует многопоточную корректность, отсутствие гонок и строгое сохранение FIFO-порядка.
func FuzzRingBuffer_Concurrency(f *testing.F) {
	f.Add(uint8(10), uint8(4), uint8(8), uint8(20))
	f.Add(uint8(1), uint8(1), uint8(1), uint8(5))
	f.Add(uint8(50), uint8(10), uint8(20), uint8(50))
	f.Add(uint8(100), uint8(2), uint8(50), uint8(10))

	f.Fuzz(func(t *testing.T, capacity uint8, writersCount uint8, readersCount uint8, framesPerWriter uint8) {
		capInt := int(capacity)
		if capInt == 0 {
			capInt = 10
		}
		numWriters := int(writersCount%16) + 1
		numReaders := int(readersCount%32) + 1
		numFrames := int(framesPerWriter%64) + 1

		rb := NewRingBuffer(capInt)

		var wg sync.WaitGroup
		stopCh := make(chan struct{})

		// Запускаем конкурентных читателей со строгой проверкой FIFO
		for r := 0; r < numReaders; r++ {
			wg.Add(1)
			go func(_ int) {
				defer wg.Done()
				reader := rb.Subscribe()
				defer reader.Close()

				lastSeqPerWriter := make(map[uint32]uint32)

				for {
					select {
					case <-stopCh:
						return
					case frame, ok := <-reader.C:
						if !ok {
							return
						}
						if frame == nil {
							t.Errorf("Received nil frame from reader channel")
							return
						}
						if len(frame.NALUs) == 0 || len(frame.NALUs[0]) < 8 {
							continue
						}

						writerID := binary.BigEndian.Uint32(frame.NALUs[0][:4])
						seq := binary.BigEndian.Uint32(frame.NALUs[0][4:8])

						if prevSeq, exists := lastSeqPerWriter[writerID]; exists {
							if seq <= prevSeq {
								t.Errorf("FIFO order violation for writer %d: received seq %d after %d", writerID, seq, prevSeq)
							}
						}
						lastSeqPerWriter[writerID] = seq
					}
				}
			}(r)
		}

		// Запускаем писателей, внедряющих (WriterID, Seq) в NALU
		var writersWg sync.WaitGroup
		for w := 0; w < numWriters; w++ {
			writersWg.Add(1)
			go func(writerID int) {
				defer writersWg.Done()
				for i := 0; i < numFrames; i++ {
					isKey := (i%5 == 0)
					payload := make([]byte, 8)
					binary.BigEndian.PutUint32(payload[:4], uint32(writerID))
					binary.BigEndian.PutUint32(payload[4:], uint32(i+1))

					f := &Frame{
						NALUs:      [][]byte{payload},
						IsKeyFrame: isKey,
						Timestamp:  time.Duration(i+1) * time.Millisecond,
					}
					rb.Write(f)
				}
			}(w)
		}

		writersWg.Wait()
		time.Sleep(5 * time.Millisecond)

		close(stopCh)
		rb.Close()
		wg.Wait()
	})
}

// FuzzRingBuffer_DropRecovery тестирует машину состояний при сбросе кадров (Drop-Tail) и восстановлении по I-Frame.
func FuzzRingBuffer_DropRecovery(f *testing.F) {
	f.Add(uint8(5), uint8(10))
	f.Add(uint8(2), uint8(4))
	f.Add(uint8(20), uint8(50))
	f.Add(uint8(50), uint8(100))

	f.Fuzz(func(t *testing.T, capacity uint8, extraWrites uint8) {
		capInt := int(capacity%30) + 2
		extra := int(extraWrites%50) + 1

		rb := NewRingBuffer(capInt)
		defer rb.Close()

		reader := rb.Subscribe()
		defer reader.Close()

		// 1. Заполняем канал читателя до упора
		for i := 0; i < capInt; i++ {
			rb.Write(&Frame{
				IsKeyFrame: (i == 0),
				Timestamp:  time.Duration(i+1) * time.Millisecond,
			})
		}

		// 2. Переполняем канал P-кадрами, гарантируя срабатывание дропа
		for i := 0; i < extra; i++ {
			rb.Write(&Frame{
				IsKeyFrame: false,
				Timestamp:  time.Duration(capInt+i+1) * time.Millisecond,
			})
		}

		// Дропы обязаны быть зафиксированы
		if atomic.LoadUint64(&reader.Drops) == 0 {
			t.Fatalf("Expected drops when overflowing channel, got 0")
		}
		if !reader.NeedsIFrame.Load() {
			t.Fatalf("Expected NeedsIFrame to be true after dropped P-Frames")
		}

		// 3. Вычитываем все старые кадры из забитого буфера
		for len(reader.C) > 0 {
			<-reader.C
		}

		// 4. Пишем серию P-кадров (не ключевых) — они обязаны игнорироваться!
		for i := 0; i < 5; i++ {
			rb.Write(&Frame{IsKeyFrame: false, Timestamp: 9999})
		}
		if len(reader.C) != 0 {
			t.Fatalf("P-Frames were delivered to reader while NeedsIFrame was true")
		}

		// 5. Пишем I-Frame (ключевой) — обязан восстановить поток
		recoveryKey := &Frame{IsKeyFrame: true, Timestamp: 10000}
		rb.Write(recoveryKey)

		select {
		case f := <-reader.C:
			if f == nil || !f.IsKeyFrame || f.Timestamp != 10000 {
				t.Fatalf("Expected recovery keyframe with ts 10000, got: %+v", f)
			}
			if reader.NeedsIFrame.Load() {
				t.Fatalf("Expected NeedsIFrame to be reset to false after receiving recovery keyframe")
			}
		default:
			t.Fatalf("Recovery keyframe was not delivered to reader")
		}
	})
}
