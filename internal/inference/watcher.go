package inference

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"hbtop/internal/theme"
)

type State struct {
	Online             bool
	Model              string
	IsProcessing       bool
	PromptTPS          float64
	PredictedTPS       float64
	RequestsProcessing float64
	RequestsDeferred   float64
	LastError          string

	PromptTokensTotal    float64
	PromptSecondsTotal   float64
	PredictedTokensTotal float64
	PredictedSecondsTotal float64

	LastRunPromptTPS    float64
	LastRunPredictedTPS float64
	LivePromptTPS       float64
	LivePredictedTPS    float64
	SlotPromptProcessed int
	SlotDecoded         int
	PromptAvg           float64
	PredictedAvg        float64
}

type EventKind int

const (
	EventNone EventKind = iota
	EventModelLoading
	EventModelReady
	EventInferenceStarted
	EventInferenceFinished
	EventOffline
)

type Event struct {
	Kind         EventKind
	Model        string
	PromptTPS    float64
	PredictedTPS float64
}

type Watcher struct {
	baseURL string
	client  *http.Client
	last    State
	events  chan Event

	runPromptMax    float64
	runPredictedMax float64
	lastPromptNZ    float64
	lastPredictedNZ float64

	lastPT, lastPS, lastTT, lastTS float64
	runPT, runPS, runTT, runTS    float64
	basePT, basePS, baseTT, baseTS float64
	baseValid                      bool

	lastRunPrompt    float64
	lastRunPredicted float64
	failCount        int

	lastSlotPromptProcessed int
	lastSlotDecoded         int
	lastSlotAt              time.Time
}

func NewWatcher(cfg theme.InferenceConfig) *Watcher {
	base := strings.TrimRight(cfg.URL, "/")
	base = strings.TrimSuffix(base, "/metrics")
	timeout := time.Duration(cfg.TimeoutMS) * time.Millisecond
	if timeout <= 0 {
		timeout = 500 * time.Millisecond
	}
	return &Watcher{
		baseURL: base,
		client:  &http.Client{Timeout: timeout},
		events:  make(chan Event, 16),
	}
}

func (w *Watcher) Events() <-chan Event { return w.events }

