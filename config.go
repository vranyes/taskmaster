package taskmaster

import "os"

type Config struct {
	BaseURL string
	Model   string
	APIKey  string
	Addr    string
}

func LoadConfig() Config {
	addr := os.Getenv("TASKMASTER_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	return Config{
		BaseURL: os.Getenv("TASKMASTER_BASE_URL"),
		Model:   os.Getenv("TASKMASTER_MODEL"),
		APIKey:  os.Getenv("TASKMASTER_API_KEY"),
		Addr:    addr,
	}
}
