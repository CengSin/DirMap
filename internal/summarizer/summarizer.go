package summarizer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/cengsin/system-agent-rag/internal/config"
	"github.com/cengsin/system-agent-rag/internal/model"
)

type Summarizer struct {
	client anthropic.Client
	cfg    *config.Config
}

func New(cfg *config.Config) *Summarizer {
	var opts []option.RequestOption
	if cfg.LLM.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(cfg.LLM.BaseURL))
	}
	if cfg.LLM.APIKey != "" {
		opts = append(opts, option.WithAPIKey(cfg.LLM.APIKey))
	}
	client := anthropic.NewClient(opts...)
	return &Summarizer{
		client: client,
		cfg:    cfg,
	}
}

func (s *Summarizer) SummarizeBatch(ctx context.Context, files []model.FileInfo) ([]model.FileInfo, error) {
	if len(files) == 0 {
		return files, nil
	}

	batchSize := s.cfg.LLM.MaxBatchSize
	concurrency := s.cfg.LLM.MaxConcurrency

	// Split into batches
	var batches [][]model.FileInfo
	for i := 0; i < len(files); i += batchSize {
		end := i + batchSize
		if end > len(files) {
			end = len(files)
		}
		batches = append(batches, files[i:end])
	}

	// Single batch — no need for goroutine overhead
	if len(batches) == 1 {
		results, err := s.callWithRetry(ctx, batches[0], 3)
		if err != nil {
			log.Printf("summarizer: batch failed: %v", err)
			return batches[0], err
		}
		return results, nil
	}

	// Concurrent worker pool
	results := make([][]model.FileInfo, len(batches))
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	for i, batch := range batches {
		wg.Add(1)
		go func(idx int, b []model.FileInfo) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				mu.Lock()
				results[idx] = b
				mu.Unlock()
				return
			}

			res, err := s.callWithRetry(ctx, b, 3)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				log.Printf("summarizer: batch %d failed: %v", idx, err)
				results[idx] = b // fallback without descriptions
				if firstErr == nil {
					firstErr = err
				}
			} else {
				results[idx] = res
			}
		}(i, batch)
	}

	wg.Wait()

	// Flatten in order
	var allResults []model.FileInfo
	for _, r := range results {
		allResults = append(allResults, r...)
	}
	return allResults, firstErr
}

func (s *Summarizer) callWithRetry(ctx context.Context, files []model.FileInfo, maxRetries int) ([]model.FileInfo, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			log.Printf("summarizer: retry %d/%d after %v", attempt, maxRetries, backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return files, ctx.Err()
			}
		}

		results, err := s.call(ctx, files)
		if err == nil {
			return results, nil
		}

		// Don't retry on context cancellation
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return files, err
		}

		lastErr = err
	}

	return files, fmt.Errorf("after %d retries: %w", maxRetries, lastErr)
}

func (s *Summarizer) call(ctx context.Context, files []model.FileInfo) ([]model.FileInfo, error) {
	userPrompt := buildUserPrompt(files)

	params := anthropic.MessageNewParams{
		Model:       s.cfg.LLM.Model,
		MaxTokens:   int64(s.cfg.LLM.MaxTokens),
		Temperature: param.NewOpt(s.cfg.LLM.Temperature),
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	}

	stream := s.client.Messages.NewStreaming(ctx, params)

	var responseText strings.Builder
	for stream.Next() {
		event := stream.Current()
		if event.Type == "content_block_delta" && event.Delta.Type == "text_delta" {
			responseText.WriteString(event.Delta.Text)
		}
	}
	if err := stream.Err(); err != nil {
		return files, fmt.Errorf("api stream: %w", err)
	}

	if responseText.Len() == 0 {
		return files, fmt.Errorf("no text in API response")
	}

	descriptions := parseDescriptions(responseText.String())

	for i := range files {
		if desc, ok := descriptions[files[i].Path]; ok {
			files[i].Description = desc
		} else {
			files[i].Description = "(no description)"
		}
	}

	return files, nil
}

func parseDescriptions(text string) map[string]string {
	result := make(map[string]string)
	lines := strings.Split(strings.TrimSpace(text), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		idx := strings.Index(line, "|")
		if idx < 0 {
			continue
		}

		path := strings.TrimSpace(line[:idx])
		desc := strings.TrimSpace(line[idx+1:])

		if path != "" && desc != "" {
			result[path] = desc
		}
	}

	return result
}
