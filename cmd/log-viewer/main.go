package main

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

//go:embed static
var staticFiles embed.FS

type Config struct {
	LogFolder       string `json:"logFolder"`
	RefreshInterval int    `json:"refreshInterval"`
	Theme           string `json:"theme"`
}

type LogFile struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type LogContent struct {
	Content      string `json:"content"`
	LastModified string `json:"lastModified"`
}

var config Config

func loadConfig() error {
	refreshInterval := flag.Int("r", 0, "Refresh interval in seconds")
	logFolder := flag.String("l", "", "Log folder path")
	theme := flag.String("t", "", "Theme (light or dark)")
	flag.Parse()

	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}

	targetFolder := currentDir
	if *logFolder != "" {
		targetFolder = *logFolder
	}

	absTargetFolder, err := filepath.Abs(targetFolder)
	if err != nil {
		return err
	}

	configPath := filepath.Join(absTargetFolder, "log-viewer.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			config = Config{
				LogFolder:       absTargetFolder,
				RefreshInterval: 5,
				Theme:           "light",
			}

			if *refreshInterval > 0 {
				config.RefreshInterval = *refreshInterval
			}
			if *theme != "" {
				config.Theme = *theme
			}

			configData, err := json.MarshalIndent(config, "", "  ")
			if err != nil {
				return err
			}

			if err := os.WriteFile(configPath, configData, 0644); err != nil {
				return err
			}

			fmt.Printf("Created %s with default values\n", configPath)
		} else {
			return err
		}
	} else {
		if err := json.Unmarshal(data, &config); err != nil {
			return err
		}

		needsUpdate := false

		if config.LogFolder != absTargetFolder {
			config.LogFolder = absTargetFolder
			needsUpdate = true
		}

		if *refreshInterval > 0 && config.RefreshInterval != *refreshInterval {
			config.RefreshInterval = *refreshInterval
			needsUpdate = true
		}

		if *theme != "" && config.Theme != *theme {
			config.Theme = *theme
			needsUpdate = true
		}

		if needsUpdate {
			configData, err := json.MarshalIndent(config, "", "  ")
			if err != nil {
				return err
			}

			if err := os.WriteFile(configPath, configData, 0644); err != nil {
				return err
			}

			fmt.Printf("Updated %s\n", configPath)
		}
	}

	return nil
}

func configHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

func logsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	entries, err := os.ReadDir(config.LogFolder)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var files []LogFile
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".log" || ext == ".txt" || ext == "" {
			files = append(files, LogFile{
				Name: name,
				Path: filepath.Join(config.LogFolder, name),
			})
		}
	}

	json.NewEncoder(w).Encode(files)
}

func logFileHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	fileName := r.URL.Query().Get("file")
	if fileName == "" {
		http.Error(w, "Missing file parameter", http.StatusBadRequest)
		return
	}

	filePath := filepath.Join(config.LogFolder, fileName)

	if !strings.HasPrefix(filePath, config.LogFolder) {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := LogContent{
		Content:      string(content),
		LastModified: info.ModTime().Format(time.RFC3339),
	}

	json.NewEncoder(w).Encode(response)
}

func getStaticFS() http.FileSystem {
	if _, err := os.Stat("static"); err == nil {
		return http.Dir("static")
	}

	sub, err := fs.Sub(staticFiles, "static")
	if err != nil {
		log.Fatal(err)
	}
	return http.FS(sub)
}

func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}

func main() {
	if err := loadConfig(); err != nil {
		log.Fatalf("Error loading config: %v", err)
	}

	staticFS := getStaticFS()

	http.HandleFunc("/api/config", configHandler)
	http.HandleFunc("/api/logs", logsHandler)
	http.HandleFunc("/api/log", logFileHandler)

	http.Handle("/", http.FileServer(staticFS))

	url := "http://localhost:3000/"
	fmt.Printf("Log Viewer running at %s\n", url)
	fmt.Printf("Watching folder: %s\n", config.LogFolder)
	fmt.Printf("Refresh interval: %d seconds\n", config.RefreshInterval)

	go func() {
		time.Sleep(500 * time.Millisecond)
		if err := openBrowser(url); err != nil {
			fmt.Printf("Could not open browser automatically: %v\n", err)
			fmt.Printf("Please open %s manually\n", url)
		} else {
			fmt.Printf("Opening browser...\n")
		}
	}()

	log.Fatal(http.ListenAndServe(":3000", nil))
}
