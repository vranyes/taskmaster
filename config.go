package taskmaster

import "os"

type Config struct {
	BaseURL      string
	Model        string
	APIKey       string
	Addr         string
	EdgeSecret   string
	DirectoryURL string
}

func LoadConfig() Config {
	addr := os.Getenv("TASKMASTER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	dir := os.Getenv("TASKMASTER_DIRECTORY_URL")
	if dir == "" {
		dir = "http://voice-enroll-svc.voice-enroll.svc:8082"
	}
	return Config{
		BaseURL:      os.Getenv("TASKMASTER_BASE_URL"),
		Model:        os.Getenv("TASKMASTER_MODEL"),
		APIKey:       os.Getenv("TASKMASTER_API_KEY"),
		Addr:         addr,
		EdgeSecret:   os.Getenv("TASKMASTER_EDGE_SECRET"),
		DirectoryURL: dir,
	}
}
