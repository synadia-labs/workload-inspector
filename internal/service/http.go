package service

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/nats-io/nkeys"
	"github.com/synadia-labs/workloads-demo/internal/config"
)

type Middleware func(http.Handler) http.Handler

type HTTPServer interface {
	Start() error
}

type httpServer struct {
	port   string
	server *http.ServeMux
}

func (s *httpServer) Start() error {
	addr := fmt.Sprintf(":%s", s.port)
	log.Printf("http server started on %s", addr)
	return http.ListenAndServe(addr, s.server)
}

func NewHTTPServer(cfg *config.HttpConfig, insp Inspector) HTTPServer {
	port := cfg.Port
	if port == "" {
		port = "8080"
	}

	middlewares := []Middleware{}
	if cfg.UseAuth {
		token, err := createToken()
		if err != nil {
			log.Fatalf("error creating token: %s", err)
		}
		log.Println("--------------------------------")
		log.Printf("http server api token: %s", token)
		log.Printf("to use the token include, 'Authorization: Bearer %s' in the request header", token)
		log.Println("--------------------------------")
		middlewares = append(middlewares, authMiddleware(token))
	}

	mux := http.NewServeMux()

	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			for _, middleware := range middlewares {
				next = middleware(next)
			}
			next.ServeHTTP(w, r)
		})
	}

	// ping
	var ping http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(insp.Ping()))
	})
	mux.Handle("GET /ping", middleware(ping))

	// env
	var env http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		environ := insp.GetEnvironment()
		json.NewEncoder(w).Encode(environ)
	})
	mux.Handle("GET /env", middleware(env))

	// run
	var run http.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req RunCommandRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, fmt.Errorf(`expected request format is {"command": "string"}`).Error(), http.StatusBadRequest)
			return
		}
		response, err := insp.RunCommand(req.Command)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})
	mux.Handle("POST /run", middleware(run))

	return &httpServer{server: mux, port: port}
}

func authMiddleware(token string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			bearer := r.Header.Get("Authorization")
			if bearer != "Bearer "+token {
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Generate a random API token.
func createToken() (string, error) {
	nkey, err := nkeys.CreatePair(nkeys.PrefixByteUser)
	if err != nil {
		return "", fmt.Errorf("error creating nkey pair: %s", err)
	}

	token, err := nkey.PublicKey()
	if err != nil {
		return "", fmt.Errorf("error getting public key: %s", err)
	}

	return string(token), nil
}
