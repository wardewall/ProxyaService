package bot

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"ProxyaService/internal/auth"
	"ProxyaService/internal/config"
	"ProxyaService/internal/ratelimit"
	"ProxyaService/internal/storage"

	tele "gopkg.in/telebot.v4"
)

type Service struct {
	log   *slog.Logger
	conf  config.Config
	auth  *auth.Service
	store *storage.Store
	rl    *ratelimit.Limiter
}

func New(log *slog.Logger, conf config.Config, auth *auth.Service) *Service {
	return &Service{log: log, conf: conf, auth: auth}
}

func (s *Service) Start() error {
	_ = godotenv.Load()

	pref := tele.Settings{
		Token:  s.conf.BotToken,
		Poller: &tele.LongPoller{Timeout: 10 * time.Second},
	}

	b, err := tele.NewBot(pref)
	if err != nil {
		return err
	}

	// Init store and limiter if DSN configured
	if s.store == nil && s.conf.PostgresDSN != "" {
		st, err := storage.NewPostgres(context.Background(), s.conf.PostgresDSN)
		if err != nil {
			s.log.Error("postgres connect failed", "error", err)
		} else {
			if err := st.Migrate(context.Background()); err != nil {
				s.log.Error("migrate failed", "error", err)
			}
			s.store = st
			s.auth.AttachStore(st)
			s.rl = ratelimit.New(st, s.conf.RatePerMinFree, s.conf.RatePerMinPremium, s.conf.RatePerMinAdmin, s.conf.ThrottleSeconds)
		}
	}

	b.Handle("/start", func(c tele.Context) error {
		uid := c.Sender().ID
		if !s.auth.AuthorizeUserByID(uid) {
			s.log.Warn("access denied by id", "user", uid)
			// попробуем deep-link токен из payload
			payload := strings.TrimSpace(c.Message().Payload)
			if payload != "" {
				if _, err := s.auth.Authenticate(context.Background(), payload, uid); err == nil {
					s.log.Info("authed via deeplink", "user", uid)
					return c.Send("Аутентификация успешна. Используйте /proxy или меню ниже", s.mainMenu())
				}
			}
			return c.Send("Доступ ограничён. Обратитесь к администратору или используйте /auth <token>.")
		}
		return c.Send("Отправьте /proxy для получения кнопки подключения. Либо выполните /auth <token>.", s.mainMenu())
	})

	b.Handle("/menu", func(c tele.Context) error {
		return c.Send("Главное меню:", s.mainMenu())
	})
	// Admin: /issue_token <role> [ttl]
	b.Handle("/issue_token", func(c tele.Context) error {
		uid := c.Sender().ID
		// Simple admin check: must be in AllowedUserIDs and role admin in DB (if any)
		if !s.auth.AuthorizeUserByID(uid) {
			return c.Send("Нет прав")
		}
		if s.store == nil {
			return c.Send("Хранилище не настроено")
		}
		parts := strings.Fields(c.Message().Payload)
		if len(parts) < 1 {
			return c.Send("Использование: /issue_token <free|premium|admin> [30m|24h|7d]")
		}
		role := storage.Role(parts[0])
		var ttl *time.Duration
		if len(parts) >= 2 {
			if d, err := time.ParseDuration(parts[1]); err == nil {
				ttl = &d
			}
		}
		token := genToken()
		var exp *time.Time
		if ttl != nil {
			t := time.Now().Add(*ttl)
			exp = &t
		}
		if err := s.store.CreateToken(context.Background(), token, role, exp, uid, nil); err != nil {
			s.log.Error("token create failed", "error", err)
			return c.Send("Ошибка создания токена")
		}
		return c.Send("Токен: " + token)
	})

	b.Handle("/auth", func(c tele.Context) error {
		uid := c.Sender().ID
		args := strings.Fields(c.Message().Payload)
		if len(args) < 1 {
			return c.Send("Использование: /auth <token>")
		}
		if _, err := s.auth.Authenticate(context.Background(), args[0], uid); err != nil {
			s.log.Info("auth failed", "user", uid)
			return c.Send("Неверный токен")
		}
		s.log.Info("auth ok", "user", uid)
		return c.Send("Аутентификация успешна. Используйте /proxy")
	})

	b.Handle("/proxy", s.handleProxy)
	b.Handle("/disable", s.handleDisable)

	b.Handle("/status", s.handleStatus)
	b.Handle("/help", s.handleHelp)

	// Обработка нажатий текстовых кнопок меню
	b.Handle(tele.OnText, func(c tele.Context) error {
		txt := strings.TrimSpace(c.Text())
		switch txt {
		case "Подключить прокси":
			return s.handleProxy(c)
		case "Отключить прокси":
			return s.handleDisable(c)
		case "Мой статус":
			return s.handleStatus(c)
		case "Помощь":
			return s.handleHelp(c)
		}
		return nil
	})

	s.log.Info("bot started")
	b.Start()
	return nil
}

