package dbbackup

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func RunBackup(dbPath, backupDir string) (string, error) {
	src := dbPath

	if err := os.MkdirAll(backupDir, 0755); err != nil {
		log.Printf("failed to create backup dir: %v", err)
		return "", err
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	dst := filepath.Join(backupDir, fmt.Sprintf("app_%s.db", timestamp))

	if err := copyFile(src, dst); err != nil {
		log.Printf("backup copy failed: %v", err)
		return "", err
	}

	return dst, nil
}

func copyFile(src, dst string) error {
	s, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	defer s.Close()

	d, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("create dst: %w", err)
	}
	defer d.Close()

	_, err = io.Copy(d, s)
	if err != nil {
		return fmt.Errorf("copy data: %w", err)
	}

	return d.Sync()
}

func PruneBackups(backupDir string, keep int) (int, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		return 0, err
	}

	type fileInfo struct {
		path string
		mod  time.Time
	}

	var files []fileInfo

	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		info, err := e.Info()
		if err != nil {
			continue
		}

		files = append(files, fileInfo{
			path: filepath.Join(backupDir, e.Name()),
			mod:  info.ModTime(),
		})
	}

	// Newest first
	sort.Slice(files, func(i, j int) bool {
		return files[i].mod.After(files[j].mod)
	})

	total := 0

	// Delete everything after the first `keep`
	for i := keep; i < len(files); i++ {
		if err := os.Remove(files[i].path); err != nil {
			log.Printf("failed to remove backup %s: %v", files[i].path, err)
		}
		total++
	}

	return total, nil
}
