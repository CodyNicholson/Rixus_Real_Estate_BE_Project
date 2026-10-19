package config

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
)

type Config struct {
	Port         string
	DatabaseURL  string
	JWTSecret    string
	CORSOrigin   string
	CFTeamDomain string
	CFPolicyAUD  string
	CFCertsURL   string
	DevCFEmail   string
	AllowDevAuth bool
}

func Load() (Config, error) {
	loadDotEnv(".env", "../.env")

	c := Config{
		Port:         getenv("PORT", "8080"),
		DatabaseURL:  os.Getenv("DATABASE_URL"),
		JWTSecret:    os.Getenv("JWT_SECRET"),
		CORSOrigin:   strings.TrimSpace(os.Getenv("CORS_ORIGIN")),
		CFTeamDomain: os.Getenv("CF_TEAM_DOMAIN"),
		CFPolicyAUD:  os.Getenv("CF_POLICY_AUD"),
		CFCertsURL:   os.Getenv("CF_CERTS_URL"),
		DevCFEmail:   os.Getenv("DEV_CF_EMAIL"),
		AllowDevAuth: os.Getenv("ALLOW_DEV_AUTH") == "true" || os.Getenv("ALLOW_DEV_AUTH") == "1",
	}

	if c.DatabaseURL == "" {
		dbHost := os.Getenv("DB_HOST")
		if dbHost != "" {
			dbPort := getenv("DB_PORT", "5432")
			dbUser := getenv("DB_USER", "postgres")
			dbPass := os.Getenv("DB_PASSWORD")
			dbName := getenv("DB_NAME", "rixus")
			dbSSL := getenv("DB_SSLMODE", "disable")

			var userInfo *url.Userinfo
			if dbPass != "" {
				userInfo = url.UserPassword(dbUser, dbPass)
			} else if dbUser != "" {
				userInfo = url.User(dbUser)
			}

			u := url.URL{
				Scheme:   "postgres",
				User:     userInfo,
				Host:     net.JoinHostPort(dbHost, dbPort),
				Path:     dbName,
				RawQuery: fmt.Sprintf("sslmode=%s", dbSSL),
			}
			c.DatabaseURL = u.String()
		}
	}

	if c.DatabaseURL == "" {
		return c, errors.New("DATABASE_URL or DB_HOST is required")
	}
	if len(c.JWTSecret) < 32 {
		return c, errors.New("JWT_SECRET is required and must be at least 32 characters")
	}
	if err := validateCORSOrigins(c.CORSOrigin); err != nil {
		return c, err
	}
	return c, nil
}

// validateCORSOrigins requires CORS_ORIGIN to be a comma-separated list of
// explicit origins (scheme://host[:port]). Wildcards are rejected because the
// server sends Access-Control-Allow-Credentials: true.
func validateCORSOrigins(v string) error {
	if v == "" {
		return errors.New("CORS_ORIGIN is required (e.g. https://app.example.com)")
	}
	for _, o := range strings.Split(v, ",") {
		o = strings.TrimSpace(o)
		if o == "" || o == "*" || strings.Contains(o, "*") {
			return fmt.Errorf("CORS_ORIGIN contains invalid origin %q: wildcards and empty entries are not allowed", o)
		}
		u, err := url.Parse(o)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
			(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			return fmt.Errorf("CORS_ORIGIN contains invalid origin %q: must be scheme://host[:port]", o)
		}
	}
	return nil
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func loadDotEnv(filenames ...string) {
	for _, filename := range filenames {
		f, err := os.Open(filename)
		if err != nil {
			continue
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, val, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			val = strings.TrimSpace(val)
			if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
				val = val[1 : len(val)-1]
			}
			if os.Getenv(key) == "" {
				_ = os.Setenv(key, val)
			}
		}
		break
	}
}
