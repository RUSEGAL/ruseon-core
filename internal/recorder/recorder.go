// Package recorder provides continuous fragmented MP4 (fMP4 / ISO BMFF) archive recording,
// power-loss crash recovery, page cache dropping on Linux, and automated retention cleanup.
package recorder

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bluenviron/mediacommon/pkg/formats/fmp4"
	"github.com/rs/zerolog/log"

	"github.com/RUSEGAL/ruseon-core/internal/buffer"
	"github.com/RUSEGAL/ruseon-core/internal/rtsp"
	"github.com/RUSEGAL/ruseon-core/pkg/metrics"
	"github.com/RUSEGAL/ruseon-core/pkg/registry"
)

// Recorder continuously consumes video frames from a RingBuffer and writes them into fragmented MP4 files (fMP4).
//
// Concurrency & Durability:
//   - Flushes each GOP into independent fMP4 `moof` + `mdat` fragments.
//   - Drops Linux page cache after writing each GOP to avoid starving OS memory buffers.
//   - Guarantees playable recordings across sudden power failures or server reboots.
type Recorder struct {
	streamID   string
	ringBuffer *buffer.RingBuffer
	ctx        context.Context
	cancel     context.CancelFunc
	recordDir  string
	onDegraded func(bool)
	wg         sync.WaitGroup
}

// NewRecorder creates and launches a new fMP4 video archive recorder for the given stream.
func NewRecorder(streamID string, rb *buffer.RingBuffer, recordDir string, onDegraded func(bool)) *Recorder {
	ctx, cancel := context.WithCancel(context.Background())
	r := &Recorder{
		streamID:   streamID,
		ringBuffer: rb,
		ctx:        ctx,
		cancel:     cancel,
		recordDir:  recordDir,
		onDegraded: onDegraded,
	}

	if err := registry.CurrentBlobStore.MkdirAll(recordDir); err != nil {
		log.Warn().Err(err).Str("recordDir", recordDir).Msg("Failed to create record root directory")
	}

	r.wg.Add(1)
	go r.run()
	return r
}

