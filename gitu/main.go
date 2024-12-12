package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

var printMu sync.Mutex

const (
	notOnBranchMsg = iota
	hasUncommittedChangesMsg
	hasUnpushedCommitsMsg
	noRemoteUpdatesMsg
)

type Config struct {
	Branch      string
	Parallelism int
}

type repoStatusStruct struct {
	notOnBranch        []string
	uncommittedChanges []string
	unpushedCommits    []string
	updatedRepos       []string
	noUpdates          []string
}

func parseFlags() *Config {
	branchName := flag.String("b", "master", "Branch name to check and update")
	parallelism := flag.Int("p", runtime.NumCPU()*10, "Number of parallel workers")

	flag.Parse()

	return &Config{
		Branch:      *branchName,
		Parallelism: *parallelism,
	}
}

func main() {
	config := parseFlags()
	log.SetFlags(log.LstdFlags)

	currentDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get current directory: %v", err)
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, config.Parallelism)

	repoStatus := repoStatusStruct{
		notOnBranch:        []string{},
		uncommittedChanges: []string{},
		unpushedCommits:    []string{},
		updatedRepos:       []string{},
		noUpdates:          []string{},
	}

	var mu sync.Mutex

	err = filepath.Walk(currentDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() && filepath.Base(path) == ".git" {
			repoPath := filepath.Dir(path)

			wg.Add(1)
			sem <- struct{}{}
			go func(repoPath string) {
				defer wg.Done()
				processRepository(repoPath, config.Branch, &repoStatus, &mu)
				<-sem
			}(repoPath)

			return filepath.SkipDir
		}

		return nil
	})

	wg.Wait()

	if err != nil {
		log.Printf("Error during directory traversal: %v", err)
	}

	printResults(config.Branch, repoStatus)
}

func processRepository(repoPath, branchName string, repoStatus *repoStatusStruct, mu *sync.Mutex) {
	projectName := filepath.Base(repoPath)
	checks := []struct {
		fail func(string, string) bool
		msg  int
	}{
		{func(p, b string) bool { return notOnBranch(p, b) }, notOnBranchMsg},
		{hasUncommittedChanges, hasUncommittedChangesMsg},
		{hasUnpushedCommits, hasUnpushedCommitsMsg},
		{noRemoteUpdates, noRemoteUpdatesMsg},
	}

	allChecksPassed := true

	for _, check := range checks {
		if check.fail(repoPath, branchName) {
			mu.Lock()
			switch check.msg {
			case notOnBranchMsg:
				repoStatus.notOnBranch = append(repoStatus.notOnBranch, projectName)
			case hasUncommittedChangesMsg:
				repoStatus.uncommittedChanges = append(repoStatus.uncommittedChanges, projectName)
			case hasUnpushedCommitsMsg:
				repoStatus.unpushedCommits = append(repoStatus.unpushedCommits, projectName)
			case noRemoteUpdatesMsg:
				repoStatus.noUpdates = append(repoStatus.noUpdates, projectName)
			}
			mu.Unlock()
			allChecksPassed = false
		}
	}

	if allChecksPassed && gitPull(repoPath) {
		mu.Lock()
		repoStatus.updatedRepos = append(repoStatus.updatedRepos, projectName)
		mu.Unlock()
	}
}

func notOnBranch(repoPath, branchName string) bool {
	return executeGitCommand(repoPath, "rev-parse", "--abbrev-ref", "HEAD") != branchName
}

func hasUncommittedChanges(repoPath, branchName string) bool {
	return executeGitCommand(repoPath, "status", "--porcelain") != ""
}

func hasUnpushedCommits(repoPath, branchName string) bool {
	return executeGitCommand(repoPath, "cherry", "-v") != ""
}

func noRemoteUpdates(repoPath, branchName string) bool {
	if executeGitCommand(repoPath, "fetch") == "" {
		return true
	}
	return strings.Contains(executeGitCommand(repoPath, "status", "-uno"), "Your branch is up to date")
}

func gitPull(repoPath string) bool {
	projectName := filepath.Base(repoPath)

	cmd := exec.Command("git", "-C", repoPath, "pull")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout, cmd.Stderr = &outBuf, &errBuf

	if err := cmd.Run(); err != nil {
		log.Printf("Failed to pull repository %s: %v\n%s", repoPath, err, errBuf.String())
		return false
	}

	printMu.Lock()
	defer printMu.Unlock()

	fmt.Printf("-------------------------------------------- [ git pull: %s ] --------------------------------------------\n", projectName)
	fmt.Print(outBuf.String())

	return true
}

func executeGitCommand(repoPath string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", repoPath}, args...)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if err != nil {
		log.Printf("Git command failed: %s\n%s", cmd.String(), stderr.String())
		return ""
	}

	return strings.TrimSpace(string(output))
}

func printResults(branchName string, status repoStatusStruct) {
	printList("Repositories not on branch "+branchName+", skipping:", status.notOnBranch)
	printList("Repositories with uncommitted changes, skipping:", status.uncommittedChanges)
	printList("Repositories with unpushed commits, skipping:", status.unpushedCommits)
	printList("Repositories with no updates remotely, skipping:", status.noUpdates)
	printList("Repositories successfully updated:", status.updatedRepos)
}

func printList(header string, items []string) {
	if len(items) > 0 {
		fmt.Printf("\n%s\n- %s\n", header, strings.Join(items, ", "))
	}
}