func buildTgSocksLink(host, port, user, pass string) string {
	if host == "" || port == "" {
		return ""
	}
	u := url.URL{Scheme: "tg", Host: "socks"}
	q := url.Values{}
	q.Set("server", host)
	q.Set("port", port)
	if user != "" {
		q.Set("user", user)
	}
	if pass != "" {
		q.Set("pass", pass)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func safe(v string) string {
	if strings.TrimSpace(v) == "" {
		return "<не задано>"
	}
	return v
}

// genToken generates a short random token (base62, 24 chars)
func genToken() string {
	const alphabet = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	const n = 24
	b := make([]byte, n)
	// crypto/rand; fallback to time if error (rare)
	if _, err := rand.Read(b); err != nil {
		for i := range b {
			b[i] = alphabet[int(time.Now().UnixNano()+int64(i))%len(alphabet)]
		}
		return string(b)
	}
	for i := range b {
		b[i] = alphabet[int(b[i])%len(alphabet)]
	}
	return string(b)
}

// UI helpers and handlers
func (s *Service) mainMenu() *tele.ReplyMarkup {
	m := &tele.ReplyMarkup{ResizeKeyboard: true}
	btnProxy := m.Text("Подключить прокси")
	btnDisable := m.Text("Отключить прокси")
	btnStatus := m.Text("Мой статус")
	btnHelp := m.Text("Помощь")
	m.Reply(m.Row(btnProxy, btnDisable), m.Row(btnStatus, btnHelp))
	return m
}

func (s *Service) handleProxy(c tele.Context) error {
	uid := c.Sender().ID
	if !s.auth.AuthorizeUserByID(uid) {
		s.log.Warn("access denied by id", "user", uid)
		return c.Send("Доступ ограничён. Обратитесь к администратору.")
	}
	if s.store != nil && s.rl != nil {
		if u, err := s.store.GetUser(context.Background(), uid); err == nil {
			if ok, err := s.rl.Allow(context.Background(), u, "proxy"); err == nil {
				if !ok {
					return c.Send("Слишком часто. Попробуйте позже.")
				}
			} else {
				s.log.Error("rate check error", "error", err)
			}
		}
	}
	if s.store != nil {
		if _, err := s.store.GetUser(context.Background(), uid); err != nil {
			_ = s.store.UpsertUser(context.Background(), storage.User{ID: uid, Role: storage.Role(s.conf.DefaultRole), IsAuthed: true})
		}
	}
	link := buildTgSocksLink(s.conf.ProxyHost, s.conf.ProxyPort, s.conf.ProxyUser, s.conf.ProxyPass)
	markup := &tele.ReplyMarkup{}
	btn := markup.URL("Подключить прокси", link)
	markup.Inline(markup.Row(btn))
	info := fmt.Sprintf("Готово к подключению. Если кнопка не сработает, добавьте прокси вручную:\nHost: %s\nPort: %s", safe(s.conf.ProxyHost), safe(s.conf.ProxyPort))
	if s.conf.ProxyUser != "" {
		info += fmt.Sprintf("\nUser: %s", s.conf.ProxyUser)
	}
	if s.conf.ProxyPass != "" {
		info += "\nPass: <скрыт>"
	}
	s.log.Info("sent proxy data", "user", uid)
	return c.Send(info, markup)
}

func (s *Service) handleStatus(c tele.Context) error {
	uid := c.Sender().ID
	role := "<нет БД>"
	authed := false
	if s.store != nil {
		if u, err := s.store.GetUser(context.Background(), uid); err == nil {
			role = string(u.Role)
			authed = u.IsAuthed
		} else {
			role = s.conf.DefaultRole
		}
	}
	return c.Send(fmt.Sprintf("Ваш статус:\nID: %d\nРоль: %s\nАутентифицирован: %t", uid, role, authed), s.mainMenu())
}

func (s *Service) handleHelp(c tele.Context) error {
	return c.Send("Команды:\n/start — меню\n/proxy — подключение\n/disable — отключить прокси\n/status — мой статус\n/help — помощь\n/auth <token> — аутентификация", s.mainMenu())
}

func (s *Service) handleDisable(c tele.Context) error {
	uid := c.Sender().ID
	if !s.auth.AuthorizeUserByID(uid) {
		s.log.Warn("access denied by id", "user", uid)
		return c.Send("Доступ ограничён. Обратитесь к администратору.")
	}
	// Telegram не имеет прямого API для выключения прокси из бота.
	// Даем пользователю быстрые ссылки и инструкцию.
	markup := &tele.ReplyMarkup{}
	btn := markup.URL("Открыть настройки Telegram", "tg://settings")
	markup.Inline(markup.Row(btn))
	msg := "Чтобы отключить прокси: Откройте Telegram → Настройки → Данные и память → Прокси → Выключить."
	return c.Send(msg, markup)
}
