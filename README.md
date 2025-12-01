# DocHelper - Git file modification time tool

A tool for getting file modification times from Git and adjusting file system times or generating documents.

### Usage examples

#### 1. Generate JSON document

- Windows
```cmd
dochelper.exe . document file_times.json
```

- Linux/macOS
``` bash
dochelper ./ document ./file_times.json
```

#### 2. Adjust file system times

- Windows
```cmd
dochelper.exe . adjust
```

- Linux/macOS
``` bash
dochelper ./ adjust
```

### Output format description

#### JSON format (`.json`)
```json
[
  {
    "path": "main.go",
    "last_modified": "2024-01-15T10:30:00Z",
    "unix_time": 1705315800
  }
]
```

#### CSV format (`.csv`)
```csv
path,last_modified,unix_time
main.go,2024-01-15 10:30:00,1705315800
```

### Notes

1. **Git repository requirement**: The target directory must be a Git repository (containing `.git` directory)
2. **File tracking**: Only files tracked in Git will be processed, files not tracked will be skipped
3. **Permission requirements**:
   - Document mode: requires write permission
   - Adjust mode: requires permission to modify file time (may require administrator permissions)
