// this file defines ring buffer which is used for capturing audio clips
package myaudio

import (
	"errors"
	"sync"
	"time"
)

// AudioBuffer represents a circular buffer for storing PCM audio data, with timestamp tracking.
type AudioBuffer struct {
	data           []byte
	writeIndex     int
	sampleRate     int
	bytesPerSample int
	bufferSize     int
	bufferDuration time.Duration
	startTime      time.Time
	initialized    bool
	lock           sync.Mutex
}

// NewAudioBuffer initializes a new AudioBuffer with timestamp tracking
func NewAudioBuffer(durationSeconds int, sampleRate, bytesPerSample int) *AudioBuffer {
	bufferSize := durationSeconds * sampleRate * bytesPerSample
	alignedBufferSize := ((bufferSize + 2047) / 2048) * 2048 // Round up to the nearest multiple of 2048
	ab := &AudioBuffer{
		data:           make([]byte, alignedBufferSize),
		sampleRate:     sampleRate,
		bytesPerSample: bytesPerSample,
		bufferSize:     alignedBufferSize,
		bufferDuration: time.Second * time.Duration(durationSeconds),
		initialized:    false,
	}

	return ab
}

// Write adds PCM audio data to the buffer, ensuring thread safety and accurate timekeeping.
func (ab *AudioBuffer) Write(data []byte) {
	// Lock the buffer to prevent concurrent writes or reads from interfering with the update process.
	ab.lock.Lock()
	defer ab.lock.Unlock()

	if !ab.initialized {
		// Initialize the buffer's start time based on the current time.
		ab.startTime = time.Now()
		ab.initialized = true
	}

	// Store the current write index to determine if we've wrapped around the buffer.
	prevWriteIndex := ab.writeIndex

	// Copy the incoming data into the buffer starting at the current write index.
	bytesWritten := copy(ab.data[ab.writeIndex:], data)

	// Update the write index, wrapping around the buffer if necessary.
	ab.writeIndex = (ab.writeIndex + bytesWritten) % ab.bufferSize

	// Determine if the write operation has overwritten old data.
	if ab.writeIndex <= prevWriteIndex {
		// If old data has been overwritten, adjust startTime to maintain accurate timekeeping.
		ab.startTime = time.Now().Add(-ab.bufferDuration)
		//log.Printf("Buffer has wrapped around, adjusting start time to %v", ab.startTime)
	}
}

// ReadSegment extracts a segment of audio data based on precise start and end times, handling wraparounds.
// It waits until the current time is past the requested end time.
func (ab *AudioBuffer) ReadSegment(requestedStartTime time.Time, duration int) ([]byte, error) {
	requestedEndTime := requestedStartTime.Add(time.Duration(duration) * time.Second)

	for {
		ab.lock.Lock()

		startOffset := requestedStartTime.Sub(ab.startTime)
		endOffset := requestedEndTime.Sub(ab.startTime)

		startIndex := int(startOffset.Seconds()) * ab.sampleRate * ab.bytesPerSample
		endIndex := int(endOffset.Seconds()) * ab.sampleRate * ab.bytesPerSample

		startIndex = startIndex % ab.bufferSize
		endIndex = endIndex % ab.bufferSize

		if startOffset < 0 {
			if ab.writeIndex == 0 || ab.writeIndex+int(startOffset.Seconds())*ab.sampleRate*ab.bytesPerSample > ab.bufferSize {
				ab.lock.Unlock()
				return nil, errors.New("requested start time is outside the buffer's current timeframe")
			}
			startIndex = (ab.bufferSize + startIndex) % ab.bufferSize
		}

		if endOffset < 0 || endOffset <= startOffset {
			ab.lock.Unlock()
			return nil, errors.New("requested times are outside the buffer's current timeframe")
		}

		// Wait until the current time is past the requested end time
		if time.Now().After(requestedEndTime) {
			var segment []byte
			if startIndex < endIndex {

				//log.Printf("Reading segment from %d to %d", startIndex, endIndex)
				segmentSize := endIndex - startIndex
				segment = make([]byte, segmentSize)
				copy(segment, ab.data[startIndex:endIndex])
			} else {
				//log.Printf("Buffer has wrapped, reading segment from %d to %d", startIndex, endIndex)
				segmentSize := (ab.bufferSize - startIndex) + endIndex
				segment = make([]byte, segmentSize)
				firstPartSize := ab.bufferSize - startIndex
				copy(segment[:firstPartSize], ab.data[startIndex:])
				copy(segment[firstPartSize:], ab.data[:endIndex])
			}
			ab.lock.Unlock()
			return segment, nil
		}

		//log.Printf("Buffer is not filled yet, waiting for data to be available")
		ab.lock.Unlock()
		time.Sleep(1 * time.Second) // Sleep briefly to avoid busy waiting
	}
}
