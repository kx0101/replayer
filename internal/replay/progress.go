package replay

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type ProgressBar struct {
	total     int
	current   int
	startTime time.Time
	mu        sync.Mutex
	width     int
}

func NewProgressBar(total int) *ProgressBar {
	pb := &ProgressBar{
		total:     total,
		current:   0,
		startTime: time.Now(),
		width:     50,
	}

	pb.render()
	return pb
}

func (pb *ProgressBar) Increment() {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.current++
	pb.render()
}

func (pb *ProgressBar) Finish() {
	pb.mu.Lock()
	defer pb.mu.Unlock()

	pb.current = pb.total
	pb.render()
	fmt.Println()
}

func (pb *ProgressBar) render() {
	if pb.total == 0 {
		return
	}

	percent := float64(pb.current) / float64(pb.total)
	filled := int(percent * float64(pb.width))

	bar := strings.Repeat("█", filled) + strings.Repeat("░", pb.width-filled)

	elapsed := time.Since(pb.startTime)
	var eta time.Duration
	if pb.current > 0 {
		rate := float64(pb.current) / elapsed.Seconds()
		remaining := pb.total - pb.current

		eta = time.Duration(float64(remaining)/rate) * time.Second
	}

	fmt.Printf("\r[%s] %d/%d (%.1f%%) | Elapsed: %s | ETA: %s  ",
		bar,
		pb.current,
		pb.total,
		percent*100,
		formatDuration(elapsed),
		formatDuration(eta),
	)
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "0s"
	}

	d = d.Round(time.Second)
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second

	if h > 0 {
		return fmt.Sprintf("%dh%dm%ds", h, m, s)
	}

	if m > 0 {
		return fmt.Sprintf("%dm%ds", m, s)
	}

	return fmt.Sprintf("%ds", s)
}
