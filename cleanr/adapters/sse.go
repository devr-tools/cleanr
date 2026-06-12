package adapters

import (
	"bufio"
	"io"
	"strings"
	"time"

	"github.com/devr-tools/cleanr/cleanr/core"
)

type sseEvent struct {
	Name string
	Data string
}

func parseSSEStream(r io.Reader) ([]sseEvent, core.StreamMetrics, error) {
	start := time.Now()
	reader := bufio.NewReader(r)
	events := make([]sseEvent, 0, 16)
	metrics := core.StreamMetrics{}

	var (
		eventName string
		dataLines []string
	)

	dispatch := func() {
		if eventName == "" && len(dataLines) == 0 {
			return
		}
		event := sseEvent{
			Name: strings.TrimSpace(eventName),
			Data: strings.Join(dataLines, "\n"),
		}
		events = append(events, event)
		if strings.TrimSpace(event.Data) != "" {
			metrics.ChunkCount++
			if metrics.TTFTMS == 0 {
				metrics.TTFTMS = time.Since(start).Milliseconds()
			}
		}
		eventName = ""
		dataLines = nil
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			metrics.DurationMS = time.Since(start).Milliseconds()
			metrics.ErrorCount++
			return events, metrics, err
		}

		line = strings.TrimRight(line, "\r\n")
		switch {
		case line == "":
			dispatch()
		case strings.HasPrefix(line, ":"):
		case strings.HasPrefix(line, "event:"):
			eventName = strings.TrimSpace(line[len("event:"):])
		case strings.HasPrefix(line, "data:"):
			dataLines = append(dataLines, strings.TrimSpace(line[len("data:"):]))
		}

		if err == io.EOF {
			dispatch()
			metrics.DurationMS = time.Since(start).Milliseconds()
			return events, metrics, nil
		}
	}
}

func markStreamParseRecovery(metrics *core.StreamMetrics) {
	if metrics.ErrorCount > 0 {
		metrics.Recovered = true
	}
}
