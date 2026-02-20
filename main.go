package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Config struct {
	RetentionDays       int      `json:"retention_days"`
	BackupRetentionDays int      `json:"backup_retention_days"`
	Parent              string   `json:"parent"`
	Directories         []string `json:"directories"`
	BackupParent        string   `json:"backup_parent"`
	LogFile             string   `json:"log_file"`
}

func main() {
	configFile, err := os.ReadFile("config.json")
	if err != nil {
		fmt.Printf("failed to read config.json: %v\n", err)
		return
	}

	var cfg Config
	if err := json.Unmarshal(configFile, &cfg); err != nil {
		fmt.Printf("failed to parse config json: %v\n", err)
		return
	}

	os.MkdirAll(cfg.BackupParent, 0755)

	f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("failed to open log file: %v\n", err)
		return
	}
	defer f.Close()
	logger := log.New(f, "[cleanup] ", log.LstdFlags)

	archiveThreshold := time.Now().AddDate(0, 0, -cfg.RetentionDays)
	deleteThreshold := time.Now().AddDate(0, 0, -cfg.BackupRetentionDays)
	
	logger.Printf("starting cleanup. archiving logs older than %d days. deleting logs older than %d days.", cfg.RetentionDays, cfg.BackupRetentionDays)

	for _, dir := range cfg.Directories {
		fullPath := filepath.Join(cfg.Parent, dir)
		logger.Printf("scanning directory: %s", fullPath)
		
		var filesToBackup []string

		err = filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				if info.ModTime().Before(deleteThreshold) {
					// file is extremely old, skip backup and just delete it
					logger.Printf("permanently deleting old log: %s", path)
					os.Remove(path)
				} else if info.ModTime().Before(archiveThreshold) {
					// file is between 7 and 14 days old, queue for backup
					filesToBackup = append(filesToBackup, path)
				}
			}
			return nil
		})

		if err != nil {
			logger.Printf("failed to walk %s: %v", fullPath, err)
			continue
		}

		// proceed to zip if there's anything to backup, OR if we need to clean old files out of an existing zip
		zipName := strings.ReplaceAll(dir, "\\", "_") + ".zip"
		targetZip := filepath.Join(cfg.BackupParent, zipName)

		// check if zip exists to see if we need to run the merge/clean process even if no new files are added
		_, zipExists := os.Stat(targetZip)
		
		if len(filesToBackup) > 0 || zipExists == nil {
			err := backupAndRemove(targetZip, filesToBackup, deleteThreshold, logger)
			if err != nil {
				logger.Printf("failed to process backup for %s: %v", dir, err)
			}
		}
	}
	
	logger.Println("cleanup and backup finished")
}

func backupAndRemove(zipPath string, files []string, deleteThreshold time.Time, logger *log.Logger) error {
	tempZipPath := zipPath + ".tmp"
	
	newZipFile, err := os.Create(tempZipPath)
	if err != nil {
		return err
	}
	
	zipWriter := zip.NewWriter(newZipFile)
	filesKeptInZip := 0

	// step 1: copy non-expired files from the old zip
	if _, err := os.Stat(zipPath); err == nil {
		oldZipReader, err := zip.OpenReader(zipPath)
		if err != nil {
			newZipFile.Close()
			return err
		}
		
		for _, file := range oldZipReader.File {
			// check if the file inside the zip is older than 14 days
			if file.Modified.Before(deleteThreshold) {
				logger.Printf("removing expired file from backup zip: %s", file.Name)
				continue 
			}
			
			oldFileReader, err := file.Open()
			if err != nil {
				continue
			}
			
			header := file.FileHeader
			writer, err := zipWriter.CreateHeader(&header)
			if err == nil {
				io.Copy(writer, oldFileReader)
				filesKeptInZip++
			}
			oldFileReader.Close()
		}
		oldZipReader.Close()
	}

	// step 2: add the new log files
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			logger.Printf("failed to read %s for zipping: %v", file, err)
			continue
		}
		
		info, _ := f.Stat()
		header, _ := zip.FileInfoHeader(info)
		header.Name = filepath.Base(file) 
		header.Method = zip.Deflate
		
		writer, err := zipWriter.CreateHeader(header)
		if err == nil {
			io.Copy(writer, f)
			logger.Printf("added to zip: %s", file)
			filesKeptInZip++
		}
		f.Close()
	}

	zipWriter.Close()
	newZipFile.Close()

	// step 3: replace old zip or delete it entirely if it's empty
	if _, err := os.Stat(zipPath); err == nil {
		os.Remove(zipPath)
	}

	if filesKeptInZip > 0 {
		err = os.Rename(tempZipPath, zipPath)
		if err != nil {
			return err
		}
	} else {
		// nothing left in the zip, just remove the temp file
		os.Remove(tempZipPath)
	}

	// step 4: delete the original log files from the server
	for _, file := range files {
		os.Remove(file)
		logger.Printf("deleted original file: %s", file)
	}

	return nil
}