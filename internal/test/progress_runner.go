package test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/jedib0t/go-pretty/v6/progress"
)

// ProgressRunner extends the basic Runner with visual progress tracking
type ProgressRunner struct {
	*Runner
	enableProgress bool
	pw             progress.Writer
}

// NewProgressRunner creates a new runner with progress bar support
func NewProgressRunner(maxConcurrency int, enableProgress bool) *ProgressRunner {
	runner := NewRunner(maxConcurrency)

	pr := &ProgressRunner{
		Runner:         runner,
		enableProgress: enableProgress,
	}

	if enableProgress {
		pr.setupProgressWriter()
	}

	return pr
}

func (pr *ProgressRunner) setupProgressWriter() {
	pr.pw = progress.NewWriter()
	pr.pw.SetAutoStop(false)
	pr.pw.SetTrackerLength(50)
	pr.pw.SetMessageWidth(30)
	pr.pw.SetSortBy(progress.SortByPercentDsc)
	pr.pw.SetStyle(progress.StyleDefault)
	pr.pw.SetTrackerPosition(progress.PositionRight)
	pr.pw.SetUpdateFrequency(time.Millisecond * 100)
	pr.pw.Style().Colors = progress.StyleColorsExample
	pr.pw.Style().Options.PercentFormat = "%4.1f%%"
}

// Run executes tests with optional progress visualization
func (pr *ProgressRunner) Run(ctx context.Context, tests []TestDefinition, devices []device.Device) ([]TestResult, error) {
	totalTests := len(tests) * len(devices)
	if totalTests == 0 {
		return []TestResult{}, nil
	}

	if !pr.enableProgress {
		// Use the standard runner if progress is disabled
		return pr.Runner.Run(ctx, tests, devices)
	}

	// Create progress trackers for each device
	deviceTrackers := make(map[string]*progress.Tracker)
	var overallTracker *progress.Tracker

	// Setup overall progress tracker
	overallTracker = &progress.Tracker{
		Message: "Overall Progress",
		Total:   int64(totalTests),
		Units:   progress.UnitsDefault,
	}
	pr.pw.AppendTracker(overallTracker)

	// Setup per-device progress trackers
	for _, dev := range devices {
		tracker := &progress.Tracker{
			Message: fmt.Sprintf("Device: %s", dev.Name()),
			Total:   int64(len(tests)),
			Units:   progress.UnitsDefault,
		}
		deviceTrackers[dev.Name()] = tracker
		pr.pw.AppendTracker(tracker)
	}

	// Start the progress writer
	go pr.pw.Render()

	// Run tests with progress tracking
	results, err := pr.runWithProgress(ctx, tests, devices, deviceTrackers, overallTracker)

	// Stop progress writer
	pr.pw.Stop()

	// Show summary
	pr.showSummary(results)

	return results, err
}

func (pr *ProgressRunner) runWithProgress(
	ctx context.Context,
	tests []TestDefinition,
	devices []device.Device,
	deviceTrackers map[string]*progress.Tracker,
	overallTracker *progress.Tracker,
) ([]TestResult, error) {
	totalTests := len(tests) * len(devices)

	type testJob struct {
		test   TestDefinition
		device device.Device
	}

	jobs := make(chan testJob, totalTests)
	results := make(chan TestResult, totalTests)

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, pr.maxConcurrency)

	// Queue all jobs
	for _, test := range tests {
		for _, dev := range devices {
			jobs <- testJob{test: test, device: dev}
		}
	}
	close(jobs)

	// Start workers
	for i := 0; i < pr.maxConcurrency && i < totalTests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				select {
				case <-ctx.Done():
					// Handle cancellation
					result := TestResult{
						TestName:   job.test.Name,
						DeviceName: job.device.Name(),
						Status:     TestSkipped,
						Message:    "Test cancelled",
						Timestamp:  time.Now(),
					}
					results <- result
					overallTracker.Increment(1)
					if tracker, exists := deviceTrackers[job.device.Name()]; exists {
						tracker.Increment(1)
					}
					return
				case semaphore <- struct{}{}:
					// Run test and update progress
					result := pr.runTestWithProgress(ctx, job.test, job.device, deviceTrackers)
					results <- result
					overallTracker.Increment(1)
					<-semaphore
				}
			}
		}()
	}

	// Collect results
	go func() {
		wg.Wait()
		close(results)
	}()

	allResults := make([]TestResult, 0, totalTests)
	for result := range results {
		allResults = append(allResults, result)
	}

	pr.mu.Lock()
	pr.results = append(pr.results, allResults...)
	pr.mu.Unlock()

	return allResults, nil
}

func (pr *ProgressRunner) runTestWithProgress(
	ctx context.Context,
	testDef TestDefinition,
	dev device.Device,
	deviceTrackers map[string]*progress.Tracker,
) TestResult {

	// Update tracker message to show current test
	if tracker, exists := deviceTrackers[dev.Name()]; exists {
		tracker.UpdateMessage(fmt.Sprintf("Device: %s - %s", dev.Name(), testDef.Name))
	}

	// Run the actual test (reuse logic from base runner)
	result := pr.Runner.runTest(ctx, testDef, dev)

	// Update device tracker
	if tracker, exists := deviceTrackers[dev.Name()]; exists {
		tracker.Increment(1)

		// Update message with result
		var statusSymbol string
		switch result.Status {
		case TestSuccess:
			statusSymbol = "✓"
		case TestFailure:
			statusSymbol = "✗"
		case TestError:
			statusSymbol = "⚠"
		case TestSkipped:
			statusSymbol = "⊝"
		default:
			statusSymbol = "?"
		}

		tracker.UpdateMessage(fmt.Sprintf("Device: %s %s", dev.Name(), statusSymbol))
	}

	return result
}

func (pr *ProgressRunner) showSummary(results []TestResult) {
	if !pr.enableProgress {
		return
	}

	// Give a moment for progress bars to finish
	time.Sleep(100 * time.Millisecond)

	stats := make(map[TestStatus]int)
	for _, result := range results {
		stats[result.Status]++
	}

	fmt.Println("\n" + repeat("=", 60))
	fmt.Println("TEST EXECUTION SUMMARY")
	fmt.Println(repeat("=", 60))

	total := len(results)
	if total > 0 {
		fmt.Printf("Total Tests:    %d\n", total)
		fmt.Printf("✓ Success:     %d (%.1f%%)\n", stats[TestSuccess], float64(stats[TestSuccess])/float64(total)*100)
		fmt.Printf("✗ Failure:     %d (%.1f%%)\n", stats[TestFailure], float64(stats[TestFailure])/float64(total)*100)
		fmt.Printf("⚠ Error:       %d (%.1f%%)\n", stats[TestError], float64(stats[TestError])/float64(total)*100)
		fmt.Printf("⊝ Skipped:     %d (%.1f%%)\n", stats[TestSkipped], float64(stats[TestSkipped])/float64(total)*100)

		// Calculate success rate
		successRate := float64(stats[TestSuccess]) / float64(total) * 100
		fmt.Printf("\nSuccess Rate: %.1f%%\n", successRate)

		if successRate == 100.0 {
			fmt.Println("🎉 All tests passed!")
		} else if successRate >= 90.0 {
			fmt.Println("👍 Most tests passed")
		} else if successRate >= 70.0 {
			fmt.Println("⚠️  Some tests failed")
		} else {
			fmt.Println("❌ Many tests failed")
		}
	}
	fmt.Println(repeat("=", 60))
}

// Helper method for string repetition
func repeat(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}