package scanner

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// --- read_file ---

type ReadFileInput struct {
	Path     string `json:"path" jsonschema:"description=Path to the file to read"`
	MaxLines int    `json:"maxLines,omitempty" jsonschema:"description=Maximum lines to return, default 100"`
	Offset   int    `json:"offset,omitempty" jsonschema:"description=Line offset to start from, 0-based, default 0"`
}

type ReadFileOutput struct {
	Content    string `json:"content"`
	TotalLines int    `json:"totalLines"`
	Truncated  bool   `json:"truncated"`
	IsBinary   bool   `json:"isBinary,omitempty"`
}

func (s *Scanner) toolReadFile(_ context.Context, input *ReadFileInput) (*ReadFileOutput, error) {
	maxLines := input.MaxLines
	if maxLines <= 0 {
		maxLines = 100
	}

	f, err := os.Open(input.Path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, _ := f.Read(buf)
	if n > 0 && isBinary(buf[:n]) {
		return &ReadFileOutput{IsBinary: true, TotalLines: 0}, nil
	}
	f.Seek(0, 0)

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var lines []string
	totalLines := 0
	lineNum := 0

	for sc.Scan() {
		lineNum++
		totalLines++
		if lineNum <= input.Offset {
			continue
		}
		if len(lines) >= maxLines {
			continue
		}
		lines = append(lines, sc.Text())
	}

	return &ReadFileOutput{
		Content:    strings.Join(lines, "\n"),
		TotalLines: totalLines,
		Truncated:  totalLines > input.Offset+maxLines,
	}, nil
}

// --- read_file_tail ---

type ReadFileTailInput struct {
	Path  string `json:"path" jsonschema:"description=Path to the file"`
	Lines int    `json:"lines,omitempty" jsonschema:"description=Number of lines from the end, default 50"`
}

type ReadFileTailOutput struct {
	Content    string `json:"content"`
	TotalLines int    `json:"totalLines"`
}

func (s *Scanner) toolReadFileTail(_ context.Context, input *ReadFileTailInput) (*ReadFileTailOutput, error) {
	n := input.Lines
	if n <= 0 {
		n = 50
	}

	f, err := os.Open(input.Path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	buf := make([]byte, 512)
	readN, _ := f.Read(buf)
	if readN > 0 && isBinary(buf[:readN]) {
		return &ReadFileTailOutput{TotalLines: 0}, nil
	}
	f.Seek(0, 0)

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	ring := make([]string, n)
	idx := 0
	totalLines := 0

	for sc.Scan() {
		ring[idx%n] = sc.Text()
		idx++
		totalLines++
	}

	start := 0
	count := totalLines
	if totalLines > n {
		start = idx % n
		count = n
	}

	lines := make([]string, 0, count)
	for i := 0; i < count; i++ {
		lines = append(lines, ring[(start+i)%n])
	}

	return &ReadFileTailOutput{
		Content:    strings.Join(lines, "\n"),
		TotalLines: totalLines,
	}, nil
}

// --- search_files ---

type SearchFilesInput struct {
	Path       string `json:"path" jsonschema:"description=Directory to search in"`
	Pattern    string `json:"pattern" jsonschema:"description=Glob pattern to match file names, e.g. *.log, *.tmp, package.json"`
	MaxResults int    `json:"maxResults,omitempty" jsonschema:"description=Maximum results to return, default 100"`
}

type SearchFilesOutput struct {
	Files        []FileEntry `json:"files"`
	TotalMatches int         `json:"totalMatches"`
	MoreResults  bool        `json:"moreResults"`
}

func (s *Scanner) toolSearchFiles(_ context.Context, input *SearchFilesInput) (*SearchFilesOutput, error) {
	maxResults := input.MaxResults
	if maxResults <= 0 {
		maxResults = 100
	}

	var matches []FileEntry
	totalMatches := 0

	err := filepath.WalkDir(input.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		matched, _ := filepath.Match(input.Pattern, d.Name())
		if !matched {
			return nil
		}

		totalMatches++
		if len(matches) < maxResults {
			info, infoErr := d.Info()
			entry := FileEntry{Path: path, Name: d.Name(), IsDir: d.IsDir()}
			if infoErr == nil {
				entry.Size = info.Size()
				entry.ModTime = info.ModTime()
			}
			matches = append(matches, entry)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}

	return &SearchFilesOutput{
		Files:        matches,
		TotalMatches: totalMatches,
		MoreResults:  totalMatches > maxResults,
	}, nil
}

// --- grep_content ---

type GrepContentInput struct {
	Path        string `json:"path" jsonschema:"description=Directory to search in"`
	Query       string `json:"query" jsonschema:"description=Text pattern to search for in file contents"`
	FilePattern string `json:"filePattern,omitempty" jsonschema:"description=Glob pattern to filter files, default *"`
	MaxResults  int    `json:"maxResults,omitempty" jsonschema:"description=Maximum matches to return, default 50"`
}

type GrepMatch struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Content string `json:"content"`
}

type GrepContentOutput struct {
	Matches      []GrepMatch `json:"matches"`
	TotalMatches int         `json:"totalMatches"`
	MoreResults  bool        `json:"moreResults"`
}

func (s *Scanner) toolGrepContent(_ context.Context, input *GrepContentInput) (*GrepContentOutput, error) {
	maxResults := input.MaxResults
	if maxResults <= 0 {
		maxResults = 50
	}
	pattern := input.FilePattern
	if pattern == "" {
		pattern = "*"
	}

	var matches []GrepMatch
	totalMatches := 0

	err := filepath.WalkDir(input.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		matched, _ := filepath.Match(pattern, d.Name())
		if !matched {
			return nil
		}

		f, openErr := os.Open(path)
		if openErr != nil {
			return nil
		}
		defer f.Close()

		buf := make([]byte, 512)
		n, _ := f.Read(buf)
		if n > 0 && isBinary(buf[:n]) {
			return nil
		}
		f.Seek(0, 0)

		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		lineNum := 0
		for sc.Scan() {
			lineNum++
			if strings.Contains(sc.Text(), input.Query) {
				totalMatches++
				if len(matches) < maxResults {
					content := sc.Text()
					if len(content) > 200 {
						content = content[:200] + "..."
					}
					matches = append(matches, GrepMatch{
						File:    path,
						Line:    lineNum,
						Content: content,
					})
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}

	return &GrepContentOutput{
		Matches:      matches,
		TotalMatches: totalMatches,
		MoreResults:  totalMatches > maxResults,
	}, nil
}

// --- helpers ---

func isBinary(buf []byte) bool {
	for _, b := range buf {
		if b == 0 {
			return true
		}
	}
	return false
}

// RegisterContentTools returns the 4 file content tools.
func RegisterContentTools(s *Scanner) ([]tool.BaseTool, error) {
	readFile, err := utils.InferTool("read_file",
		"Read the contents of a text file. Returns lines starting from offset. Use to inspect config files, logs, package.json, etc. Skips binary files.",
		func(ctx context.Context, input *ReadFileInput) (*ReadFileOutput, error) {
			return s.toolReadFile(ctx, input)
		})
	if err != nil {
		return nil, err
	}

	readTail, err := utils.InferTool("read_file_tail",
		"Read the last N lines of a text file. Useful for viewing recent log entries.",
		func(ctx context.Context, input *ReadFileTailInput) (*ReadFileTailOutput, error) {
			return s.toolReadFileTail(ctx, input)
		})
	if err != nil {
		return nil, err
	}

	searchFiles, err := utils.InferTool("search_files",
		"Search for files matching a glob pattern in a directory tree. Use to find files like *.log, *.tmp, package.json, etc.",
		func(ctx context.Context, input *SearchFilesInput) (*SearchFilesOutput, error) {
			return s.toolSearchFiles(ctx, input)
		})
	if err != nil {
		return nil, err
	}

	grepContent, err := utils.InferTool("grep_content",
		"Search for a text pattern inside files in a directory. Like grep. Optionally filter files by glob pattern. Skips binary files.",
		func(ctx context.Context, input *GrepContentInput) (*GrepContentOutput, error) {
			return s.toolGrepContent(ctx, input)
		})
	if err != nil {
		return nil, err
	}

	return []tool.BaseTool{readFile, readTail, searchFiles, grepContent}, nil
}
