package scanner

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// DefaultExcludeDirs are directories that should be skipped on all platforms.
var DefaultExcludeDirs = []string{
	"System Volume Information",
	"$RECYCLE.BIN",
	"Windows",
	"ProgramData",
	"System32",
	"Recovery",
	".git",
	".svn",
	"node_modules",
	".Trash",
}

type Scanner struct {
	ctx         context.Context
	cancel      context.CancelFunc
	filesScanned atomic.Int64
	dirsScanned  atomic.Int64
	totalSize    atomic.Int64
	logger       *log.Logger
}

func New(ctx context.Context) *Scanner {
	return &Scanner{
		logger: log.New(os.Stderr, "[scanner] ", log.LstdFlags|log.Lshortfile),
	}
}

func (s *Scanner) Scan(root string, opts ScanOptions) (result *ScanResult, err error) {
	start := time.Now()

	// Panic recovery for the entire scan
	defer func() {
		if r := recover(); r != nil {
			s.logger.Printf("panic during scan of %s: %v", root, r)
			err = fmt.Errorf("scan panicked: %v", r)
		}
	}()

	ctx, cancel := context.WithCancel(s.ctx)
	s.cancel = cancel
	defer cancel()

	s.filesScanned.Store(0)
	s.dirsScanned.Store(0)
	s.totalSize.Store(0)

	if opts.MaxDepth <= 0 {
		opts.MaxDepth = 50
	}

	excludeSet := make(map[string]bool, len(opts.ExcludeDirs)+len(DefaultExcludeDirs))
	for _, d := range opts.ExcludeDirs {
		excludeSet[strings.ToLower(d)] = true
	}
	for _, d := range DefaultExcludeDirs {
		excludeSet[strings.ToLower(d)] = true
	}

	result = &ScanResult{Root: root}

	chunkBuf := make([]FileEntry, 0, 500)
	var mu sync.Mutex
	var wg sync.WaitGroup

	paths := make(chan string, 100)

	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					s.logger.Printf("worker %d panic: %v", id, r)
				}
			}()
			for path := range paths {
				if ctx.Err() != nil {
					return
				}
				entries, err := s.readDir(path)
				if err != nil {
					if os.IsPermission(err) {
						continue
					}
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
		}(i)
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

func (s *Scanner) readDir(path string) (entries []FileEntry, err error) {
	defer func() {
		if r := recover(); r != nil {
			s.logger.Printf("panic reading %s: %v", path, r)
			err = fmt.Errorf("readDir panic: %v", r)
		}
	}()

	dirEntries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, de := range dirEntries {
		info, err := de.Info()
		if err != nil {
			entries = append(entries, FileEntry{
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

		entries = append(entries, entry)
	}
	return entries, nil
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
