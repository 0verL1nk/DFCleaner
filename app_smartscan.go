package main

import (
	"context"

	"dfcleaner/internal/scanner"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"
)

func (a *App) SmartScan(path string, opts scanner.ScanOptions) error {
	a.logger.Printf("SmartScan: path=%s", path)

	ctx, cancel := context.WithCancel(a.ctx)
	a.smartScanCancel = cancel

	wailsrt.EventsEmit(a.ctx, "smartscan:progress", map[string]any{
		"phase":          "scanning",
		"filesScanned":   0,
		"totalToAnalyze": 0,
		"itemsFound":     0,
	})

	s := scanner.New(ctx)
	result, err := s.Scan(path, opts)
	if err != nil {
		a.logger.Printf("SmartScan scan error: %v", err)
		wailsrt.EventsEmit(a.ctx, "smartscan:error", err.Error())
		return err
	}

	entries := result.Entries
	totalFiles := len(entries)
	if totalFiles == 0 {
		wailsrt.EventsEmit(a.ctx, "smartscan:complete", map[string]any{
			"totalScanned": 0, "totalAnalyzed": 0, "itemsFound": 0,
		})
		return nil
	}

	a.logger.Printf("SmartScan: analyzing %d entries", totalFiles)

	go a.runAnalysis(ctx, entries, totalFiles)
	return nil
}

func (a *App) CancelSmartScan() {
	if a.smartScanCancel != nil {
		a.smartScanCancel()
	}
}

func (a *App) runAnalysis(ctx context.Context, entries []scanner.FileEntry, totalFiles int) {
	const batchSize = 30
	itemsFound := 0
	analyzed := 0

	for i := 0; i < totalFiles; i += batchSize {
		if ctx.Err() != nil {
			wailsrt.EventsEmit(a.ctx, "smartscan:error", "cancelled")
			return
		}

		end := i + batchSize
		if end > totalFiles {
			end = totalFiles
		}
		batch := entries[i:end]

		results, err := a.analyzer.BatchAnalyze(ctx, batch)
		if err != nil {
			a.logger.Printf("SmartScan analyze batch error: %v", err)
			continue
		}

		var cleanable []map[string]any
		for j, r := range results {
			if r.Error != "" {
				continue
			}
			if r.RiskLevel == "safe" || r.RiskLevel == "caution" {
				e := batch[j]
				cleanable = append(cleanable, map[string]any{
					"path":      r.Path,
					"name":      e.Name,
					"size":      e.Size,
					"isDir":     e.IsDir,
					"riskLevel": r.RiskLevel,
					"reason":    r.Reason,
					"category":  r.Category,
				})
			}
		}

		if len(cleanable) > 0 {
			itemsFound += len(cleanable)
			wailsrt.EventsEmit(a.ctx, "smartscan:items", cleanable)
		}

		analyzed = end
		wailsrt.EventsEmit(a.ctx, "smartscan:progress", map[string]any{
			"phase":          "analyzing",
			"filesScanned":   totalFiles,
			"filesAnalyzed":  analyzed,
			"totalToAnalyze": totalFiles,
			"itemsFound":     itemsFound,
		})
	}

	wailsrt.EventsEmit(a.ctx, "smartscan:complete", map[string]any{
		"totalScanned": totalFiles, "totalAnalyzed": analyzed, "itemsFound": itemsFound,
	})
}
