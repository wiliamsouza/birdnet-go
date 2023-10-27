package myaudio

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/smallnest/ringbuffer"
	"github.com/tphakala/go-birdnet/pkg/birdnet"
)

const (
	chunkSize      = 144000
	pollingTimeout = time.Millisecond * 1000
)

var (
	ringBuffer  *ringbuffer.RingBuffer
	bufferMutex sync.Mutex
	quitChannel = make(chan struct{})
)

func writeToBuffer(data []byte) {
	bufferMutex.Lock()
	defer bufferMutex.Unlock()
	ringBuffer.Write(data)
}

func readFromBuffer() []byte {
	bufferMutex.Lock()
	defer bufferMutex.Unlock()

	bytesWritten := ringBuffer.Length() - ringBuffer.Free()
	if bytesWritten < chunkSize {
		//fmt.Println("Not enough data in buffer")
		return nil
	}

	data := make([]byte, chunkSize)
	_, err := ringBuffer.Read(data)
	if err != nil {
		return nil
	}
	return data
}

func BufferMonitor() {
	for {
		select {
		case <-quitChannel:
			return
		default:
			data := readFromBuffer()
			fmt.Println("data length: ", len(data))
			if data != nil {
				processData(data)
			} else {
				time.Sleep(pollingTimeout)
			}
		}
	}
}

func processData(data []byte) {
	//log.Printf("Processing data: %b\n", data)
	// get time stamp to calculate processing time
	ts := time.Now()

	// temporary assignments
	var bitDepth int = 16
	var sensitivity float64 = 1.25

	sampleData, err := ConvertToFloat32(data, bitDepth)
	if err != nil {
		log.Fatalf("Error converting to float32: %v", err)
	}
	results, err := birdnet.Predict(sampleData, sensitivity)
	if err != nil {
		log.Fatalf("Error predicting: %v", err)
	}
	te := time.Now()
	fmt.Printf("processing time %v", te.Sub(ts))

	fmt.Println(results)
	//fmt.Println("Processed audio data of length:", len(data))
}
