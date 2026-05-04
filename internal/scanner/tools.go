package scanner

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// RegisterTools returns all file system tools for the Eino Agent.
func RegisterTools(s *Scanner) ([]tool.BaseTool, error) {
	scanDir, err := utils.InferTool("scan_directory",
		"Scan a directory and return an overview of its contents (subdirectory count, file count, total size, largest items).",
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
		"Find files larger than a given threshold (in bytes) in a directory tree.",
		func(ctx context.Context, input *GetLargeFilesInput) (*GetLargeFilesOutput, error) {
			return s.toolGetLargeFiles(ctx, input)
		})
	if err != nil {
		return nil, err
	}

	getOldFiles, err := utils.InferTool("get_old_files",
		"Find files not accessed within a given number of days in a directory tree.",
		func(ctx context.Context, input *GetOldFilesInput) (*GetOldFilesOutput, error) {
			return s.toolGetOldFiles(ctx, input)
		})
	if err != nil {
		return nil, err
	}

	findDuplicates, err := utils.InferTool("find_duplicates",
		"Find potential duplicate files based on filename and size in a directory tree.",
		func(ctx context.Context, input *FindDuplicatesInput) (*FindDuplicatesOutput, error) {
			return s.toolFindDuplicates(ctx, input)
		})
	if err != nil {
		return nil, err
	}

	return []tool.BaseTool{scanDir, getFileInfo, getLargeFiles, getOldFiles, findDuplicates}, nil
}

// --- Input/Output types for tools ---

type ScanDirInput struct {
	Path     string `json:"path" jsonschema:"description=Directory path to scan"`
	MaxDepth int    `json:"maxDepth,omitempty" jsonschema:"description=Maximum recursion depth, default 2"`
}

type ScanDirOutput struct {
	Path        string      `json:"path"`
	SubDirs     int         `json:"subDirs"`
	FileCount   int         `json:"fileCount"`
	TotalSize   int64       `json:"totalSize"`
	LargestItems []FileEntry `json:"largestItems"`
}

type GetFileInfoInput struct {
	Path string `json:"path" jsonschema:"description=Path to the file or directory"`
}

type GetLargeFilesInput struct {
	Path      string `json:"path" jsonschema:"description=Directory to search"`
	Threshold int64  `json:"threshold" jsonschema:"description=Minimum file size in bytes"`
}

type GetLargeFilesOutput struct {
	Files []FileEntry `json:"files"`
}

type GetOldFilesInput struct {
	Path     string `json:"path" jsonschema:"description=Directory to search"`
	MaxAgeDays int  `json:"maxAgeDays" jsonschema:"description=Files not accessed in this many days"`
}

type GetOldFilesOutput struct {
	Files []FileEntry `json:"files"`
}

type FindDuplicatesInput struct {
	Path string `json:"path" jsonschema:"description=Directory to search for duplicates"`
}

type FindDuplicatesOutput struct {
	Groups []DuplicateGroup `json:"groups"`
}

type DuplicateGroup struct {
	Name  string       `json:"name"`
	Size  int64        `json:"size"`
	Files []FileEntry  `json:"files"`
}

// --- Tool implementations ---

func (s *Scanner) toolScanDirectory(ctx context.Context, input *ScanDirInput) (*ScanDirOutput, error) {
	if input.MaxDepth <= 0 {
		input.MaxDepth = 2
	}

	entries, err := os.ReadDir(input.Path)
	if err != nil {
		return nil, err
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
		} else {
			out.FileCount++
			out.TotalSize += info.Size()
			allFiles = append(allFiles, entry)
		}
	}

	sort.Slice(allFiles, func(i, j int) bool {
		return allFiles[i].Size > allFiles[j].Size
	})
	if len(allFiles) > 10 {
		allFiles = allFiles[:10]
	}
	out.LargestItems = allFiles
	return out, nil
}

func (s *Scanner) toolGetFileInfo(ctx context.Context, input *GetFileInfoInput) (*FileEntry, error) {
	info, err := os.Stat(input.Path)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	sort.Slice(files, func(i, j int) bool {
		return files[i].Size > files[j].Size
	})
	return &GetLargeFilesOutput{Files: files}, nil
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
				Path:      path,
				Name:      d.Name(),
				Size:      info.Size(),
				ModTime:   info.ModTime(),
				AccessTime: atime,
				IsDir:     false,
				Extension: ext(d.Name()),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &GetOldFilesOutput{Files: files}, nil
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
		return nil, err
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
	return &FindDuplicatesOutput{Groups: groups}, nil
}