func (w *Watcher) Poll() State {
	st := State{
		LastRunPromptTPS:    w.lastRunPrompt,
		LastRunPredictedTPS: w.lastRunPredicted,
	}
	now := time.Now()

	if _, err := w.get("/health"); err != nil {
		w.failCount++
		st.LastError = err.Error()
		if w.failCount < 3 {
			prev := w.last
			prev.LastError = err.Error()
			return prev
		}
		if w.last.Online {
			w.emit(Event{Kind: EventOffline})
		}
		w.last = st
		return st
	}
	w.failCount = 0
	st.Online = true

	var props struct {
		ModelAlias string `json:"model_alias"`
	}
	if body, err := w.get("/props"); err == nil {
		_ = json.Unmarshal(body, &props)
		st.Model = props.ModelAlias
	}

	var slots []struct {
		IsProcessing           bool `json:"is_processing"`
		NPromptTokensProcessed int  `json:"n_prompt_tokens_processed"`
		NextToken              []struct {
			NDecoded int `json:"n_decoded"`
		} `json:"next_token"`
	}
	if body, err := w.get("/slots"); err == nil {
		if json.Unmarshal(body, &slots) == nil && len(slots) > 0 {
			st.IsProcessing = slots[0].IsProcessing
			st.SlotPromptProcessed = slots[0].NPromptTokensProcessed
			if len(slots[0].NextToken) > 0 {
				st.SlotDecoded = slots[0].NextToken[0].NDecoded
			}
		}
	}

	if body, err := w.get("/metrics"); err == nil {
		parseMetrics(string(body), &st)
	}

	if st.PromptTPS > 0 {
		w.lastPromptNZ = st.PromptTPS
	}
	if st.PredictedTPS > 0 {
		w.lastPredictedNZ = st.PredictedTPS
	}
	if st.PromptSecondsTotal > 0 {
		st.PromptAvg = st.PromptTokensTotal / st.PromptSecondsTotal
	}
	if st.PredictedSecondsTotal > 0 {
		st.PredictedAvg = st.PredictedTokensTotal / st.PredictedSecondsTotal
	}

	if st.IsProcessing {
		if !w.last.IsProcessing {
			w.lastSlotPromptProcessed = st.SlotPromptProcessed
			w.lastSlotDecoded = st.SlotDecoded
			w.lastSlotAt = now
		} else if !w.lastSlotAt.IsZero() {
			dt := now.Sub(w.lastSlotAt).Seconds()
			if dt > 0 {
				dp := st.SlotPromptProcessed - w.lastSlotPromptProcessed
				dd := st.SlotDecoded - w.lastSlotDecoded
				if dp >= 0 {
					st.LivePromptTPS = float64(dp) / dt
				}
				if dd >= 0 {
					st.LivePredictedTPS = float64(dd) / dt
				}
			}
			w.lastSlotPromptProcessed = st.SlotPromptProcessed
			w.lastSlotDecoded = st.SlotDecoded
			w.lastSlotAt = now
		}
		// Fall back to the Prometheus counters if slot progress is unavailable.
		if st.LivePredictedTPS == 0 && w.lastTT > 0 && st.PredictedTokensTotal >= w.lastTT && st.PredictedSecondsTotal > w.lastTS {
			st.LivePredictedTPS = (st.PredictedTokensTotal - w.lastTT) / (st.PredictedSecondsTotal - w.lastTS)
		}
		if st.LivePromptTPS == 0 && w.lastPT > 0 && st.PromptTokensTotal >= w.lastPT && st.PromptSecondsTotal > w.lastPS {
			st.LivePromptTPS = (st.PromptTokensTotal - w.lastPT) / (st.PromptSecondsTotal - w.lastPS)
		}
	} else {
		w.lastSlotAt = time.Time{}
	}

	if !st.IsProcessing && !w.last.IsProcessing {
		w.basePT = st.PromptTokensTotal
		w.basePS = st.PromptSecondsTotal
		w.baseTT = st.PredictedTokensTotal
		w.baseTS = st.PredictedSecondsTotal
		w.baseValid = true
	}

	if st.IsProcessing {
		if !w.last.IsProcessing {
			w.runPT, w.runPS, w.runTT, w.runTS = 0, 0, 0, 0
		}
		if st.PromptTokensTotal >= w.lastPT {
			w.runPT += st.PromptTokensTotal - w.lastPT
		}
		if st.PromptSecondsTotal >= w.lastPS {
			w.runPS += st.PromptSecondsTotal - w.lastPS
		}
		if st.PredictedTokensTotal >= w.lastTT {
			w.runTT += st.PredictedTokensTotal - w.lastTT
		}
		if st.PredictedSecondsTotal >= w.lastTS {
			w.runTS += st.PredictedSecondsTotal - w.lastTS
		}
		if w.lastPromptNZ > w.runPromptMax {
			w.runPromptMax = w.lastPromptNZ
		}
		if w.lastPredictedNZ > w.runPredictedMax {
			w.runPredictedMax = w.lastPredictedNZ
		}
	}
	w.lastPT = st.PromptTokensTotal
	w.lastPS = st.PromptSecondsTotal
	w.lastTT = st.PredictedTokensTotal
	w.lastTS = st.PredictedSecondsTotal

	if !w.last.Online {
		w.emit(Event{Kind: EventModelReady, Model: st.Model})
	}
	if !w.last.IsProcessing && st.IsProcessing {
		w.runPromptMax = 0
		w.runPredictedMax = 0
		w.emit(Event{Kind: EventInferenceStarted, Model: st.Model})
	}
	if w.last.IsProcessing && !st.IsProcessing {
		pp := 0.0
		if w.baseValid && st.PromptTokensTotal >= w.basePT && st.PromptSecondsTotal >= w.basePS {
			dPT := st.PromptTokensTotal - w.basePT
			dPS := st.PromptSecondsTotal - w.basePS
			if dPS > 0 {
				pp = dPT / dPS
			}
		}
		if pp == 0 {
			if w.runPS > 0 {
				pp = w.runPT / w.runPS
			} else if w.runPromptMax > 0 {
				pp = w.runPromptMax
			} else {
				pp = w.lastPromptNZ
			}
		}
		tg := 0.0
		if w.baseValid && st.PredictedTokensTotal >= w.baseTT && st.PredictedSecondsTotal >= w.baseTS {
			dTT := st.PredictedTokensTotal - w.baseTT
			dTS := st.PredictedSecondsTotal - w.baseTS
			if dTS > 0 {
				tg = dTT / dTS
			}
		}
		if tg == 0 {
			if w.runTS > 0 {
				tg = w.runTT / w.runTS
			} else if w.runPredictedMax > 0 {
				tg = w.runPredictedMax
			} else {
				tg = w.lastPredictedNZ
			}
		}
		w.lastRunPrompt = pp
		w.lastRunPredicted = tg
		st.LastRunPromptTPS = pp
		st.LastRunPredictedTPS = tg
		w.emit(Event{Kind: EventInferenceFinished, Model: st.Model, PromptTPS: pp, PredictedTPS: tg})
		w.basePT = st.PromptTokensTotal
		w.basePS = st.PromptSecondsTotal
		w.baseTT = st.PredictedTokensTotal
		w.baseTS = st.PredictedSecondsTotal
		w.baseValid = true
	}
	if w.last.Model != "" && st.Model != "" && w.last.Model != st.Model {
		w.emit(Event{Kind: EventModelReady, Model: st.Model})
	}

	w.last = st
	return st
}

func (w *Watcher) get(path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), w.client.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (w *Watcher) emit(e Event) {
	select {
	case w.events <- e:
	default:
	}
}

func parseMetrics(text string, st *State) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		val, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}
		switch fields[0] {
		case "llamacpp:predicted_tokens_seconds":
			st.PredictedTPS = val
		case "llamacpp:prompt_tokens_seconds":
			st.PromptTPS = val
		case "llamacpp:prompt_tokens_total":
			st.PromptTokensTotal = val
		case "llamacpp:prompt_seconds_total":
			st.PromptSecondsTotal = val
		case "llamacpp:tokens_predicted_total":
			st.PredictedTokensTotal = val
		case "llamacpp:tokens_predicted_seconds_total":
			st.PredictedSecondsTotal = val
		case "llamacpp:requests_processing":
			st.RequestsProcessing = val
		case "llamacpp:requests_deferred":
			st.RequestsDeferred = val
		}
	}
}
