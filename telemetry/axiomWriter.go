package telemetry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/axiomhq/axiom-go/axiom"
	"github.com/axiomhq/axiom-go/axiom/ingest"
	"github.com/rs/zerolog/diode"
)

var ErrMissingDatasetName = errors.New("missing dataset name")

type Option func(*AxiomWriter) error

func SetClient(client *axiom.Client) Option {
	return func(ws *AxiomWriter) error {
		ws.client = client
		return nil
	}
}

func SetClientOptions(options ...axiom.Option) Option {
	return func(ws *AxiomWriter) error {
		ws.clientOptions = options
		return nil
	}
}

func SetDataset(datasetName string) Option {
	return func(ws *AxiomWriter) error {
		ws.datasetName = datasetName
		return nil
	}
}

func SetIngestOptions(opts ...ingest.Option) Option {
	return func(ws *AxiomWriter) error {
		ws.ingestOptions = opts
		return nil
	}
}

func SetBatchSize(size int) Option {
	return func(ws *AxiomWriter) error {
		ws.batchSize = size
		return nil
	}
}

type AxiomWriter struct {
	client        *axiom.Client
	queue         *LogBuffer
	mutex         *sync.Mutex
	datasetName   string
	clientOptions []axiom.Option
	ingestOptions []ingest.Option
	batchSize     int
	counter       int
}

func NewAxiomWriter(options ...Option) (diode.Writer, error) {
	ws := &AxiomWriter{
		queue: &LogBuffer{
			buf:   &bytes.Buffer{},
			mutex: &sync.Mutex{},
			size:  0,
		},
		counter: 0,
		mutex:   &sync.Mutex{},
	}

	for _, option := range options {
		if option == nil {
			continue
		} else if err := option(ws); err != nil {
			return diode.Writer{}, err
		}
	}

	if ws.client == nil {
		var err error
		if ws.client, err = axiom.NewClient(ws.clientOptions...); err != nil {
			return diode.Writer{}, err
		}
	}

	if ws.batchSize == 0 || ws.batchSize > MaxBatchSize {
		ws.batchSize = MaxBatchSize
	}

	go ws.sendEvents()
	return diode.NewWriter(ws, 1024, 0, func(missed int) {
		fmt.Printf("AxiomWriter: Missed %d log messages", missed)
	}), nil
}

func (ws *AxiomWriter) Write(p []byte) (n int, err error) {
	ws.queue.mutex.Lock()
	defer ws.queue.mutex.Unlock()

	return ws.queue.buf.Write(p)
}

func (ws *AxiomWriter) Sync() error {
	if ws.datasetName == "" {
		return ErrMissingDatasetName
	}

	if ws.queue.buf.Len() == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	defer cancel()

	r, err := axiom.ZstdEncoder()(ws.queue.buf)
	if err != nil {
		return err
	}

	res, err := ws.client.Ingest(ctx, ws.datasetName, r, axiom.NDJSON, axiom.Zstd, ws.ingestOptions...)
	if err != nil {
		return err
	} else if res.Failed > 0 {
		// Best effort on notifying the user about the ingest failure.
		return fmt.Errorf("event at %s failed to ingest: %s", res.Failures[0].Timestamp, res.Failures[0].Error)
	}
	defer ws.queue.Reset()

	return nil
}

func (ws *AxiomWriter) sendEvents() {
	ticker := time.NewTicker(time.Second * 3)
	for range ticker.C {
		if err := ws.Sync(); err != nil {
			fmt.Println(err.Error())
		}
		ticker.Reset(time.Second * 3)
	}
}
