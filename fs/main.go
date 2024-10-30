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
	// 定义和解析命令行参数
	filePattern := flag.String("f", "prod.yml$", "[file] The file pattern to search for (regex)")
	searchPattern := flag.String("s", "", "[search] The regex pattern to search within files (required)")
	exclusionPath := flag.String("e", "target", "[exclusion] Directory path to exclude from search")
	//parallelism := flag.Int("P", runtime.NumCPU()*10, "[parallel] Number of parallel workers")
	module := flag.Int("m", 0, "[module] Override file pattern (1 for .java$, 2 for .yml$, 3 for .yaml$, 4 for .xml$)")

	// 解析命令行参数
	flag.Parse()

	// 根据 m 参数设置 filePattern
	switch *module {
	case 1:
		*filePattern = `\.java$`
	case 2:
		*filePattern = `\.yml$`
	case 3:
		*filePattern = `\.yaml$`
	case 4:
		*filePattern = `\.xml$`
	default:
		// 使用默认值
	}

	// 检查必需参数
	if *searchPattern == "" {
		fmt.Println("Error: -s or -search is required")
		flag.Usage()
		os.Exit(1)
	}

	// 编译正则表达式
	regex, err := regexp.Compile(*searchPattern)
	if err != nil {
		fmt.Printf("Invalid regex pattern: %v\n", err)
		return
	}

	// 获取查询路径
	searchPath := "."
	if len(flag.Args()) > 0 {
		searchPath = flag.Args()[0]
	}

	// 检查查询路径是否存在
	if _, err := os.Stat(searchPath); os.IsNotExist(err) {
		fmt.Printf("Error: search path %s does not exist\n", searchPath)
		os.Exit(1)
	}

	// 适配路径分隔符
	*exclusionPath = filepath.FromSlash(*exclusionPath)

	// 用于控制并发度的通道
	//sem := make(chan struct{}, *parallelism)

	// 使用 WaitGroup 和 goroutine 实现并发
	var wg sync.WaitGroup
	err = filepath.Walk(searchPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 跳过包含排除路径的文件夹
		if strings.Contains(path, *exclusionPath) {
			return nil
		}

		// 检查文件名是否匹配指定的正则表达式模式
		matched, err := regexp.MatchString(*filePattern, info.Name())
		if err != nil || !matched || info.IsDir() {
			return nil
		}

		// 读取并搜索文件内容
		//sem <- struct{}{} // 占用一个并发槽
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			searchInFile(path, regex)
			//<-sem // 释放并发槽
		}(path)

		return nil
	})

	// 等待所有 goroutine 完成
	wg.Wait()

	if err != nil {
		fmt.Printf("Error while walking the path: %v\n", err)
	}
}

// 搜索文件内容中符合正则表达式的行，并按要求输出
func searchInFile(path string, regex *regexp.Regexp) {
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
		if regex.MatchString(line) {
			// 按要求格式化输出：路径和搜索结果之间使用 \t 分隔
			fmt.Printf("%s\t\t%s\n", path, line)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Printf("Error reading file %s: %v\n", path, err)
	}
}
