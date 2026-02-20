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
	RetentionDays int      `json:"retention_days"`
	Parent        string   `json:"parent"`
	Directories   []string `json:"directories"`
	BackupParent  string   `json:"backup_parent"`
	LogFile       string   `json:"log_file"`
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

	// ensure backup directory exists
	os.MkdirAll(cfg.BackupParent, 0755)

	f, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("failed to open log file: %v\n", err)
		return
	}
	defer f.Close()
	logger := log.New(f, "[cleanup] ", log.LstdFlags)

	threshold := time.Now().AddDate(0, 0, -cfg.RetentionDays)
	logger.Printf("starting cleanup. merging files older than %d days to backup", cfg.RetentionDays)

	for _, dir := range cfg.Directories {
		fullPath := filepath.Join(cfg.Parent, dir)
		logger.Printf("scanning directory: %s", fullPath)
		
		var filesToBackup []string

		// step 1: collect files
		err = filepath.Walk(fullPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && info.ModTime().Before(threshold) {
				filesToBackup = append(filesToBackup, path)
			}
			return nil
		})

		if err != nil {
			logger.Printf("failed to walk %s: %v", fullPath, err)
			continue
		}

		// step 2: process the collected files
		if len(filesToBackup) > 0 {
			// create a zip name like "prdsanmlypan_log.zip"
			zipName := strings.ReplaceAll(dir, "\\", "_") + ".zip"
			targetZip := filepath.Join(cfg.BackupParent, zipName)

			err := backupAndRemove(targetZip, filesToBackup, logger)
			if err != nil {
				logger.Printf("failed to backup %s: %v", dir, err)
			}
		}
	}
	
	logger.Println("cleanup and backup finished")
}

// backupAndRemove handles merging into an existing zip and deleting the source files
func backupAndRemove(zipPath string, files []string, logger *log.Logger) error {
	tempZipPath := zipPath + ".tmp"
	
	newZipFile, err := os.Create(tempZipPath)
	if err != nil {
		return err
	}
	
	zipWriter := zip.NewWriter(newZipFile)

	// if the zip already exists, copy its old contents to the new temp zip
	if _, err := os.Stat(zipPath); err == nil {
		oldZipReader, err := zip.OpenReader(zipPath)
		if err != nil {
			newZipFile.Close()
			return err
		}
		
		for _, file := range oldZipReader.File {
			oldFileReader, err := file.Open()
			if err != nil {
				continue
			}
			
			header := file.FileHeader
			writer, err := zipWriter.CreateHeader(&header)
			if err == nil {
				io.Copy(writer, oldFileReader)
			}
			oldFileReader.Close()
		}
		oldZipReader.Close()
	}

	// append the new log files
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
		}
		f.Close()
	}

	zipWriter.Close()
	newZipFile.Close()

	// replace old zip with the new merged zip
	if _, err := os.Stat(zipPath); err == nil {
		os.Remove(zipPath)
	}
	err = os.Rename(tempZipPath, zipPath)
	if err != nil {
		return err
	}

	// now that backup is totally secure, delete original files
	for _, file := range files {
		os.Remove(file)
		logger.Printf("deleted original file: %s", file)
	}

	return nil
}