package telemetry

import (
	"bytes"
	"sync"
)

type LogBuffer struct {
	buf   *bytes.Buffer
	mutex *sync.Mutex
	size  int
}

func (q *LogBuffer) Reset() {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	q.buf.Reset()
	q.size = 0
}

const MaxBatchSize int = 10