func (r *Recorder) run() {
	defer r.wg.Done()
	
	var file registry.WriteSeekCloser
	var seq uint32
	var partsWritten uint32
	var partSamples = make([]*fmp4.PartSample, 0, 150) // Preallocate for ~5 sec GOP

	var partStartBaseTime uint64
	var lastPts int64
	var recordStartTime time.Time
	var pendingSample *fmp4.PartSample
	var initialPts int64 = -1
	var currentFilename string
	var lastGoodDuration uint32 = 90000 / 25

	partTrack := &fmp4.PartTrack{ID: 1}
	part := &fmp4.Part{Tracks: []*fmp4.PartTrack{partTrack}}

	finalize := func(isError bool) {
		if file != nil {
			_ = file.Close()
			switch {
			case isError:
				metrics.ArchiveErrorsTotal.WithLabelValues(r.streamID).Inc()
				if registry.CurrentEventBus != nil {
					registry.CurrentEventBus.Publish("recording_failed", r.streamID, map[string]string{
						"error": "recording write failure",
						"file":  currentFilename,
					})
				}
				corruptedFilename := filepath.Join(filepath.Dir(currentFilename), strings.TrimSuffix(filepath.Base(currentFilename), ".mp4")+".corrupted")
				if err := registry.CurrentBlobStore.Rename(currentFilename, corruptedFilename); err != nil {
					log.Warn().Err(err).Str("from", currentFilename).Str("to", corruptedFilename).Msg("Failed to rename corrupted recording file")
				}
			case partsWritten > 0:
				recordEndTime := time.Now()
				finalFilename := filepath.Join(filepath.Dir(currentFilename), fmt.Sprintf("%s_to_%s.mp4", recordStartTime.Format("2006-01-02_15-04-05"), recordEndTime.Format("15-04-05")))
				if err := registry.CurrentBlobStore.Rename(currentFilename, finalFilename); err != nil {
					log.Error().Err(err).Str("from", currentFilename).Str("to", finalFilename).Msg("Failed to rename completed recording file")
					metrics.ArchiveErrorsTotal.WithLabelValues(r.streamID).Inc()
					if registry.CurrentEventBus != nil {
						registry.CurrentEventBus.Publish("recording_failed", r.streamID, map[string]string{
							"error": fmt.Sprintf("failed to rename recording: %v", err),
							"file":  currentFilename,
						})
					}
				} else {
					metrics.ArchiveSegmentsWrittenTotal.WithLabelValues(r.streamID).Inc()
					if registry.CurrentEventBus != nil {
						registry.CurrentEventBus.Publish("archive_segment_ready", r.streamID, map[string]string{"file": finalFilename})
					}
				}
			default:
				// Пустой файл (0 частей записано) - удаляем, исключая загрязнение архива
				if err := registry.CurrentBlobStore.Delete(currentFilename); err != nil {
					log.Warn().Err(err).Str("file", currentFilename).Msg("Failed to delete empty recording file")
				}
			}
			file = nil
		}
	}

	finalizeSuccess := func() { finalize(false) }
	finalizeFailure := func() { finalize(true) }

	defer func() {
		if err := recover(); err != nil {
			log.Error().Interface("panic", err).Str("streamID", r.streamID).Msg("Recovered from panic in Recorder.run")
			if r.onDegraded != nil {
				r.onDegraded(true)
			}
			finalizeFailure()
		}
	}()

	reader := r.ringBuffer.NewReader()
	defer reader.Close()

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
		var frame *buffer.Frame
		select {
		case <-r.ctx.Done():
			goto ExitLoop
		case f, ok := <-reader.C:
			if !ok {
				goto ExitLoop
			}
			frame = f
		}

		rawPts := rtsp.DurationTo90k(frame.Timestamp)

		// Ротация файла каждый час. Проверяем перед обработкой I-кадра.
		if file != nil && frame.IsKeyFrame && time.Since(recordStartTime) > 1*time.Hour {
			if pendingSample != nil {
				pendingSample.Duration = lastGoodDuration
				partSamples = append(partSamples, pendingSample)
			}
			if len(partSamples) > 0 {
				part.SequenceNumber = seq
				partTrack.BaseTime = partStartBaseTime
				partTrack.Samples = partSamples
				if err := part.Marshal(file); err != nil {
					if r.onDegraded != nil {
						r.onDegraded(true)
					}
					finalizeFailure()
					pendingSample = nil
					partSamples = partSamples[:0]
					initialPts = -1
					continue
				}
				partsWritten++
			}

			// Считаем объем записанного
			var size uint32
			for _, s := range partSamples {
				size += uint32(len(s.Payload)) // #nosec G115 -- payload length fits within uint32
			}
			metrics.DiskWriteBytesTotal.WithLabelValues(r.streamID).Add(float64(size))

			if dropper, ok := file.(registry.CacheDropper); ok {
				_ = dropper.DropCache()
			}
			
			log.Info().Str("stream", r.streamID).Msg("Rotating record file")
			finalizeSuccess()
			pendingSample = nil
			partSamples = partSamples[:0]
			initialPts = -1
			partsWritten = 0
		}

		if file == nil {
			// Начинаем запись только с ключевого кадра
			if !frame.IsKeyFrame {
				continue
			}

			// Создаем подпапку по имени потока
			streamDir := filepath.Join(r.recordDir, r.streamID)
			if err := registry.CurrentBlobStore.MkdirAll(streamDir); err != nil {
				log.Warn().Err(err).Str("streamDir", streamDir).Msg("Failed to create stream record directory")
			}

			recordStartTime = time.Now()
			currentFilename = filepath.Join(streamDir, fmt.Sprintf("%s_ongoing.mp4", recordStartTime.Format("2006-01-02_15-04-05")))
			var err error
			file, err = registry.CurrentBlobStore.Create(currentFilename)
			if err != nil {
				metrics.ArchiveErrorsTotal.WithLabelValues(r.streamID).Inc()
				log.Error().Err(err).Msg("Failed to create record file")
				if registry.CurrentEventBus != nil {
					registry.CurrentEventBus.Publish("recording_failed", r.streamID, map[string]string{"error": err.Error(), "file": currentFilename})
				}
				if r.onDegraded != nil {
					r.onDegraded(true)
				}
				time.Sleep(1 * time.Second)
				continue
			}

			if r.onDegraded != nil {
				r.onDegraded(false)
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
				if r.onDegraded != nil {
					r.onDegraded(true)
				}
				finalizeFailure()
				continue
			}

			initialPts = rawPts
			partStartBaseTime = 0
			lastPts = 0
			seq = 1
			partsWritten = 0
			pendingSample = nil
			partSamples = partSamples[:0]
		}

		currentPts := rawPts - initialPts

		// Устанавливаем Duration для предыдущего сэмпла
		if pendingSample != nil {
			dur := currentPts - lastPts
			if dur <= 0 || dur > 90000*5 { // Защита от регрессий, джиттера B-кадров и аномальных скачков
				pendingSample.Duration = lastGoodDuration
			} else {
				lastGoodDuration = uint32(dur) // #nosec G115 -- dur is bounded <= 90000*5
				pendingSample.Duration = lastGoodDuration
			}
			partSamples = append(partSamples, pendingSample)
		}

		sample, err := fmp4.NewPartSampleH26x(0, frame.IsKeyFrame, frame.NALUs)
		if err == nil {
			pendingSample = sample
		}
		lastPts = currentPts

		// Формируем и сбрасываем Part в файл на каждом I-кадре (новый GOP)
		if frame.IsKeyFrame && len(partSamples) > 0 {
			part.SequenceNumber = seq
			partTrack.BaseTime = partStartBaseTime
			partTrack.Samples = partSamples

			if err := part.Marshal(file); err != nil {
				log.Error().Err(err).Msg("Failed to write fMP4 part")
				if r.onDegraded != nil {
					r.onDegraded(true)
				}
				finalizeFailure()
				pendingSample = nil
				partSamples = partSamples[:0]
				initialPts = -1
				partsWritten = 0
				continue
			}

			partsWritten++
			var size uint32
			for _, s := range partSamples {
				size += uint32(len(s.Payload)) // #nosec G115 -- payload length fits within uint32
			}
			metrics.DiskWriteBytesTotal.WithLabelValues(r.streamID).Add(float64(size))

			if dropper, ok := file.(registry.CacheDropper); ok {
				_ = dropper.DropCache()
			}

			partSamples = partSamples[:0]
			seq++
			partStartBaseTime = uint64(currentPts) // #nosec G115 -- currentPts is non-negative
		}
	}
ExitLoop:

	if file != nil {
		if pendingSample != nil {
			pendingSample.Duration = lastGoodDuration
			partSamples = append(partSamples, pendingSample)
		}
		if len(partSamples) > 0 {
			part.SequenceNumber = seq
			partTrack.BaseTime = partStartBaseTime
			partTrack.Samples = partSamples
			if err := part.Marshal(file); err == nil {
				partsWritten++
			}
		}
		finalizeSuccess()
	}
}

// Stop halts archive recording, flushes the trailing GOP samples to disk, and waits for background workers to finish.
func (r *Recorder) Stop() {
	r.cancel()
	r.wg.Wait()
}
