package recorder

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/bluenviron/mediacommon/pkg/formats/fmp4"
	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/REA-Stream-Engine/internal/buffer"
)

// Recorder читает кадры из RingBuffer и пишет их в fMP4 архив.
type Recorder struct {
	streamID   string
	ringBuffer *buffer.RingBuffer
	ctx        context.Context
	cancel     context.CancelFunc
	recordDir  string
}

// NewRecorder создает и запускает архиватор для потока.
func NewRecorder(streamID string, rb *buffer.RingBuffer, recordDir string) *Recorder {
	ctx, cancel := context.WithCancel(context.Background())
	r := &Recorder{
		streamID:   streamID,
		ringBuffer: rb,
		ctx:        ctx,
		cancel:     cancel,
		recordDir:  recordDir,
	}

	_ = os.MkdirAll(recordDir, 0755)

	go r.run()
	return r
}

func (r *Recorder) run() {
	var file *os.File
	var seq uint32
	var partSamples = make([]*fmp4.PartSample, 0, 150) // Preallocate for ~5 sec GOP

	var partStartBaseTime uint64
	var lastPts int64
	var recordStartTime time.Time
	var pendingSample *fmp4.PartSample
	var initialPts int64 = -1
	var currentFilename string

	closeAndRename := func() {
		if file != nil {
			file.Close()
			recordEndTime := time.Now()
			finalFilename := filepath.Join(filepath.Dir(currentFilename), fmt.Sprintf("%s_to_%s.mp4", recordStartTime.Format("2006-01-02_15-04-05"), recordEndTime.Format("15-04-05")))
			_ = os.Rename(currentFilename, finalFilename)
			file = nil
		}
	}

	defer func() {
		if err := recover(); err != nil {
			log.Error().Interface("panic", err).Str("streamID", r.streamID).Msg("Recovered from panic in Recorder.run")
			closeAndRename()
		}
	}()

	reader := r.ringBuffer.NewReader()

	// Ждем получения параметров кодека
	_, sps, pps := r.ringBuffer.GetParams()
	for sps == nil || pps == nil {
		if r.ctx.Err() != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
		_, sps, pps = r.ringBuffer.GetParams()
	}

	for {
		if r.ctx.Err() != nil {
			break
		}

		frame := reader.Read()
		if frame == nil {
			break
		}
		rawPts := int64(frame.Timestamp * 90000 / time.Second)

		// Ротация файла каждый час. Проверяем перед обработкой I-кадра.
		if file != nil && frame.IsKeyFrame && time.Since(recordStartTime) > 1*time.Hour {
			if pendingSample != nil {
				pendingSample.Duration = 90000 / 25
				partSamples = append(partSamples, pendingSample)
			}
			if len(partSamples) > 0 {
				part := &fmp4.Part{
					SequenceNumber: seq,
					Tracks: []*fmp4.PartTrack{{
						ID:       1,
						BaseTime: partStartBaseTime,
						Samples:  partSamples,
					}},
				}
				_ = part.Marshal(file)
			}
			
			log.Info().Str("stream", r.streamID).Msg("Rotating record file")
			closeAndRename()
			pendingSample = nil
			partSamples = partSamples[:0]
			initialPts = -1
		}

		if file == nil {
			// Начинаем запись только с ключевого кадра
			if !frame.IsKeyFrame {
				continue
			}

			// Создаем подпапку по имени потока
			streamDir := filepath.Join(r.recordDir, r.streamID)
			_ = os.MkdirAll(streamDir, 0755)

			recordStartTime = time.Now()
			currentFilename = filepath.Join(streamDir, fmt.Sprintf("%s_ongoing.mp4", recordStartTime.Format("2006-01-02_15-04-05")))
			var err error
			file, err = os.Create(currentFilename)
			if err != nil {
				log.Error().Err(err).Msg("Failed to create record file")
				time.Sleep(1 * time.Second)
				continue
			}

			log.Info().Str("file", currentFilename).Msg("Started recording fMP4")

			vps, sps, pps := r.ringBuffer.GetParams()

			var codec fmp4.Codec
			if vps != nil {
				codec = &fmp4.CodecH265{VPS: vps, SPS: sps, PPS: pps}
			} else {
				codec = &fmp4.CodecH264{SPS: sps, PPS: pps}
			}

			init := &fmp4.Init{
				Tracks: []*fmp4.InitTrack{{
					ID:         1,
					TimeScale:  90000,
					Codec:      codec,
				}},
			}
			if err := init.Marshal(file); err != nil {
				log.Error().Err(err).Msg("Failed to write fMP4 init")
				closeAndRename()
				continue
			}

			initialPts = rawPts
			partStartBaseTime = 0
			lastPts = 0
			seq = 1
			pendingSample = nil
			partSamples = partSamples[:0]
		}

		currentPts := rawPts - initialPts

		// Устанавливаем Duration для предыдущего сэмпла
		if pendingSample != nil {
			pendingSample.Duration = uint32(currentPts - lastPts) //nolint:gosec
			partSamples = append(partSamples, pendingSample)
		}

		sample, err := fmp4.NewPartSampleH26x(0, frame.IsKeyFrame, frame.NALUs)
		if err == nil {
			pendingSample = sample
		}
		lastPts = currentPts

		// Формируем и сбрасываем Part в файл на каждом I-кадре (новый GOP)
		if frame.IsKeyFrame && len(partSamples) > 0 {
			part := &fmp4.Part{
				SequenceNumber: seq,
				Tracks: []*fmp4.PartTrack{{
					ID:       1,
					BaseTime: partStartBaseTime,
					Samples:  partSamples,
				}},
			}

			if err := part.Marshal(file); err != nil {
				log.Error().Err(err).Msg("Failed to write fMP4 part")
				closeAndRename()
				pendingSample = nil
				partSamples = partSamples[:0]
				initialPts = -1
				continue
			}

			partSamples = partSamples[:0]
			seq++
			partStartBaseTime = uint64(currentPts) //nolint:gosec
		}
	}

	if file != nil {
		// Дописываем последний сэмпл при остановке
		if pendingSample != nil {
			pendingSample.Duration = 90000 / 25
			partSamples = append(partSamples, pendingSample)
		}
		if len(partSamples) > 0 {
			part := &fmp4.Part{
				SequenceNumber: seq,
				Tracks: []*fmp4.PartTrack{{
					ID:       1,
					BaseTime: partStartBaseTime,
					Samples:  partSamples,
				}},
			}
			_ = part.Marshal(file)
		}
		closeAndRename()
	}
}

// Stop останавливает архиватор.
func (r *Recorder) Stop() {
	r.cancel()
}
