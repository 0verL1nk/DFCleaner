package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

const (
	// maxResultChars is the approximate character limit for tool responses.
	// Results exceeding this are saved to a temp file.
	maxResultChars = 6000
	// defaultViewLimit is the default page size for view_result.
	defaultViewLimit = 50
)

// RegisterTools returns all file system tools for the Eino Agent.
func RegisterTools(s *Scanner) ([]tool.BaseTool, error) {
	scanDir, err := utils.InferTool("scan_directory",
		"Scan a directory. Returns: subDirSummary (all subdirs with file count + total size, sorted by size desc), all files sorted by size, file count, total size. If too many files, results are saved to a temp file — use view_result to page through.",
		func(ctx context.Context, input *ScanDirInput) (*ScanDirOutput, error) {
			return s.toolScanDirectory(ctx, input)
		})
	if err != nil {
		return nil, err
	}

	getFileInfo, err := utils.InferTool("get_file_info",
		"Get detailed metadata for a specific file or directory.",
		func(ctx context.Context, input *GetFileInfoInput) (*FileEntry, error) {
			return s.toolGetFileInfo(ctx, input)
		})
	if err != nil {
		return nil, err
	}

	getLargeFiles, err := utils.InferTool("get_large_files",
		"Find files larger than a given threshold (in bytes) in a directory tree. Large results are saved to a temp file — use view_result to page through.",
		func(ctx context.Context, input *GetLargeFilesInput) (*GetLargeFilesOutput, error) {
			return s.toolGetLargeFiles(ctx, input)
		})
	if err != nil {
		return nil, err
	}

	getOldFiles, err := utils.InferTool("get_old_files",
		"Find files not accessed within a given number of days in a directory tree. Large results are saved to a temp file — use view_result to page through.",
		func(ctx context.Context, input *GetOldFilesInput) (*GetOldFilesOutput, error) {
			return s.toolGetOldFiles(ctx, input)
		})
	if err != nil {
		return nil, err
	}

	findDuplicates, err := utils.InferTool("find_duplicates",
		"Find potential duplicate files based on filename and size in a directory tree. Large results are saved to a temp file — use view_result to page through.",
		func(ctx context.Context, input *FindDuplicatesInput) (*FindDuplicatesOutput, error) {
			return s.toolFindDuplicates(ctx, input)
		})
	if err != nil {
		return nil, err
	}

	viewResult, err := utils.InferTool("view_result",
		"Page through a large result set saved to a temp file. Returns items starting at offset, up to limit items.",
		func(ctx context.Context, input *ViewResultInput) (*ViewResultOutput, error) {
			return s.toolViewResult(ctx, input)
		})
	if err != nil {
		return nil, err
	}

	tools := []tool.BaseTool{scanDir, getFileInfo, getLargeFiles, getOldFiles, findDuplicates, viewResult}

	contentTools, err := RegisterContentTools(s)
	if err != nil {
		return nil, fmt.Errorf("register content tools: %w", err)
	}
	tools = append(tools, contentTools...)

	analysisTools, err := RegisterAnalysisTools(s)
	if err != nil {
		return nil, fmt.Errorf("register analysis tools: %w", err)
	}
	tools = append(tools, analysisTools...)

	return tools, nil
}

// --- Input/Output types ---

type ScanDirInput struct {
	Path     string `json:"path" jsonschema:"description=Directory path to scan"`
	MaxDepth int    `json:"maxDepth,omitempty" jsonschema:"description=Maximum recursion depth, default 2"`
}

type ScanDirOutput struct {
	Path          string          `json:"path"`
	SubDirs       int             `json:"subDirs"`
	FileCount     int             `json:"fileCount"`
	TotalSize     int64           `json:"totalSize"`
	LargestItems  []FileEntry     `json:"largestItems"`
	MoreItems     int             `json:"moreItems,omitempty"`
	ResultFile    string          `json:"resultFile,omitempty"`
	SubDirSummary []SubDirSummary `json:"subDirSummary"`
}

type SubDirSummary struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	FileCount int    `json:"fileCount"`
	TotalSize int64  `json:"totalSize"`
	IsDir     bool   `json:"isDir"`
}

type GetFileInfoInput struct {
	Path string `json:"path" jsonschema:"description=Path to the file or directory"`
}

type GetLargeFilesInput struct {
	Path      string `json:"path" jsonschema:"description=Directory to search"`
	Threshold int64  `json:"threshold" jsonschema:"description=Minimum file size in bytes"`
}

type GetLargeFilesOutput struct {
	Files      []FileEntry `json:"files"`
	MoreItems  int         `json:"moreItems,omitempty"`
	ResultFile string      `json:"resultFile,omitempty"`
}

type GetOldFilesInput struct {
	Path       string `json:"path" jsonschema:"description=Directory to search"`
	MaxAgeDays int    `json:"maxAgeDays" jsonschema:"description=Files not accessed in this many days"`
}

type GetOldFilesOutput struct {
	Files      []FileEntry `json:"files"`
	MoreItems  int         `json:"moreItems,omitempty"`
	ResultFile string      `json:"resultFile,omitempty"`
}

