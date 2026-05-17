package scanner

import (
	"context"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// --- get_directory_size ---

type GetDirectorySizeInput struct {
	Path     string `json:"path" jsonschema:"description=Directory path to calculate size for"`
	MaxDepth int    `json:"maxDepth,omitempty" jsonschema:"description=Maximum recursion depth, default 3"`
}

type SubDirSize struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

type GetDirectorySizeOutput struct {
	TotalSize int64        `json:"totalSize"`
	FileCount int          `json:"fileCount"`
	DirCount  int          `json:"dirCount"`
	SubDirs   []SubDirSize `json:"subDirs"`
}

func (s *Scanner) toolGetDirectorySize(_ context.Context, input *GetDirectorySizeInput) (*GetDirectorySizeOutput, error) {
	maxDepth := input.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 3
	}

	result := &GetDirectorySizeOutput{}
	subDirSizes := map[string]int64{}

	err := filepath.WalkDir(input.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		rel, relErr := filepath.Rel(input.Path, path)
		if relErr != nil {
			return nil
		}
		depth := strings.Count(rel, string(filepath.Separator))
		if rel == "." {
			depth = 0
		}

		if depth > maxDepth {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			if path != input.Path {
				result.DirCount++
			}
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}
		result.FileCount++
		result.TotalSize += info.Size()

		if depth > 0 {
			topDir := strings.Split(rel, string(filepath.Separator))[0]
			subDirSizes[topDir] += info.Size()
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}

	for name, size := range subDirSizes {
		result.SubDirs = append(result.SubDirs, SubDirSize{
			Name: name,
			Path: filepath.Join(input.Path, name),
			Size: size,
		})
	}
	sort.Slice(result.SubDirs, func(i, j int) bool {
		return result.SubDirs[i].Size > result.SubDirs[j].Size
	})

	return result, nil
}

// --- analyze_disk_usage ---

type AnalyzeDiskUsageInput struct {
	Path string `json:"path" jsonschema:"description=Directory path to analyze"`
}

type ExtensionStat struct {
	Extension string `json:"extension"`
	Count     int    `json:"count"`
	TotalSize int64  `json:"totalSize"`
}

type CategoryStat struct {
	Category  string `json:"category"`
	Count     int    `json:"count"`
	TotalSize int64  `json:"totalSize"`
}

type AnalyzeDiskUsageOutput struct {
	TotalSize   int64           `json:"totalSize"`
	FileCount   int             `json:"fileCount"`
	ByExtension []ExtensionStat `json:"byExtension"`
	ByCategory  []CategoryStat  `json:"byCategory"`
}

func (s *Scanner) toolAnalyzeDiskUsage(_ context.Context, input *AnalyzeDiskUsageInput) (*AnalyzeDiskUsageOutput, error) {
	extMap := map[string]*ExtensionStat{}
	catMap := map[string]*CategoryStat{}
	result := &AnalyzeDiskUsageOutput{}

	err := filepath.WalkDir(input.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			return nil
		}

		result.FileCount++
		result.TotalSize += info.Size()

		ext := strings.ToLower(filepath.Ext(d.Name()))
		if ext == "" {
			ext = "(no extension)"
		}

		if es, ok := extMap[ext]; ok {
			es.Count++
			es.TotalSize += info.Size()
		} else {
			extMap[ext] = &ExtensionStat{Extension: ext, Count: 1, TotalSize: info.Size()}
		}

		cat := categorizeExtension(ext)
		if cs, ok := catMap[cat]; ok {
			cs.Count++
			cs.TotalSize += info.Size()
		} else {
			catMap[cat] = &CategoryStat{Category: cat, Count: 1, TotalSize: info.Size()}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}

	for _, es := range extMap {
		result.ByExtension = append(result.ByExtension, *es)
	}
	sort.Slice(result.ByExtension, func(i, j int) bool {
		return result.ByExtension[i].TotalSize > result.ByExtension[j].TotalSize
	})
	if len(result.ByExtension) > 20 {
		result.ByExtension = result.ByExtension[:20]
	}

	for _, cs := range catMap {
		result.ByCategory = append(result.ByCategory, *cs)
	}
	sort.Slice(result.ByCategory, func(i, j int) bool {
		return result.ByCategory[i].TotalSize > result.ByCategory[j].TotalSize
	})

	return result, nil
}

// --- get_system_info ---

type GetSystemInfoOutput struct {
	OS        string `json:"os"`
	Arch      string `json:"arch"`
	TotalDisk int64  `json:"totalDisk"`
	FreeDisk  int64  `json:"freeDisk"`
	UsedDisk  int64  `json:"usedDisk"`
	HostName  string `json:"hostName"`
}

func (s *Scanner) toolGetSystemInfo(_ context.Context, _ struct{}) (*GetSystemInfoOutput, error) {
	total, free, used := getDiskInfo()
	hostname, _ := os.Hostname()

	return &GetSystemInfoOutput{
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		TotalDisk: total,
		FreeDisk:  free,
		UsedDisk:  used,
		HostName:  hostname,
	}, nil
}

// --- calculate_hash ---

type CalculateHashInput struct {
	Path      string `json:"path" jsonschema:"description=Path to the file"`
	Algorithm string `json:"algorithm,omitempty" jsonschema:"description=Hash algorithm: md5 or sha256, default md5"`
}

type CalculateHashOutput struct {
	Hash string `json:"hash"`
	Size int64  `json:"size"`
}

func (s *Scanner) toolCalculateHash(_ context.Context, input *CalculateHashInput) (*CalculateHashOutput, error) {
	f, err := os.Open(input.Path)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	info, _ := f.Stat()

	var h hash.Hash
	switch strings.ToLower(input.Algorithm) {
	case "sha256":
		h = sha256.New()
	default:
		h = md5.New()
	}

	if _, err := io.Copy(h, f); err != nil {
		return nil, fmt.Errorf("hash file: %w", err)
	}

	return &CalculateHashOutput{
		Hash: hex.EncodeToString(h.Sum(nil)),
		Size: info.Size(),
	}, nil
}

// --- check_package_manager ---

type CheckPackageManagerInput struct {
	Path string `json:"path" jsonschema:"description=Directory path to check"`
}

type CheckPackageManagerOutput struct {
	Manager      string `json:"manager"`
	LockFile     string `json:"lockFile"`
	DepCount     int    `json:"depCount"`
	CanReinstall bool   `json:"canReinstall"`
}

func (s *Scanner) toolCheckPackageManager(_ context.Context, input *CheckPackageManagerInput) (*CheckPackageManagerOutput, error) {
	result := &CheckPackageManagerOutput{Manager: "none"}

	if _, err := os.Stat(filepath.Join(input.Path, "package.json")); err == nil {
		result.Manager = "npm"
		result.CanReinstall = true
		for _, lock := range []string{"pnpm-lock.yaml", "yarn.lock", "package-lock.json", "bun.lockb"} {
			if _, err := os.Stat(filepath.Join(input.Path, lock)); err == nil {
				result.LockFile = lock
				break
			}
		}
		if result.LockFile == "pnpm-lock.yaml" {
			result.Manager = "pnpm"
		} else if result.LockFile == "yarn.lock" {
			result.Manager = "yarn"
		} else if result.LockFile == "bun.lockb" {
			result.Manager = "bun"
		}
	}

	for _, f := range []string{"requirements.txt", "Pipfile", "pyproject.toml"} {
		if _, err := os.Stat(filepath.Join(input.Path, f)); err == nil {
			if result.Manager == "none" {
				result.Manager = "pip"
				result.LockFile = f
				result.CanReinstall = true
			}
			break
		}
	}

	if _, err := os.Stat(filepath.Join(input.Path, "go.mod")); err == nil {
		if result.Manager == "none" {
			result.Manager = "go"
			result.LockFile = "go.sum"
			result.CanReinstall = true
		}
	}

	if _, err := os.Stat(filepath.Join(input.Path, "Cargo.toml")); err == nil {
		if result.Manager == "none" {
			result.Manager = "cargo"
			result.LockFile = "Cargo.lock"
			result.CanReinstall = true
		}
	}

	return result, nil
}

// --- get_path_info ---

type GetPathInfoInput struct {
	Path string `json:"path" jsonschema:"description=Path to analyze"`
}

type GetPathInfoOutput struct {
	PathType     string `json:"pathType"`
	IsXdgCache   bool   `json:"isXdgCache"`
	IsXdgData    bool   `json:"isXdgData"`
	IsXdgConfig  bool   `json:"isXdgConfig"`
	IsProject    bool   `json:"isProject"`
	ProjectType  string `json:"projectType"`
	StandardName string `json:"standardName"`
}

func (s *Scanner) toolGetPathInfo(_ context.Context, input *GetPathInfoInput) (*GetPathInfoOutput, error) {
	result := &GetPathInfoOutput{}

	abs, err := filepath.Abs(input.Path)
	if err != nil {
		abs = input.Path
	}
	base := filepath.Base(abs)

	xdgCache, _ := os.UserCacheDir()
	xdgData := xdgDir("XDG_DATA_HOME", "/.local/share")
	xdgConfig, _ := os.UserConfigDir()

	result.IsXdgCache = strings.HasPrefix(abs, xdgCache)
	result.IsXdgData = strings.HasPrefix(abs, xdgData)
	result.IsXdgConfig = strings.HasPrefix(abs, xdgConfig)

	projectFiles := map[string]string{
		"package.json":   "node",
		"go.mod":         "go",
		"Cargo.toml":     "rust",
		"pyproject.toml": "python",
		"Pipfile":        "python",
		".git":           "git",
	}
	for file, ptype := range projectFiles {
		if _, err := os.Stat(filepath.Join(abs, file)); err == nil {
			result.IsProject = true
			if result.ProjectType == "" {
				result.ProjectType = ptype
			}
		}
	}

	switch {
	case result.IsXdgCache:
		result.PathType = "cache"
	case result.IsXdgConfig:
		result.PathType = "config"
	case result.IsXdgData:
		result.PathType = "data"
	case result.IsProject:
		result.PathType = "project"
	case base == "node_modules":
		result.PathType = "dependencies"
	case base == "dist" || base == "build" || base == "target" || base == "out":
		result.PathType = "build-output"
	case base == "tmp" || base == "temp" || base == ".tmp":
		result.PathType = "temp"
	case base == "Downloads" || base == "downloads":
		result.PathType = "downloads"
	case base == "Documents" || base == "Desktop":
		result.PathType = "user-documents"
	case base == ".git":
		result.PathType = "version-control"
	default:
		result.PathType = "other"
	}

	result.StandardName = base

	return result, nil
}

// --- helpers ---

func categorizeExtension(ext string) string {
	switch ext {
	case ".js", ".ts", ".jsx", ".tsx", ".mjs", ".cjs":
		return "javascript"
	case ".go":
		return "go"
	case ".py", ".pyc", ".pyd", ".pyo":
		return "python"
	case ".rs":
		return "rust"
	case ".java", ".kt", ".class":
		return "jvm"
	case ".c", ".h", ".cpp", ".hpp":
		return "c-cpp"
	case ".css", ".scss", ".less", ".sass":
		return "stylesheet"
	case ".html", ".htm":
		return "html"
	case ".json", ".yaml", ".yml", ".toml", ".xml", ".ini", ".cfg":
		return "config"
	case ".md", ".txt", ".rst", ".adoc":
		return "documentation"
	case ".log":
		return "log"
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".webp":
		return "image"
	case ".mp4", ".mkv", ".avi", ".mov", ".wmv":
		return "video"
	case ".mp3", ".flac", ".wav", ".aac", ".ogg":
		return "audio"
	case ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar":
		return "archive"
	case ".db", ".sqlite", ".sqlite3":
		return "database"
	case ".so", ".dll", ".dylib", ".a", ".lib":
		return "library"
	case ".exe", ".bin", ".app":
		return "executable"
	default:
		return "other"
	}
}

func xdgDir(env, fallback string) string {
	if v := os.Getenv(env); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return home + fallback
}

// RegisterAnalysisTools returns the 6 disk analysis tools.
func RegisterAnalysisTools(s *Scanner) ([]tool.BaseTool, error) {
	dirSize, err := utils.InferTool("get_directory_size",
		"Recursively calculate the exact total size of a directory. Returns total size, file count, and per-subdirectory size breakdown sorted by size.",
		func(ctx context.Context, input *GetDirectorySizeInput) (*GetDirectorySizeOutput, error) {
			return s.toolGetDirectorySize(ctx, input)
		})
	if err != nil {
		return nil, err
	}

	analyzeUsage, err := utils.InferTool("analyze_disk_usage",
		"Analyze disk usage in a directory. Returns breakdown by file extension and category (code, images, logs, etc.), sorted by size.",
		func(ctx context.Context, input *AnalyzeDiskUsageInput) (*AnalyzeDiskUsageOutput, error) {
			return s.toolAnalyzeDiskUsage(ctx, input)
		})
	if err != nil {
		return nil, err
	}

	sysInfo, err := utils.InferTool("get_system_info",
		"Get system information: OS, architecture, disk space usage (total, used, free), and hostname.",
		func(ctx context.Context, _ struct{}) (*GetSystemInfoOutput, error) {
			return s.toolGetSystemInfo(ctx, struct{}{})
		})
	if err != nil {
		return nil, err
	}

	calcHash, err := utils.InferTool("calculate_hash",
		"Calculate the MD5 or SHA256 hash of a file. Use for precise duplicate detection by comparing file hashes.",
		func(ctx context.Context, input *CalculateHashInput) (*CalculateHashOutput, error) {
			return s.toolCalculateHash(ctx, input)
		})
	if err != nil {
		return nil, err
	}

	checkPkgMgr, err := utils.InferTool("check_package_manager",
		"Detect which package manager a project uses (npm/pnpm/yarn/pip/go/cargo). Returns lock file and whether dependencies can be reinstalled.",
		func(ctx context.Context, input *CheckPackageManagerInput) (*CheckPackageManagerOutput, error) {
			return s.toolCheckPackageManager(ctx, input)
		})
	if err != nil {
		return nil, err
	}

	pathInfo, err := utils.InferTool("get_path_info",
		"Analyze a path to determine its type: cache directory, project, build output, temp files, user documents, etc. Detects XDG standard locations.",
		func(ctx context.Context, input *GetPathInfoInput) (*GetPathInfoOutput, error) {
			return s.toolGetPathInfo(ctx, input)
		})
	if err != nil {
		return nil, err
	}

	return []tool.BaseTool{dirSize, analyzeUsage, sysInfo, calcHash, checkPkgMgr, pathInfo}, nil
}
