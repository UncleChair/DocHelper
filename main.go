package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
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

	// Display file information like adjust mode
	for _, file := range files {
		fmt.Printf("Documented: %s -> %s\n", file.Path, file.LastModified.Format("2006-01-02 15:04:05"))
	}

	fmt.Println()

	ext := strings.ToLower(filepath.Ext(outputPath))

	switch ext {
	case ".json":
		return dh.generateJSONDocument(files, outputPath)
	case ".csv":
		return dh.generateCSVDocument(files, outputPath)
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

func (dh *DocHelper) ReadFromJSON(inputPath string) ([]FileModTime, error) {
	data, err := os.ReadFile(inputPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read file: %v", err)
	}

	var files []FileModTime
	err = json.Unmarshal(data, &files)
	if err != nil {
		return nil, fmt.Errorf("cannot parse JSON: %v", err)
	}

	// Make sure UnixTime field is correct
	for i := range files {
		if files[i].UnixTime == 0 && !files[i].LastModified.IsZero() {
			files[i].UnixTime = files[i].LastModified.Unix()
		}
	}

	return files, nil
}

func (dh *DocHelper) ReadFromCSV(inputPath string) ([]FileModTime, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("cannot read CSV: %v", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file is empty or missing header")
	}

	var files []FileModTime
	for i := 1; i < len(records); i++ {
		record := records[i]
		if len(record) < 3 {
			continue
		}

		path := record[0]
		lastModifiedStr := record[1]
		unixTimeStr := record[2]

		unixTime, err := strconv.ParseInt(unixTimeStr, 10, 64)
		if err != nil {
			lastModified, err := time.Parse("2006-01-02 15:04:05", lastModifiedStr)
			if err != nil {
				lastModified, err = time.Parse(time.RFC3339, lastModifiedStr)
				if err != nil {
					fmt.Printf("Warning: cannot parse time for %s: %v\n", path, err)
					continue
				}
			}
			unixTime = lastModified.Unix()
			files = append(files, FileModTime{
				Path:         path,
				LastModified: lastModified,
				UnixTime:     unixTime,
			})
		} else {
			lastModified := time.Unix(unixTime, 0)
			files = append(files, FileModTime{
				Path:         path,
				LastModified: lastModified,
				UnixTime:     unixTime,
			})
		}
	}

	return files, nil
}

func (dh *DocHelper) RestoreFromFile(inputPath string) error {
	if _, err := os.Stat(inputPath); os.IsNotExist(err) {
		return fmt.Errorf("input file does not exist: %s", inputPath)
	}

	if _, err := os.Stat(dh.TargetDir); os.IsNotExist(err) {
		return fmt.Errorf("target directory does not exist: %s", dh.TargetDir)
	}

	ext := strings.ToLower(filepath.Ext(inputPath))
	var files []FileModTime
	var err error

	fmt.Printf("Reading from file: %s\n", inputPath)
	switch ext {
	case ".json":
		files, err = dh.ReadFromJSON(inputPath)
	case ".csv":
		files, err = dh.ReadFromCSV(inputPath)
	default:
		return fmt.Errorf("unsupported file format: %s (supported: .json, .csv)", ext)
	}

	if err != nil {
		return fmt.Errorf("cannot read file: %v", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no file data found in input file")
	}

	fmt.Printf("Loaded %d files from %s\n\n", len(files), inputPath)
	return dh.AdjustFileTimes(files)
}

func (dh *DocHelper) Run() error {
	switch dh.Mode {
	case "restore":
		if dh.Output == "" {
			return fmt.Errorf("restore mode requires an input file path")
		}
		return dh.RestoreFromFile(dh.Output)
	case "adjust", "document":
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

		if dh.Mode == "adjust" {
			return dh.AdjustFileTimes(files)
		}
		return dh.GenerateDocument(files)
	default:
		return fmt.Errorf("unknown mode: %s (supported modes: adjust, document, restore)", dh.Mode)
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Println("Usage:")
		fmt.Println("  DocHelper <directory path> <mode> [output/input file]")
		fmt.Println()
		fmt.Println("Modes:")
		fmt.Println("  adjust    - adjust file system times based on git last modified time")
		fmt.Println("  document  - generate file modification times document")
		fmt.Println("  restore   - restore file times from JSON or CSV file")
		fmt.Println()
		fmt.Println("Examples:")
		fmt.Println("  DocHelper . document file_times.json")
		fmt.Println("  DocHelper . document file_times.csv")
		fmt.Println("  DocHelper . adjust")
		fmt.Println("  DocHelper . restore file_times.json")
		fmt.Println("  DocHelper . restore file_times.csv")
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

	if mode == "restore" && output != "" {
		absOutput, err := filepath.Abs(output)
		if err == nil {
			output = absOutput
		}
	}

	helper := NewDocHelper(absDir, output, mode)
	if err := helper.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
