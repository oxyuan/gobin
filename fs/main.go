package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

func main() {
	// 定义命令行参数
	filePattern, searchPattern, searchPatternSS, exclusionPath, module := parseFlags()

	// 设置文件匹配模式
	filePattern = setFilePattern(filePattern, module)

	// 确保 -s 和 -ss 参数的互斥性
	validateSearchPatterns(searchPattern, searchPatternSS)

	// 编译搜索模式
	var matcher func(string) bool
	if searchPattern != "" {
		matcher = func(line string) bool { return strings.Contains(line, searchPattern) }
	} else {
		matcher = compileRegexMatcher(searchPatternSS)
		searchPattern = searchPatternSS
	}

	// 获取并验证搜索路径
	searchPath := getSearchPath()

	// 打印搜索路径、排除路径、文件匹配模式、搜索字符
	fmt.Printf("Searching in: \t%s\nExcluding: \t%s\nFile pattern: \t%s\nSearch value: \t%s\n\n", searchPath, exclusionPath, filePattern, searchPattern)

	// 执行文件遍历与搜索
	walkDirectory(searchPath, filePattern, matcher, exclusionPath)
}

// parseFlags 解析命令行参数
func parseFlags() (string, string, string, string, int) {
	filePattern := flag.String("f", "prod.yml$", "[file] The file pattern to search for (regex)")
	searchPattern := flag.String("s", "", "[search] The string pattern to search within files (mutually exclusive with -ss, required)")
	searchPatternSS := flag.String("ss", "", "[search-regex] The regex pattern to search within files (mutually exclusive with -s, required)")
	exclusionPath := flag.String("e", "target", "[exclusion] Directory path to exclude from search")
	module := flag.Int("m", 0, "[module] Override file pattern (1 for .java$, 2 for .yml$, 3 for .yaml$, 4 for .xml$, 5 for .txt$, 6 for .properties$, 7 for .json$, 8 for .py$, 9 for .php$)")

	flag.Parse()

	return *filePattern, *searchPattern, *searchPatternSS, *exclusionPath, *module
}

// setFilePattern 根据 -m 参数设置文件匹配模式
func setFilePattern(filePattern string, module int) string {
	switch module {
	case 1:
		return `\.java$`
	case 2:
		return `\.yml$`
	case 3:
		return `\.yaml$`
	case 4:
		return `\.xml$`
	case 5:
		return `\.txt$`
	case 6:
		return `\.properties$`
	case 7:
		return `\.json$`
	case 8:
		return `\.py$`
	case 9:
		return `\.php$`
	}
	return filePattern
}

// validateSearchPatterns 检查 -s 和 -ss 参数的互斥性
func validateSearchPatterns(searchPattern, searchPatternSS string) {
	if searchPattern == "" && searchPatternSS == "" {
		fmt.Println("Error: You must provide either -s or -ss argument, but not both.")
		flag.Usage()
		os.Exit(1)
	}
	if searchPattern != "" && searchPatternSS != "" {
		fmt.Println("Error: -s and -ss are mutually exclusive. Please provide only one.")
		flag.Usage()
		os.Exit(1)
	}
}

// compileRegexMatcher 编译正则表达式匹配器
func compileRegexMatcher(searchPatternSS string) func(string) bool {
	regex, err := regexp.Compile(searchPatternSS)
	if err != nil {
		fmt.Printf("Invalid regex pattern: %v\n", err)
		os.Exit(1)
	}
	return func(line string) bool { return regex.MatchString(line) }
}

// getSearchPath 获取搜索路径并验证
func getSearchPath() string {
	searchPath := "."
	if len(flag.Args()) > 0 {
		searchPath = flag.Args()[0]
	}
	if _, err := os.Stat(searchPath); os.IsNotExist(err) {
		fmt.Printf("Error: search path %s does not exist\n", searchPath)
		os.Exit(1)
	}
	return searchPath
}

// walkDirectory 遍历目录并进行文件搜索
func walkDirectory(searchPath, filePattern string, matcher func(string) bool, exclusionPath string) {
	// 适配路径分隔符
	exclusionPath = filepath.FromSlash(exclusionPath)

	var wg sync.WaitGroup
	err := filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过包含排除路径的文件夹
		if strings.Contains(path, exclusionPath) {
			return nil
		}

		// 检查文件名是否匹配指定的模式
		if matched, _ := regexp.MatchString(filePattern, info.Name()); !matched || info.IsDir() {
			return nil
		}

		// 并发处理文件内容搜索
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			searchInFile(path, matcher)
		}(path)

		return nil
	})

	// 等待所有 goroutine 完成
	wg.Wait()

	if err != nil {
		fmt.Printf("Error while walking the path: %v\n", err)
	}
}

// searchInFile 搜索文件内容中符合模式的行
func searchInFile(path string, matcher func(string) bool) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Printf("Error opening file %s: %v\n", path, err)
		return
	}
	defer file.Close()

	// 格式化路径
	path = "./" + strings.ReplaceAll(path, "\\", "/")

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if matcher(line) {
			// 输出匹配结果
			fmt.Printf("%s\t\t%s\n", path, line)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading file %s: %v\n", path, err)
	}
}
