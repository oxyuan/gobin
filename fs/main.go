package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

// Config 结构体集中管理命令行参数和配置信息
type Config struct {
	FilePattern        string
	SearchPattern      string
	SearchRegexPattern string
	ExclusionPath      string
	Module             int
	Parallelism        int
	SearchPath         string
}

func main() {
	// 解析并校验配置
	config := parseAndValidateFlags()

	// 打印搜索信息
	printConfig(config)

	// 创建匹配器
	matcher := createMatcher(config)

	// 执行文件搜索
	walkDirectory(config, matcher)
}

// parseAndValidateFlags 解析命令行参数并校验
func parseAndValidateFlags() *Config {
	filePattern := flag.String("f", "prod.yml$", "The file pattern to search for (regex)")
	searchPattern := flag.String("s", "", "The string pattern to search within files (mutually exclusive with -ss)")
	searchRegexPattern := flag.String("ss", "", "The regex pattern to search within files (mutually exclusive with -s)")
	exclusionPath := flag.String("e", "target", "Directory path to exclude from search")
	module := flag.Int("m", 0, "Override file pattern")
	parallelism := flag.Int("P", runtime.NumCPU()*10, "10*Number of parallel workers")

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
		ExclusionPath:      filepath.FromSlash(*exclusionPath),
		Module:             *module,
		Parallelism:        *parallelism,
		SearchPath:         filepath.FromSlash(searchPath),
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
		return func(line string) bool {
			return strings.Contains(line, config.SearchPattern)
		}
	}

	regex, err := regexp.Compile(config.SearchRegexPattern)
	if err != nil {
		log.Fatalf("Invalid regex pattern: %v\n", err)
	}
	return func(line string) bool {
		return regex.MatchString(line)
	}
}

// printConfig 打印配置信息
func printConfig(config *Config) {
	fmt.Printf("Searching in: \t\t%s\n", config.SearchPath)
	fmt.Printf("Max parallelism: \t%d\n", config.Parallelism)
	fmt.Printf("Excluding: \t\t%s\n", config.ExclusionPath)
	fmt.Printf("File pattern: \t\t%s\n", config.FilePattern)
	if config.SearchPattern != "" {
		fmt.Printf("Search value: \t\t%s\n\n", config.SearchPattern)
	} else {
		fmt.Printf("Search regex: \t\t%s\n\n", config.SearchRegexPattern)
	}
}

// walkDirectory 遍历目录并执行文件内容搜索
func walkDirectory(config *Config, matcher func(string) bool) {
	regex, err := regexp.Compile(config.FilePattern)
	if err != nil {
		log.Fatalf("Invalid file pattern regex: %v\n", err)
	}

	sem := make(chan struct{}, config.Parallelism)
	var wg sync.WaitGroup

	err = filepath.WalkDir(config.SearchPath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || strings.Contains(path, config.ExclusionPath) || !regex.MatchString(d.Name()) {
			return nil
		}

		wg.Add(1)
		sem <- struct{}{}
		go func(path string) {
			defer wg.Done()
			searchInFile(path, matcher)
			<-sem
		}(path)

		return nil
	})

	wg.Wait()
	if err != nil {
		log.Printf("Error while walking the path: %v\n", err)
	}
}

// searchInFile 搜索文件内容中符合模式的行
func searchInFile(path string, matcher func(string) bool) {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("Error opening file %s: %v\n", path, err)
		return
	}
	defer file.Close()

	path = filepath.ToSlash(path)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r\n")
		if matcher(line) {
			fmt.Printf("%s\t\t%s\n", path, line)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("Error reading file %s: %v\n", path, err)
	}
}
