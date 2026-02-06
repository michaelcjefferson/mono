package dbbackup

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"
)

func RunBackup(dbPath string) (string, error) {
	src := dbPath
	backupDir := "./db/backups"

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
