package builtin

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSplitPattern(t *testing.T) {
	tests := []struct {
		pattern         string
		wantDirPattern  string
		wantFilePattern string
	}{
		{"**", "**", "*"},
		{"*.go", "", "*.go"},
		{"executor/**", "executor", "**"},
		{"executor/*", "executor", "*"},
		{"executor/*.go", "executor", "*.go"},
		{"**/*.go", "**", "*.go"},
		{"a/b/c/*.go", "a/b/c", "*.go"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			gotDir, gotFile := splitPattern(tt.pattern)
			if gotDir != tt.wantDirPattern {
				t.Errorf("splitPattern(%q) dir = %q, want %q", tt.pattern, gotDir, tt.wantDirPattern)
			}
			if gotFile != tt.wantFilePattern {
				t.Errorf("splitPattern(%q) file = %q, want %q", tt.pattern, gotFile, tt.wantFilePattern)
			}
		})
	}
}

func TestMatchFile(t *testing.T) {
	tests := []struct {
		relPath     string
		dirPattern  string
		filePattern string
		fileName    string
		want        bool
	}{
		// 只有文件模式
		{"test.go", "", "*.go", "test.go", true},
		{"test.py", "", "*.go", "test.py", false},
		{"main.go", "", "*.go", "main.go", true},

		// **/*.go 模式
		{"executor/runner.go", "**", "*.go", "runner.go", true},
		{"executor/runner.py", "**", "*.go", "runner.py", false},
		{"deep/nested/file.go", "**", "*.go", "file.go", true},

		// executor/* 模式
		{"executor/runner.go", "executor", "*", "runner.go", true},
		{"other/runner.go", "executor", "*", "runner.go", false},

		// executor/*.go 模式
		{"executor/runner.go", "executor", "*.go", "runner.go", true},
		{"executor/runner.py", "executor", "*.go", "runner.py", false},
		{"other/runner.go", "executor", "*.go", "runner.go", false},
		{"executor/sub/file.go", "executor", "*.go", "file.go", false}, // 不匹配子目录

		// ** 模式
		{"any/file.go", "**", "*", "file.go", true},
		{"file.go", "**", "*", "file.go", true},
	}

	for _, tt := range tests {
		t.Run(tt.relPath+"_"+tt.dirPattern+"_"+tt.filePattern, func(t *testing.T) {
			got := matchFile(tt.relPath, tt.dirPattern, tt.filePattern, tt.fileName)
			if got != tt.want {
				t.Errorf("matchFile(%q, %q, %q, %q) = %v, want %v",
					tt.relPath, tt.dirPattern, tt.filePattern, tt.fileName, got, tt.want)
			}
		})
	}
}

func TestGlobTool_searchFiles(t *testing.T) {
	// 创建临时目录结构
	tmpDir := t.TempDir()

	// 创建测试目录结构
	// tmpDir/
	//   executor/
	//     runner.go
	//     helper.go
	//   other/
	//     file.txt
	//   root.go

	os.MkdirAll(filepath.Join(tmpDir, "executor"), 0755)
	os.MkdirAll(filepath.Join(tmpDir, "other"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "executor", "runner.go"), []byte("package executor"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "executor", "helper.go"), []byte("package executor"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "other", "file.txt"), []byte("text"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "root.go"), []byte("package root"), 0644)

	tool := NewGlobTool()
	globTool := tool.(*GlobTool)

	tests := []struct {
		name      string
		pattern   string
		recursive bool
		wantCount int
		wantFiles []string
	}{
		{
			name:      "executor/** should find all files in executor",
			pattern:   "executor/**",
			recursive: true,
			wantCount: 2,
		},
		{
			name:      "executor/* should find files in executor",
			pattern:   "executor/*",
			recursive: true,
			wantCount: 2,
		},
		{
			name:      "*.go should find all go files",
			pattern:   "*.go",
			recursive: true,
			wantCount: 3,
		},
		{
			name:      "**/*.go should find all go files recursively",
			pattern:   "**/*.go",
			recursive: true,
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := globTool.searchFiles(tmpDir, tt.pattern, tt.recursive, 0)
			if err != nil {
				t.Fatalf("searchFiles() error = %v", err)
			}
			if len(files) != tt.wantCount {
				t.Errorf("searchFiles() found %d files, want %d: %v", len(files), tt.wantCount, files)
			}
		})
	}
}
