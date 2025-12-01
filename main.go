package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type FileModTime struct {
	Path         string    `json:"path"`
	LastModified time.Time `json:"last_modified"`
	UnixTime     int64     `json:"unix_time"`
}

type DocHelper struct {
	TargetDir string
	Output    string
	Mode      string
}

func NewDocHelper(targetDir, output, mode string) *DocHelper {
	return &DocHelper{
		TargetDir: targetDir,
		Output:    output,
		Mode:      mode,
	}
}

func (dh *DocHelper) GetGitLastModified(filePath string) (time.Time, error) {
	relPath, err := filepath.Rel(dh.TargetDir, filePath)
	if err != nil {
		return time.Time{}, err
	}

	cmd := exec.Command("git", "log", "-1", "--format=%ct", "--", relPath)
	cmd.Dir = dh.TargetDir
	output, err := cmd.Output()
	if err != nil {
		return time.Time{}, nil
	}

	timestampStr := strings.TrimSpace(string(output))
	if timestampStr == "" {
		return time.Time{}, nil
	}

	var timestamp int64
	fmt.Sscanf(timestampStr, "%d", &timestamp)
	return time.Unix(timestamp, 0), nil
}

func (dh *DocHelper) ScanDirectory() ([]FileModTime, error) {
	var files []FileModTime

	err := filepath.Walk(dh.TargetDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			if info.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		lastModified, err := dh.GetGitLastModified(path)
		if err != nil {
			fmt.Printf("Error: cannot get git modified time of %s: %v\n", path, err)
			return nil
		}

		if lastModified.IsZero() {
			return nil
		}

		relPath, _ := filepath.Rel(dh.TargetDir, path)
		files = append(files, FileModTime{
			Path:         relPath,
			LastModified: lastModified,
			UnixTime:     lastModified.Unix(),
		})

		return nil
	})

	return files, err
}

func (dh *DocHelper) AdjustFileTimes(files []FileModTime) error {
	adjustedCount := 0
	errorCount := 0

	for _, file := range files {
		fullPath := filepath.Join(dh.TargetDir, file.Path)

		err := os.Chtimes(fullPath, file.LastModified, file.LastModified)
		if err != nil {
			fmt.Printf("Error: cannot adjust time of %s: %v\n", file.Path, err)
			errorCount++
			continue
		}

		fmt.Printf("Adjusted: %s -> %s\n", file.Path, file.LastModified.Format("2006-01-02 15:04:05"))
		adjustedCount++
	}

	fmt.Printf("\nCompleted: adjusted %d files, failed %d files\n", adjustedCount, errorCount)
	return nil
}

func (dh *DocHelper) GenerateDocument(files []FileModTime) error {
	sort.Slice(files, func(i, j int) bool {
		return files[i].LastModified.After(files[j].LastModified)
	})

	outputPath := dh.Output
	if outputPath == "" {
		outputPath = filepath.Join(dh.TargetDir, "file_modification_times.json")
	}

	ext := strings.ToLower(filepath.Ext(outputPath))

	switch ext {
	case ".json":
		return dh.generateJSONDocument(files, outputPath)
	case ".csv":
		return dh.generateCSVDocument(files, outputPath)
	case ".md", ".markdown":
		return dh.generateMarkdownDocument(files, outputPath)
	default:
		return dh.generateJSONDocument(files, outputPath)
	}
}

func (dh *DocHelper) generateJSONDocument(files []FileModTime, outputPath string) error {
	data, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot serialize JSON: %v", err)
	}

	err = os.WriteFile(outputPath, data, 0644)
	if err != nil {
		return fmt.Errorf("cannot write file: %v", err)
	}

	fmt.Printf("Generated JSON document: %s (total %d files)\n", outputPath, len(files))
	return nil
}

func (dh *DocHelper) generateCSVDocument(files []FileModTime, outputPath string) error {
	var builder strings.Builder
	builder.WriteString("path,last_modified,unix_time\n")

	for _, file := range files {
		builder.WriteString(fmt.Sprintf("%s,%s,%d\n",
			file.Path,
			file.LastModified.Format("2006-01-02 15:04:05"),
			file.UnixTime,
		))
	}

	err := os.WriteFile(outputPath, []byte(builder.String()), 0644)
	if err != nil {
		return fmt.Errorf("cannot write file: %v", err)
	}

	fmt.Printf("Generated CSV document: %s (total %d files)\n", outputPath, len(files))
	return nil
}

func (dh *DocHelper) generateMarkdownDocument(files []FileModTime, outputPath string) error {
	var builder strings.Builder
	builder.WriteString("# File modification times document\n\n")
	builder.WriteString(fmt.Sprintf("Generated time: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	builder.WriteString(fmt.Sprintf("Target directory: %s\n\n", dh.TargetDir))
	builder.WriteString(fmt.Sprintf("Total files: %d\n\n", len(files)))
	builder.WriteString("## File list\n\n")
	builder.WriteString("| File path | Last modified time | Unix time |\n")
	builder.WriteString("|---------|-------------|-----------|\n")

	for _, file := range files {
		builder.WriteString(fmt.Sprintf("| %s | %s | %d |\n",
			file.Path,
			file.LastModified.Format("2006-01-02 15:04:05"),
			file.UnixTime,
		))
	}

	err := os.WriteFile(outputPath, []byte(builder.String()), 0644)
	if err != nil {
		return fmt.Errorf("cannot write file: %v", err)
	}

	fmt.Printf("Generated Markdown document: %s (total %d files)\n", outputPath, len(files))
	return nil
}

func (dh *DocHelper) Run() error {
	if _, err := os.Stat(dh.TargetDir); os.IsNotExist(err) {
		return fmt.Errorf("target directory does not exist: %s", dh.TargetDir)
	}

	gitDir := filepath.Join(dh.TargetDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		return fmt.Errorf("target directory is not a git repository: %s", dh.TargetDir)
	}

	fmt.Printf("Scanning directory: %s\n", dh.TargetDir)
	fmt.Println("Getting file last modified time from git...")

	files, err := dh.ScanDirectory()
	if err != nil {
		return fmt.Errorf("scan directory failed: %v", err)
	}

	if len(files) == 0 {
		fmt.Println("Warning: no files found in git")
		return nil
	}

	fmt.Printf("Found %d files\n\n", len(files))

	switch dh.Mode {
	case "adjust":
		return dh.AdjustFileTimes(files)
	case "document":
		return dh.GenerateDocument(files)
	default:
		return fmt.Errorf("unknown mode: %s (supported modes: adjust, document)", dh.Mode)
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  DocHelper <directory path> <mode> [output file]")
		fmt.Println()
		fmt.Println("Modes:")
		fmt.Println("  adjust    - adjust file system times based on git last modified time")
		fmt.Println("  document  - generate file modification times document")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  DocHelper . document file_times.json")
		fmt.Println("  DocHelper . document file_times.md")
		fmt.Println("  DocHelper . document file_times.csv")
		fmt.Println("  DocHelper . adjust")
		os.Exit(1)
	}

	targetDir := os.Args[1]
	mode := os.Args[2]
	output := ""
	if len(os.Args) > 3 {
		output = os.Args[3]
	}

	absDir, err := filepath.Abs(targetDir)
	if err != nil {
		fmt.Printf("Error: cannot parse directory path: %v\n", err)
		os.Exit(1)
	}

	helper := NewDocHelper(absDir, output, mode)
	if err := helper.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
