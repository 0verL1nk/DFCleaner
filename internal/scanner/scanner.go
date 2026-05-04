package scanner

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type Scanner struct {
	ctx         context.Context
	cancel      context.CancelFunc
	filesScanned atomic.Int64
	dirsScanned  atomic.Int64
	totalSize    atomic.Int64
}

func New(ctx context.Context) *Scanner {
	return &Scanner{ctx: ctx}
}

func (s *Scanner) Scan(root string, opts ScanOptions) (*ScanResult, error) {
	start := time.Now()

	ctx, cancel := context.WithCancel(s.ctx)
	s.cancel = cancel
	defer cancel()

	s.filesScanned.Store(0)
	s.dirsScanned.Store(0)
	s.totalSize.Store(0)

	if opts.MaxDepth <= 0 {
		opts.MaxDepth = 50
	}

	excludeSet := make(map[string]bool, len(opts.ExcludeDirs))
	for _, d := range opts.ExcludeDirs {
		excludeSet[strings.ToLower(d)] = true
	}

	result := &ScanResult{Root: root}

	chunkBuf := make([]FileEntry, 0, 500)
	var mu sync.Mutex
	var wg sync.WaitGroup

	paths := make(chan string, 100)

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range paths {
				if ctx.Err() != nil {
					return
				}
				entries, err := s.readDir(path)
				if err != nil {
					mu.Lock()
					result.Errors = append(result.Errors, FileEntry{
						Path:  path,
						Error: err.Error(),
					})
					mu.Unlock()
					continue
				}
				for _, e := range entries {
					mu.Lock()
					chunkBuf = append(chunkBuf, e)
					if len(chunkBuf) >= 500 {
						runtime.EventsEmit(s.ctx, "scan:chunk", ScanChunk{
							Entries:  chunkBuf,
							Progress: s.progress(path),
						})
						result.Entries = append(result.Entries, chunkBuf...)
						chunkBuf = make([]FileEntry, 0, 500)
					}
					mu.Unlock()

					if e.IsDir && !excludeSet[strings.ToLower(e.Name)] {
						depth := strings.Count(strings.TrimPrefix(e.Path, root), string(filepath.Separator))
						if depth < opts.MaxDepth {
							select {
							case paths <- e.Path:
							case <-ctx.Done():
								return
							}
						}
					}
				}
			}
		}()
	}

	s.dirsScanned.Add(1)
	paths <- root
	close(paths)
	wg.Wait()

	mu.Lock()
	if len(chunkBuf) > 0 {
		result.Entries = append(result.Entries, chunkBuf...)
		runtime.EventsEmit(s.ctx, "scan:chunk", ScanChunk{
			Entries:  chunkBuf,
			Progress: s.progress("done"),
		})
	}
	mu.Unlock()

	result.TotalFiles = int(s.filesScanned.Load())
	result.TotalDirs = int(s.dirsScanned.Load())
	result.TotalSize = s.totalSize.Load()
	result.DurationMs = time.Since(start).Milliseconds()

	runtime.EventsEmit(s.ctx, "scan:complete", result)
	return result, nil
}

func (s *Scanner) readDir(path string) ([]FileEntry, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var result []FileEntry
	for _, de := range entries {
		info, err := de.Info()
		if err != nil {
			result = append(result, FileEntry{
				Path:  filepath.Join(path, de.Name()),
				Name:  de.Name(),
				Error: err.Error(),
			})
			continue
		}

		entry := FileEntry{
			Path:      filepath.Join(path, de.Name()),
			Name:      de.Name(),
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			IsDir:     de.IsDir(),
			Extension: ext(de.Name()),
		}

		if sys := info.Sys(); sys != nil {
			entry.AccessTime = getAccessTime(sys)
		} else {
			entry.AccessTime = info.ModTime()
		}

		if de.IsDir() {
			s.dirsScanned.Add(1)
		} else {
			s.filesScanned.Add(1)
			s.totalSize.Add(info.Size())
		}

		result = append(result, entry)
	}
	return result, nil
}

func (s *Scanner) progress(current string) ScanProgress {
	return ScanProgress{
		FilesScanned: int(s.filesScanned.Load()),
		DirsScanned:  int(s.dirsScanned.Load()),
		CurrentPath:  current,
	}
}

func (s *Scanner) Cancel() {
	if s.cancel != nil {
		s.cancel()
	}
}

func ext(name string) string {
	e := filepath.Ext(name)
	if e != "" {
		return strings.ToLower(e[1:])
	}
	return ""
}
