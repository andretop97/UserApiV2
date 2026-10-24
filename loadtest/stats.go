package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"slices"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"text/tabwriter"
	"time"
)

// recorder acumula, por etapa, as latências e as falhas agrupadas por motivo.
type recorder struct {
	mu    sync.Mutex
	order []string
	steps map[string]*stepStats

	total  atomic.Int64
	failed atomic.Int64
}

type stepStats struct {
	ok       int
	failures map[string]int
	durs     []time.Duration
}

func newRecorder() *recorder {
	return &recorder{steps: map[string]*stepStats{}}
}

func (r *recorder) step(name string) *stepStats {
	s, ok := r.steps[name]
	if !ok {
		s = &stepStats{failures: map[string]int{}}
		r.steps[name] = s
		r.order = append(r.order, name)
	}
	return s
}

// add registra a resposta de uma etapa e diz se ela veio com o status esperado.
func (r *recorder) add(name string, want int, res result) bool {
	// Ctrl+C no meio de uma requisição não é falha da API.
	if errors.Is(res.err, context.Canceled) {
		return false
	}
	ok := res.err == nil && res.status == want

	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.step(name)
	if res.err == nil {
		s.durs = append(s.durs, res.dur)
	}
	r.total.Add(1)
	if ok {
		s.ok++
		return true
	}
	r.failed.Add(1)
	s.failures[failureReason(res)]++
	return false
}

// addDuration registra uma medida que não é uma requisição (ex.: o fluxo inteiro).
func (r *recorder) addDuration(name string, d time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.step(name)
	s.ok++
	s.durs = append(s.durs, d)
}

func failureReason(res result) string {
	if res.err != nil {
		var netErr net.Error
		switch {
		case errors.Is(res.err, context.DeadlineExceeded), errors.As(res.err, &netErr) && netErr.Timeout():
			return "timeout do cliente"
		case strings.Contains(res.err.Error(), "connection refused"), strings.Contains(res.err.Error(), "actively refused"):
			return "conexão recusada"
		case strings.Contains(res.err.Error(), "EOF"), strings.Contains(res.err.Error(), "reset"), strings.Contains(res.err.Error(), "forcibly closed"):
			return "conexão derrubada pelo servidor"
		default:
			return "erro: " + res.err.Error()
		}
	}
	if code := errorCode(res.body); code != "" {
		return fmt.Sprintf("%d %s", res.status, code)
	}
	return fmt.Sprintf("%d", res.status)
}

// progress imprime a vazão e as falhas a cada 5 s enquanto o estágio roda.
func (r *recorder) progress(start time.Time) (stop func()) {
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		var lastTotal, lastFailed int64
		last := start
		for {
			select {
			case <-done:
				return
			case now := <-ticker.C:
				total, failed := r.total.Load(), r.failed.Load()
				rate := float64(total-lastTotal) / now.Sub(last).Seconds()
				fmt.Printf("  [%3.0fs] %6.1f req/s, %d falhas no intervalo\n", now.Sub(start).Seconds(), rate, failed-lastFailed)
				lastTotal, lastFailed, last = total, failed, now
			}
		}
	}()
	return func() { close(done); wg.Wait() }
}

func (r *recorder) report(w io.Writer, elapsed time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "etapa\ttotal\tok\tfalhas\tok/s\tp50\tp95\tp99\tmax\t")
	for _, name := range r.order {
		s := r.steps[name]
		failed := 0
		for _, n := range s.failures {
			failed += n
		}
		sorted := slices.Clone(s.durs)
		slices.Sort(sorted)
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%.1f\t%s\t%s\t%s\t%s\t\n",
			name, s.ok+failed, s.ok, failed, float64(s.ok)/elapsed.Seconds(),
			fmtDur(percentile(sorted, .50)), fmtDur(percentile(sorted, .95)),
			fmtDur(percentile(sorted, .99)), fmtDur(percentile(sorted, 1)))
	}
	tw.Flush()

	for _, name := range r.order {
		s := r.steps[name]
		if len(s.failures) == 0 {
			continue
		}
		reasons := make([]string, 0, len(s.failures))
		for reason := range s.failures {
			reasons = append(reasons, reason)
		}
		sort.Slice(reasons, func(i, j int) bool { return s.failures[reasons[i]] > s.failures[reasons[j]] })
		fmt.Fprintf(w, "  falhas em %s:\n", name)
		for _, reason := range reasons {
			fmt.Fprintf(w, "    %6d × %s\n", s.failures[reason], reason)
		}
	}
	fmt.Fprintf(w, "  duração real do estágio: %s\n", elapsed.Round(time.Millisecond))
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	i := int(math.Ceil(p*float64(len(sorted)))) - 1
	return sorted[max(i, 0)]
}

func fmtDur(d time.Duration) string {
	switch {
	case d == 0:
		return "-"
	case d < time.Millisecond:
		return d.Round(time.Microsecond).String()
	case d < time.Second:
		return d.Round(100 * time.Microsecond).String()
	default:
		return d.Round(10 * time.Millisecond).String()
	}
}
