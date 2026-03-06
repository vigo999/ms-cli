package permission

import (
	"testing"
)

func TestCompileWildcard(t *testing.T) {
	tests := []struct {
		pattern string
		match   string
		want    bool
	}{
		// 精确匹配
		{"exact", "exact", true},
		{"exact", "wrong", false},

		// 前缀匹配
		{"/home/*", "/home/user", true},
		{"/home/*", "/root/user", false},

		// 后缀匹配
		{"*.go", "main.go", true},
		{"*.go", "main.py", false},

		// 包含匹配
		{"*test*", "unit_test.go", true},
		{"*test*", "main.go", false},

		// 单字符匹配
		{"?at", "cat", true},
		{"?at", "bat", true},
		{"?at", "at", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.match, func(t *testing.T) {
			matcher := CompileWildcard(tt.pattern)
			got := matcher.Match(tt.match)
			if got != tt.want {
				t.Errorf("CompileWildcard(%q).Match(%q) = %v, want %v",
					tt.pattern, tt.match, got, tt.want)
			}
		})
	}
}

func TestMatchAny(t *testing.T) {
	patterns := []string{"*.go", "*.js", "*.ts"}

	if !MatchAny("main.go", patterns) {
		t.Error("MatchAny(main.go) should be true")
	}

	if !MatchAny("script.js", patterns) {
		t.Error("MatchAny(script.js) should be true")
	}

	if MatchAny("style.css", patterns) {
		t.Error("MatchAny(style.css) should be false")
	}
}

func TestMatchAll(t *testing.T) {
	if !MatchAll("test", []string{}) {
		t.Error("MatchAll with empty patterns should be true")
	}

	patterns := []string{"*test*", "*.go"}

	if !MatchAll("unit_test.go", patterns) {
		t.Error("MatchAll(unit_test.go) should be true")
	}

	if MatchAll("main.go", patterns) {
		t.Error("MatchAll(main.go) should be false")
	}
}

func TestWildcardCache(t *testing.T) {
	// First compile
	m1 := CompileWildcard("/home/*")
	m2 := CompileWildcard("/home/*")

	// Should be the same object (cached)
	if m1 != m2 {
		t.Error("CompileWildcard should return cached matcher")
	}

	// Get cache size
	size := GetMatcherCacheSize()
	if size < 1 {
		t.Error("Cache size should be at least 1")
	}

	// Clear cache
	ClearMatcherCache()

	// After clear, size should be 0
	size = GetMatcherCacheSize()
	if size != 0 {
		t.Errorf("Cache size after clear = %d, want 0", size)
	}
}

func BenchmarkWildcardMatch(b *testing.B) {
	matcher := CompileWildcard("/home/user/*")
	target := "/home/user/documents/file.txt"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matcher.Match(target)
	}
}

func BenchmarkWildcardCompile(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CompileWildcard("/home/user/*")
	}
}
