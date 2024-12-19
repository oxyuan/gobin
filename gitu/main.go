package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

type Config struct {
	Branch      string
	Parallelism int
}

type RepoStatus struct {
	NotOnBranch        []string
	UncommittedChanges []string
	UnpushedCommits    []string
	UpdatedRepos       []string
	NoUpdates          []string
}

func main() {
	config := parseFlags()
	currentDir := getCurrentDir()

	repoStatus := RepoStatus{}
	processRepos(currentDir, config, &repoStatus)
	printResults(config.Branch, repoStatus)
}

func parseFlags() *Config {
	branch := flag.String("b", "master", "Branch name to check and update")
	parallelism := flag.Int("p", runtime.NumCPU()*10, "Parallelism level")
	flag.Parse()
	return &Config{*branch, *parallelism}
}

func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}
	return dir
}

func processRepos(baseDir string, config *Config, repoStatus *RepoStatus) {
	var wg sync.WaitGroup
	sem := make(chan struct{}, config.Parallelism)
	var mu sync.Mutex

	err := filepath.Walk(baseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && filepath.Base(path) == ".git" {
			repoPath := filepath.Dir(path)
			sem <- struct{}{}
			wg.Add(1)
			go func() {
				defer wg.Done()
				processRepo(repoPath, config.Branch, repoStatus, &mu)
				<-sem
			}()
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		log.Printf("Error walking directories: %v", err)
	}
	wg.Wait()
}

func processRepo(repoPath, branch string, repoStatus *RepoStatus, mu *sync.Mutex) {
	projectName := filepath.Base(repoPath)
	checks := []struct {
		Check func(string) bool
		List  *[]string
	}{
		{notOnBranch(branch), &repoStatus.NotOnBranch},
		{hasUncommittedChanges(), &repoStatus.UncommittedChanges},
		{hasUnpushedCommits(), &repoStatus.UnpushedCommits},
		{noRemoteUpdates(), &repoStatus.NoUpdates},
	}

	allPassed := true
	for _, check := range checks {
		if check.Check(repoPath) {
			mu.Lock()
			*check.List = append(*check.List, projectName)
			mu.Unlock()
			allPassed = false
		}
	}
	if allPassed && gitPull(repoPath) {
		mu.Lock()
		repoStatus.UpdatedRepos = append(repoStatus.UpdatedRepos, projectName)
		mu.Unlock()
	}
}

// 动态生成具体的检查函数
func notOnBranch(branch string) func(repoPath string) bool {
	return func(repoPath string) bool {
		return runGitCommand(repoPath, "rev-parse", "--abbrev-ref", "HEAD") != branch
	}
}

func hasUncommittedChanges() func(repoPath string) bool {
	return func(repoPath string) bool {
		return runGitCommand(repoPath, "status", "--porcelain") != ""
	}
}

func hasUnpushedCommits() func(repoPath string) bool {
	return func(repoPath string) bool {
		return runGitCommand(repoPath, "cherry", "-v") != ""
	}
}

func noRemoteUpdates() func(repoPath string) bool {
	return func(repoPath string) bool {
		return strings.Contains(runGitCommand(repoPath, "status", "-uno"), "up to date")
	}
}

func gitPull(repoPath string) bool {
	projectName := filepath.Base(repoPath)
	if out, err := exec.Command("git", "-C", repoPath, "pull").CombinedOutput(); err != nil {
		log.Printf("Failed to pull %s: %v", projectName, err)
		return false
	} else {
		log.Printf("Pulled %s:\n%s", projectName, out)
		return true
	}
}

func runGitCommand(repoPath string, args ...string) string {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)
	if out, err := cmd.Output(); err == nil {
		return strings.TrimSpace(string(out))
	}
	return ""
}

func printResults(branch string, repoStatus RepoStatus) {
	printList("Repositories not on branch "+branch, repoStatus.NotOnBranch)
	printList("Repositories with uncommitted changes", repoStatus.UncommittedChanges)
	printList("Repositories with unpushed commits", repoStatus.UnpushedCommits)
	printList("Repositories with no remote updates", repoStatus.NoUpdates)
	printList("Repositories updated", repoStatus.UpdatedRepos)
}

func printList(header string, items []string) {
	if len(items) > 0 {
		fmt.Printf("\n%s:\n%s\n", header, strings.Join(items, ", "))
	}
}
