package test

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gavmckee/go-anta/internal/device"
	"github.com/gavmckee/go-anta/internal/logger"
)

type Runner struct {
	maxConcurrency int
	results        []TestResult
	mu             sync.Mutex
	registry       *Registry
}

func NewRunner(maxConcurrency int) *Runner {
	if maxConcurrency <= 0 {
		maxConcurrency = 10
	}

	return &Runner{
		maxConcurrency: maxConcurrency,
		results:        make([]TestResult, 0),
		registry:       GetRegistry(),
	}
}

func (r *Runner) Run(ctx context.Context, tests []TestDefinition, devices []device.Device) ([]TestResult, error) {
	totalTests := len(tests) * len(devices)
	if totalTests == 0 {
		return []TestResult{}, nil
	}

	logger.Infof("Starting test run: %d tests on %d devices (%d total executions)", len(tests), len(devices), totalTests)

	type testJob struct {
		test   TestDefinition
		device device.Device
	}

	jobs := make(chan testJob, totalTests)
	results := make(chan TestResult, totalTests)
	
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, r.maxConcurrency)

	for _, test := range tests {
		for _, dev := range devices {
			logger.Debugf("Queuing test %s for device %s", test.Name, dev.Name())
			jobs <- testJob{test: test, device: dev}
		}
	}
	close(jobs)

	for i := 0; i < r.maxConcurrency && i < totalTests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobs {
				select {
				case <-ctx.Done():
					logger.Warnf("Test %s cancelled for device %s", job.test.Name, job.device.Name())
					results <- TestResult{
						TestName:   job.test.Name,
						DeviceName: job.device.Name(),
						Status:     TestSkipped,
						Message:    "Test cancelled",
						Timestamp:  time.Now(),
					}
					return
				case semaphore <- struct{}{}:
					result := r.runTest(ctx, job.test, job.device)
					results <- result
					<-semaphore
				}
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	allResults := make([]TestResult, 0, totalTests)
	for result := range results {
		allResults = append(allResults, result)
	}

	r.mu.Lock()
	r.results = append(r.results, allResults...)
	r.mu.Unlock()

	return allResults, nil
}

func (r *Runner) runTest(ctx context.Context, testDef TestDefinition, dev device.Device) TestResult {
	start := time.Now()
	logger.Debugf("Running test %s on device %s", testDef.Name, dev.Name())
	
	if !dev.IsEstablished() {
		logger.Errorf("Device %s not connected for test %s", dev.Name(), testDef.Name)
		return TestResult{
			TestName:   testDef.Name,
			DeviceName: dev.Name(),
			Status:     TestError,
			Message:    "Device not connected",
			Duration:   time.Since(start),
			Timestamp:  time.Now(),
			Categories: testDef.Categories,
		}
	}

	testImpl, err := r.registry.GetTestWithInputs(testDef.Module, testDef.Name, testDef.Inputs)
	if err != nil {
		logger.Errorf("Test %s not found: %v", testDef.Name, err)
		return TestResult{
			TestName:   testDef.Name,
			DeviceName: dev.Name(),
			Status:     TestError,
			Message:    fmt.Sprintf("Test not found: %v", err),
			Duration:   time.Since(start),
			Timestamp:  time.Now(),
			Categories: testDef.Categories,
		}
	}

	if err := testImpl.ValidateInput(testDef.Inputs); err != nil {
		return TestResult{
			TestName:   testDef.Name,
			DeviceName: dev.Name(),
			Status:     TestError,
			Message:    fmt.Sprintf("Invalid input: %v", err),
			Duration:   time.Since(start),
			Timestamp:  time.Now(),
			Categories: testDef.Categories,
		}
	}

	logger.Debugf("Executing test %s on device %s", testDef.Name, dev.Name())
	result, err := testImpl.Execute(ctx, dev)
	if err != nil {
		logger.Errorf("Test %s failed on device %s: %v", testDef.Name, dev.Name(), err)
		return TestResult{
			TestName:   testDef.Name,
			DeviceName: dev.Name(),
			Status:     TestError,
			Message:    fmt.Sprintf("Test execution failed: %v", err),
			Duration:   time.Since(start),
			Timestamp:  time.Now(),
			Categories: testDef.Categories,
		}
	}

	result.Duration = time.Since(start)
	result.Timestamp = time.Now()
	
	if result.Status == TestSuccess {
		logger.Infof("Test %s passed on device %s (%.2fs)", testDef.Name, dev.Name(), result.Duration.Seconds())
	} else if result.Status == TestFailure {
		logger.Warnf("Test %s failed on device %s: %s (%.2fs)", testDef.Name, dev.Name(), result.Message, result.Duration.Seconds())
	}
	
	if result.Categories == nil {
		result.Categories = testDef.Categories
	}

	return *result
}

func (r *Runner) GetResults() []TestResult {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	results := make([]TestResult, len(r.results))
	copy(results, r.results)
	return results
}

func (r *Runner) ClearResults() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results = make([]TestResult, 0)
}

func (r *Runner) FilterResults(status TestStatus) []TestResult {
	r.mu.Lock()
	defer r.mu.Unlock()

	filtered := make([]TestResult, 0)
	for _, result := range r.results {
		if result.Status == status {
			filtered = append(filtered, result)
		}
	}
	return filtered
}

func (r *Runner) GetStatistics() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()

	stats := map[string]int{
		"total":   len(r.results),
		"success": 0,
		"failure": 0,
		"error":   0,
		"skipped": 0,
	}

	for _, result := range r.results {
		switch result.Status {
		case TestSuccess:
			stats["success"]++
		case TestFailure:
			stats["failure"]++
		case TestError:
			stats["error"]++
		case TestSkipped:
			stats["skipped"]++
		}
	}

	return stats
}