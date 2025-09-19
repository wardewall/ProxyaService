package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	BotToken          string
	ProxyHost         string
	ProxyPort         string
	ProxyUser         string
	ProxyPass         string
	AllowedUserIDs    []int64
	AuthTokens        []string
	LogLevel          string
	PostgresDSN       string
	DefaultRole       string
	RatePerMinFree    int
	RatePerMinPremium int
	RatePerMinAdmin   int
	ThrottleSeconds   int
}

func Load() Config {
	return Config{
		BotToken:          firstNonEmpty(os.Getenv("TOKEN"), os.Getenv("BOT_TOKEN")),
		ProxyHost:         firstNonEmpty(os.Getenv("PROXY_HOST"), os.Getenv("PROXY_SERVER")),
		ProxyPort:         os.Getenv("PROXY_PORT"),
		ProxyUser:         os.Getenv("PROXY_USER"),
		ProxyPass:         os.Getenv("PROXY_PASS"),
		AllowedUserIDs:    parseInt64List(os.Getenv("ALLOWED_USER_IDS")),
		AuthTokens:        parseStringList(os.Getenv("AUTH_TOKENS"), os.Getenv("AUTH_TOKEN")),
		LogLevel:          firstNonEmpty(os.Getenv("LOG_LEVEL"), "info"),
		PostgresDSN:       firstNonEmpty(os.Getenv("PG_DSN"), buildDSN()),
		DefaultRole:       firstNonEmpty(os.Getenv("DEFAULT_ROLE"), "free"),
		RatePerMinFree:    parseIntDefault(os.Getenv("RATE_LIMIT_FREE_PER_MIN"), 10),
		RatePerMinPremium: parseIntDefault(os.Getenv("RATE_LIMIT_PREMIUM_PER_MIN"), 60),
		RatePerMinAdmin:   parseIntDefault(os.Getenv("RATE_LIMIT_ADMIN_PER_MIN"), 500),
		ThrottleSeconds:   parseIntDefault(os.Getenv("THROTTLE_SECONDS"), 2),
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func parseInt64List(s string) []int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var res []int64
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if v, err := strconv.ParseInt(p, 10, 64); err == nil {
			res = append(res, v)
		}
	}
	return res
}

func parseStringList(values ...string) []string {
	var in []string
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			in = append(in, v)
		}
	}
	if len(in) == 0 {
		return nil
	}
	var res []string
	for _, v := range in {
		for _, p := range strings.Split(v, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				res = append(res, p)
			}
		}
	}
	return res
}

func parseIntDefault(s string, def int) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return def
	}
	if v, err := strconv.Atoi(s); err == nil {
		return v
	}
	return def
}

func buildDSN() string {
	host := firstNonEmpty(os.Getenv("PG_HOST"), os.Getenv("POSTGRES_HOST"))
	port := firstNonEmpty(os.Getenv("PG_PORT"), os.Getenv("POSTGRES_PORT"))
	user := firstNonEmpty(os.Getenv("PG_USER"), os.Getenv("POSTGRES_USER"))
	pass := firstNonEmpty(os.Getenv("PG_PASSWORD"), os.Getenv("POSTGRES_PASSWORD"))
	db := firstNonEmpty(os.Getenv("PG_DB"), os.Getenv("POSTGRES_DB"))
	ssl := firstNonEmpty(os.Getenv("PG_SSLMODE"), os.Getenv("POSTGRES_SSLMODE"))
	if host == "" || user == "" || db == "" {
		return ""
	}
	if port == "" {
		port = "5432"
	}
	if ssl == "" {
		ssl = "disable"
	}
	if pass != "" {
		return "postgres://" + urlEscape(user) + ":" + urlEscape(pass) + "@" + host + ":" + port + "/" + db + "?sslmode=" + ssl
	}
	return "postgres://" + urlEscape(user) + "@" + host + ":" + port + "/" + db + "?sslmode=" + ssl
}

func urlEscape(s string) string { return strings.ReplaceAll(s, "@", "%40") }