type FindDuplicatesInput struct {
	Path string `json:"path" jsonschema:"description=Directory to search for duplicates"`
}

type FindDuplicatesOutput struct {
	Groups     []DuplicateGroup `json:"groups"`
	MoreGroups int              `json:"moreGroups,omitempty"`
	ResultFile string           `json:"resultFile,omitempty"`
}

type DuplicateGroup struct {
	Name  string      `json:"name"`
	Size  int64       `json:"size"`
	Files []FileEntry `json:"files"`
}

type ViewResultInput struct {
	Path   string `json:"path" jsonschema:"description=Path to the saved result file"`
	Offset int    `json:"offset" jsonschema:"description=Start index, 0-based, default 0"`
	Limit  int    `json:"limit" jsonschema:"description=Max items to return, default 50"`
}

type ViewResultOutput struct {
	Items      []FileEntry `json:"items"`
	TotalCount int         `json:"totalCount"`
	Offset     int         `json:"offset"`
	HasMore    bool        `json:"hasMore"`
}

// --- Temp file overflow helpers ---

// overflowFiles checks if serializing files would exceed maxResultChars.
// If so, saves all files to a temp JSON file and returns a truncated slice.
func overflowFiles(files []FileEntry) (kept []FileEntry, resultFile string, moreItems int) {
	raw, err := json.Marshal(files)
	if err != nil || len(raw) <= maxResultChars {
		return files, "", 0
	}

	// Find how many items fit within the limit
	cutoff := len(files)
	for i := 1; i <= len(files); i++ {
		partial, _ := json.Marshal(files[:i])
		if len(partial) > maxResultChars {
			cutoff = i - 1
			break
		}
	}
	if cutoff < 1 {
		cutoff = 1
	}

	// Save full result to temp file
	tmpFile, err := os.CreateTemp("", "dfcleaner-result-*.json")
	if err != nil {
		return files[:cutoff], "", len(files) - cutoff
	}
	json.NewEncoder(tmpFile).Encode(files)
	tmpFile.Close()

	return files[:cutoff], tmpFile.Name(), len(files) - cutoff
}

// overflowGroups same logic for duplicate groups.
func overflowGroups(groups []DuplicateGroup) (kept []DuplicateGroup, resultFile string, moreGroups int) {
	raw, err := json.Marshal(groups)
	if err != nil || len(raw) <= maxResultChars {
		return groups, "", 0
	}

	cutoff := len(groups)
	for i := 1; i <= len(groups); i++ {
		partial, _ := json.Marshal(groups[:i])
		if len(partial) > maxResultChars {
			cutoff = i - 1
			break
		}
	}
	if cutoff < 1 {
		cutoff = 1
	}

	tmpFile, err := os.CreateTemp("", "dfcleaner-result-*.json")
	if err != nil {
		return groups[:cutoff], "", len(groups) - cutoff
	}
	json.NewEncoder(tmpFile).Encode(groups)
	tmpFile.Close()

	return groups[:cutoff], tmpFile.Name(), len(groups) - cutoff
}

// --- Tool implementations ---

func (s *Scanner) toolScanDirectory(ctx context.Context, input *ScanDirInput) (*ScanDirOutput, error) {
	if input.MaxDepth <= 0 {
		input.MaxDepth = 2
	}

	if input.Path == "" {
		return nil, fmt.Errorf("path is required. Please provide a directory path to scan.")
	}

	entries, err := os.ReadDir(input.Path)
	if err != nil {
		return &ScanDirOutput{Path: input.Path, SubDirs: 0, FileCount: 0, TotalSize: 0}, nil
	}

	out := &ScanDirOutput{Path: input.Path}
	var allFiles []FileEntry

	for _, de := range entries {
		info, err := de.Info()
		if err != nil {
			continue
		}
		entry := FileEntry{
			Path:      filepath.Join(input.Path, de.Name()),
			Name:      de.Name(),
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			IsDir:     de.IsDir(),
			Extension: ext(de.Name()),
		}
		if de.IsDir() {
			out.SubDirs++
			summary := SubDirSummary{
				Name:  de.Name(),
				Path:  entry.Path,
				IsDir: true,
			}
			childEntries, childErr := os.ReadDir(entry.Path)
			if childErr == nil {
				for _, ce := range childEntries {
					ci, ciErr := ce.Info()
					if ciErr != nil {
						continue
					}
					if !ce.IsDir() {
						summary.FileCount++
						summary.TotalSize += ci.Size()
					}
				}
			}
			out.SubDirSummary = append(out.SubDirSummary, summary)
		} else {
			out.FileCount++
			out.TotalSize += info.Size()
			allFiles = append(allFiles, entry)
		}
	}

	// Sort files by size descending — no hard limit, overflow to file if needed
	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].Size > allFiles[j].Size
	})

	kept, resultFile, more := overflowFiles(allFiles)
	out.LargestItems = kept
	out.MoreItems = more
	out.ResultFile = resultFile

	// Sort subdirs by total size descending
	sort.Slice(out.SubDirSummary, func(i, j int) bool {
		return out.SubDirSummary[i].TotalSize > out.SubDirSummary[j].TotalSize
	})

	return out, nil
}

