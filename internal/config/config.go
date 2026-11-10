package config

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type Conf struct {
	Env  string // dev / test / prod
	Addr string // HTTP Address, example :8888
}

func Load() (Conf, error) {
	if err := loadDotEnv(".env"); err != nil {
		return Conf{}, err
	}
	c := Conf{
		Env:  getEnv("APP_ENV", "dev"),
		Addr: getEnv("HTTP_ADDR", ":8888"),
	}
	switch c.Env {
	case "dev", "test", "prod":
	default:
		return Conf{}, fmt.Errorf("unknown APP_ENV: %q", c.Env)
	}
	return c, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// loadDotEnv parses KEY=VALUE pairs line by line, skipping empty lines,
// comments starting with #, and variables that are already set.
// A missing file is not an error; other read failures (such as permission
// errors or lines exceeding the Scanner limit) are returned.
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.Trim(strings.TrimSpace(v), `"'`)
		if _, exists := os.LookupEnv(k); !exists {
			os.Setenv(k, v)
		}
	}
	if err := sc.Err(); err != nil {
		return fmt.Errorf("scan %s: %w", path, err)
	}
	return nil
}
