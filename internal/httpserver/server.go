package httpserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/mrgold/internal/telegram"
	"github.com/rs/zerolog/log"
)

type Server struct {
	httpServer *http.Server
}

func NewServer(port string, opts ...func(*Server)) *Server {
	s := &Server{
		httpServer: &http.Server{
			Addr:    ":" + port,
			Handler: &Handler{},
		},
	}

	for _, opt := range opts {
		opt(s)
	}

	return s
}

func WithTelegramClient(telegramClient *telegram.Client) func(*Server) {
	return func(s *Server) {
		s.httpServer.Handler.(*Handler).telegramClient = telegramClient
	}
}

func (s *Server) Start() bool {
	log.Info().Msg("server is listening on port " + s.httpServer.Addr)

	if s.httpServer.Handler.(*Handler).telegramClient == nil {
		return false
	}

	if err := s.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Error().Err(err).Send()
		return false
	}

	return true
}

func (s *Server) Stop() bool {
	if s.httpServer == nil {
		return true
	}

	c, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(c); err != nil {
		log.Error().Err(err).Send()
		return false
	}

	return true
}

type Handler struct {
	telegramClient *telegram.Client
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	endpoint := r.URL.Path
	if endpoint != "/webhook" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	method := r.Method
	if method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var err error

	var body []byte
	body, err = io.ReadAll(r.Body)
	if err != nil {
		log.Error().Err(err).Send()

		w.WriteHeader(http.StatusBadRequest)
		return
	}

	log.Info().
		Str("ip", getClientIP(r)).
		Str("method", method).
		Str("endpoint", endpoint).
		Str("request", string(body)).
		Send()

	var update *telegram.Update
	if err = json.Unmarshal(body, &update); err != nil {
		log.Error().Err(err).Send()

		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if update.CallbackQuery != nil {
		h.telegramClient.HandleCallbackQuery(update.CallbackQuery)
		return
	}

	h.telegramClient.HandleMessage(update.Message)
}

func getClientIP(r *http.Request) string {
	if xForwardedFor := r.Header.Get("X-Forwarded-For"); xForwardedFor != "" {
		return strings.Split(xForwardedFor, ",")[0]
	}

	if xRealIP := r.Header.Get("X-Real-IP"); xRealIP != "" {
		return xRealIP
	}

	ip, _, _ := net.SplitHostPort(r.RemoteAddr)

	return ip
}