func (s *Scanner) toolGetFileInfo(ctx context.Context, input *GetFileInfoInput) (*FileEntry, error) {
	info, err := os.Stat(input.Path)
	if err != nil {
		return &FileEntry{
			Path: input.Path,
			Name: filepath.Base(input.Path),
		}, nil
	}

	entry := &FileEntry{
		Path:      input.Path,
		Name:      filepath.Base(input.Path),
		Size:      info.Size(),
		ModTime:   info.ModTime(),
		IsDir:     info.IsDir(),
		Extension: ext(filepath.Base(input.Path)),
	}

	if sys := info.Sys(); sys != nil {
		entry.AccessTime = getAccessTime(sys)
	}
	return entry, nil
}

func (s *Scanner) toolGetLargeFiles(ctx context.Context, input *GetLargeFilesInput) (*GetLargeFilesOutput, error) {
	var files []FileEntry

	err := filepath.WalkDir(input.Path, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Size() >= input.Threshold {
			files = append(files, FileEntry{
				Path:      path,
				Name:      d.Name(),
				Size:      info.Size(),
				ModTime:   info.ModTime(),
				IsDir:     false,
				Extension: ext(d.Name()),
			})
		}
		return nil
	})
	if err != nil {
		return &GetLargeFilesOutput{Files: []FileEntry{}}, nil
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Size > files[j].Size
	})

	kept, resultFile, more := overflowFiles(files)
	return &GetLargeFilesOutput{Files: kept, MoreItems: more, ResultFile: resultFile}, nil
}

func (s *Scanner) toolGetOldFiles(ctx context.Context, input *GetOldFilesInput) (*GetOldFilesOutput, error) {
	cutoff := time.Now().AddDate(0, 0, -input.MaxAgeDays)
	var files []FileEntry

	err := filepath.WalkDir(input.Path, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}

		var atime time.Time
		if sys := info.Sys(); sys != nil {
			atime = getAccessTime(sys)
		}
		if atime.IsZero() || atime.Before(cutoff) {
			files = append(files, FileEntry{
				Path:       path,
				Name:       d.Name(),
				Size:       info.Size(),
				ModTime:    info.ModTime(),
				AccessTime: atime,
				IsDir:      false,
				Extension:  ext(d.Name()),
			})
		}
		return nil
	})
	if err != nil {
		return &GetOldFilesOutput{Files: []FileEntry{}}, nil
	}

	kept, resultFile, more := overflowFiles(files)
	return &GetOldFilesOutput{Files: kept, MoreItems: more, ResultFile: resultFile}, nil
}

func (s *Scanner) toolFindDuplicates(ctx context.Context, input *FindDuplicatesInput) (*FindDuplicatesOutput, error) {
	type key struct {
		name string
		size int64
	}
	seen := map[key][]FileEntry{}

	err := filepath.WalkDir(input.Path, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}

		k := key{name: strings.ToLower(d.Name()), size: info.Size()}
		seen[k] = append(seen[k], FileEntry{
			Path:    path,
			Name:    d.Name(),
			Size:    info.Size(),
			ModTime: info.ModTime(),
		})
		return nil
	})
	if err != nil {
		return &FindDuplicatesOutput{Groups: []DuplicateGroup{}}, nil
	}

	var groups []DuplicateGroup
	for k, files := range seen {
		if len(files) > 1 {
			groups = append(groups, DuplicateGroup{
				Name:  k.name,
				Size:  k.size,
				Files: files,
			})
		}
	}

	kept, resultFile, more := overflowGroups(groups)
	return &FindDuplicatesOutput{Groups: kept, MoreGroups: more, ResultFile: resultFile}, nil
}

func (s *Scanner) toolViewResult(ctx context.Context, input *ViewResultInput) (*ViewResultOutput, error) {
	if input.Limit <= 0 {
		input.Limit = defaultViewLimit
	}
	if input.Offset < 0 {
		input.Offset = 0
	}

	data, err := os.ReadFile(input.Path)
	if err != nil {
		return &ViewResultOutput{Items: []FileEntry{}, TotalCount: 0, Offset: input.Offset, HasMore: false}, nil
	}

	var allFiles []FileEntry
	if err := json.Unmarshal(data, &allFiles); err != nil {
		return &ViewResultOutput{Items: []FileEntry{}, TotalCount: 0, Offset: input.Offset, HasMore: false}, nil
	}

	total := len(allFiles)
	end := input.Offset + input.Limit
	if end > total {
		end = total
	}
	if input.Offset >= total {
		return &ViewResultOutput{Items: []FileEntry{}, TotalCount: total, Offset: input.Offset, HasMore: false}, nil
	}

	return &ViewResultOutput{
		Items:      allFiles[input.Offset:end],
		TotalCount: total,
		Offset:     input.Offset,
		HasMore:    end < total,
	}, nil
}
