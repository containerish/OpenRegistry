package telemetry

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/containerish/OpenRegistry/config"
	"github.com/rs/zerolog/diode"
)

type (
	fluentBit struct {
		client *http.Client
		config *config.FluentBitConfig
		queue  *LogBuffer
	}
)

func NewFluentBitWriter(config *config.FluentBitConfig) diode.Writer {
	httpClient := &http.Client{
		Timeout: time.Duration(time.Second * 15),
	}

	client := &fluentBit{
		client: httpClient,
		config: config,
		queue: &LogBuffer{
			buf:   &bytes.Buffer{},
			mutex: &sync.Mutex{},
		},
	}

	go client.sendEvents()
	return diode.NewWriter(client, 1024, time.Millisecond*1500, func(missed int) {
		fmt.Printf("FluentBitWriter: Missed %d log messages", missed)
	})
}

func (fb *fluentBit) Sync() error {
	if !fb.config.Enabled || fb.queue.buf.Len() == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, fb.config.Endpoint, fb.queue.buf)
	if err != nil {
		return err
	}

	req.Header.Set("content-type", "application/json")

	// set basic auth creds if auth is enabled
	if fb.config.AuthMethod == "Basic" {
		req.SetBasicAuth(fb.config.Username, fb.config.Password)
	}

	resp, err := fb.client.Do(req)
	if err != nil {
		return err
	}

	// these are the only supported success codes in fluent-bit
	// reference: https://docs.fluentbit.io/manual/pipeline/inputs/http#configuration-parameters
	okStatus := resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusCreated ||
		resp.StatusCode == http.StatusNoContent

	if !okStatus {
		bz, _ := io.ReadAll(resp.Body)
		defer resp.Body.Close()

		return fmt.Errorf("ERR_SEND_LOG_EVENTS: Status: %d Message: %s", resp.StatusCode, bz)
	}
	defer fb.queue.Reset()

	return nil
}

func (fb *fluentBit) Write(bz []byte) (int, error) {
	fb.queue.mutex.Lock()
	defer fb.queue.mutex.Unlock()

	return fb.queue.buf.Write(bz)
}

func (fb *fluentBit) sendEvents() {
	ticker := time.NewTicker(time.Second * 3)
	for range ticker.C {
		if err := fb.Sync(); err != nil {
			fmt.Println(err.Error())
		}
		ticker.Reset(time.Second * 3)
	}
}
