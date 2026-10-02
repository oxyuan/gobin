package main

import (
	"bufio"
	"flag"
	"fmt"
	"github.com/dlclark/regexp2"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// Config 结构体集中管理命令行参数和配置信息
type Config struct {
	FilePattern        string   `json:"filePattern"`
	SearchPattern      string   `json:"searchPattern"`
	SearchRegexPattern string   `json:"searchRegexPattern"`
	ExclusionPaths     []string `json:"exclusionPaths"`
	Module             int      `json:"module"`
	Parallelism        int      `json:"parallelism"`
	SearchPath         string   `json:"searchPath"`
	CaseSensitive      bool     `json:"caseSensitive"`
	ContextLines       int      `json:"contextLines"`
}

// SearchResult represents a single search result.
type SearchResult struct {
	FilePath    string   `json:"filePath"`
	LineNumber  int      `json:"lineNumber"`
	LineContent string   `json:"lineContent"`
	Context     []string `json:"context"`
}

func main() {
	// 解析并校验配置
	config := parseAndValidateFlags()

	// 打印搜索信息
	printConfig(config)

	// 执行文件搜索
	results := SearchFiles(config)

	// 打印搜索结果
	printResults(results)
}

// parseAndValidateFlags 解析命令行参数并校验
func parseAndValidateFlags() *Config {
	filePattern := flag.String("f", "prod.yml$", "The file pattern to search for (regex)")
	searchPattern := flag.String("s", "", "The string pattern to search within files (mutually exclusive with -ss)")
	searchRegexPattern := flag.String("ss", "", "The regex pattern to search within files (mutually exclusive with -s)")
	exclusionPaths := flag.String("e", "target", "Directory path to exclude from search")
	module := flag.Int("m", 0, "Override file pattern")
	parallelism := flag.Int("P", runtime.NumCPU()*10, "10*Number of parallel workers")
	caseSensitive := flag.Bool("c", false, "Case sensitive search")
	contextLines := flag.Int("l", 0, "Context lines to display")
	flag.Parse()

	// 参数校验
	if *searchPattern == "" && *searchRegexPattern == "" {
		log.Fatalf("Error: You must provide either -s or -ss argument.\n")
	}
	if *searchPattern != "" && *searchRegexPattern != "" {
		log.Fatalf("Error: -s and -ss are mutually exclusive.\n")
	}

	searchPath := "."
	if len(flag.Args()) > 0 {
		searchPath = flag.Args()[0]
		if _, err := os.Stat(searchPath); os.IsNotExist(err) {
			log.Fatalf("Error: Search path %s does not exist.\n", searchPath)
		}
	}

	return &Config{
		FilePattern:        setFilePattern(*filePattern, *module),
		SearchPattern:      *searchPattern,
		SearchRegexPattern: *searchRegexPattern,
		ExclusionPaths:     strings.Split(*exclusionPaths, ","),
		Module:             *module,
		Parallelism:        *parallelism,
		SearchPath:         filepath.FromSlash(searchPath),
		CaseSensitive:      *caseSensitive,
		ContextLines:       *contextLines,
	}
}

// setFilePattern 根据 -m 参数设置文件匹配模式
func setFilePattern(filePattern string, module int) string {
	modulePatterns := map[int]string{
		1: `\.java$`,
		2: `\.yml$`,
		3: `\.yaml$`,
		4: `\.xml$`,
		5: `\.txt$`,
		6: `\.properties$`,
		7: `\.json$`,
		8: `\.py$`,
		9: `\.php$`,
	}
	if pattern, exists := modulePatterns[module]; exists {
		return pattern
	}
	return filePattern
}

// createMatcher 创建搜索匹配器
func createMatcher(config *Config) func(string) bool {
	if config.SearchPattern != "" {
		if config.CaseSensitive {
			return func(line string) bool {
				return strings.Contains(line, config.SearchPattern)
			}
		} else {
			return func(line string) bool {
				return strings.Contains(strings.ToLower(line), strings.ToLower(config.SearchPattern))
			}
		}
	}

	regex := regexp2.MustCompile(config.SearchRegexPattern, regexp2.None)
	return func(line string) bool {
		if match, err := regex.MatchString(line); err == nil {
			return match
		}
		return false
	}
}

// printConfig 打印配置信息
func printConfig(config *Config) {
	fmt.Printf("Searching in: \t\t%s\n", config.SearchPath)
	fmt.Printf("Max parallelism: \t%d\n", config.Parallelism)
	fmt.Printf("Excluding: \t\t%s\n", config.ExclusionPaths)
	fmt.Printf("File pattern: \t\t%s\n", config.FilePattern)
	fmt.Printf("Case sensitive: \t%t\n", config.CaseSensitive)
	fmt.Printf("Context Lines: \t\t%d\n", config.ContextLines)
	if config.SearchPattern != "" {
		fmt.Printf("Search value: \t\t%s\n\n", config.SearchPattern)
	} else {
		fmt.Printf("Search regex: \t\t%s\n\n", config.SearchRegexPattern)
	}
}

func printResults(results []SearchResult) {
	for _, result := range results {
		fmt.Printf("%s:%d\t\t%s\n", result.FilePath, result.LineNumber, result.LineContent)
		if len(result.Context) > 0 {
			fmt.Println("Context:")
			for _, line := range result.Context {
				fmt.Println(line)
			}
		}
	}
}

// SearchFiles 执行文件搜索
func SearchFiles(config *Config) []SearchResult {
	var results []SearchResult
	regex := regexp2.MustCompile(config.FilePattern, regexp2.None)

	sem := make(chan struct{}, config.Parallelism)
	var wg sync.WaitGroup
	var mu sync.Mutex

	err := filepath.WalkDir(config.SearchPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Check for exclusion paths
		isExcluded := false
		for _, exclusionPath := range config.ExclusionPaths {
			if strings.Contains(path, exclusionPath) {
				isExcluded = true
				break
			}
		}
		if d.IsDir() || isExcluded {
			return nil
		}

		if isMatch, err := regex.MatchString(d.Name()); err != nil || !isMatch {
			return nil
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(path string) {
			defer wg.Done()
			fileResults := searchInFile(path, config)
			mu.Lock()
			results = append(results, fileResults...)
			mu.Unlock()
			<-sem
		}(path)

		return nil
	})

	wg.Wait()
	if err != nil {
		log.Printf("Error while walking the path: %v\n", err)
	}

	return results
}

// searchInFile 搜索文件内容中符合模式的行
func searchInFile(path string, config *Config) []SearchResult {
	var results []SearchResult
	file, err := os.Open(path)
	if err != nil {
		log.Printf("Error opening file %s: %v\n", path, err)
		return results
	}
	defer file.Close()

	path = "./" + strings.ReplaceAll(path, "\\", "/")

	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		lines = append(lines, line)
	}

	matcher := createMatcher(config)
	for i, line := range lines {
		if matcher(line) {
			context := getContext(lines, i, config.ContextLines)
			result := SearchResult{
				FilePath:    path,
				LineNumber:  i + 1,
				LineContent: line,
				Context:     context,
			}
			results = append(results, result)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading file %s: %v\n", path, err)
	}
	return results
}

// getContext returns the context lines around the matched line.
func getContext(lines []string, matchedLineIndex, contextLines int) []string {
	var context []string
	start := matchedLineIndex - contextLines
	end := matchedLineIndex + contextLines
	if start < 0 {
		start = 0
	}
	if end >= len(lines) {
		end = len(lines) - 1
	}
	for i := start; i <= end; i++ {
		context = append(context, fmt.Sprintf("%d: %s", i+1, lines[i]))
	}
	return context
}
