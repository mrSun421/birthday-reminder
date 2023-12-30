package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/google"
	"github.com/mrSun421/birthday-reminder/page"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{Addr: fmt.Sprintf(":%s", port)}
	conn, err := pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	_ = conn
	if err != nil {
		log.Fatal(err)
	}

	store := sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))
	store.MaxAge(int(time.Hour * 24 * 30))
	store.Options.Path = "/"
	store.Options.HttpOnly = true
	store.Options.Secure = false

	gothic.Store = store

	goth.UseProviders(
		google.New(os.Getenv("GOOGLE_OAUTH_KEY"), os.Getenv("GOOGLE_OAUTH_SECRET"), "http://localhost:8080/auth/callback?provider=google"),
	)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Loading Index Page...\n")
		err = page.Index().Render(r.Context(), w)
		if err != nil {
			log.Printf("%v\n", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}
	})

	http.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Beginning OAuth Callback...\n")
		provider := r.URL.Query().Get("provider")
		r = r.WithContext(context.WithValue(context.Background(), "provider", provider))

		user, err := gothic.CompleteUserAuth(w, r)
		if err != nil {
			log.Printf("response: %v, error: %v\n", w, err)
			return
		}
		err = page.UserInfo(user).Render(r.Context(), w)
		if err != nil {
			log.Printf("%v\n", err)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		log.Printf("Sucessfully rendered new page")

	})

	http.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Starting logout from OAuth...")
		provider := r.URL.Query().Get("provider")
		r = r.WithContext(context.WithValue(context.Background(), "provider", provider))

		err = gothic.Logout(w, r)
		if err != nil {
			log.Printf("error: %v\n", err)
			return
		}
		w.Header().Set("Location", "/")
		w.WriteHeader(http.StatusTemporaryRedirect)
		log.Printf("Sucessfully redirected to Index page")
	})

	http.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("starting stored login...")
		provider := r.URL.Query().Get("provider")
		r = r.WithContext(context.WithValue(context.Background(), "provider", provider))
		if gothUser, err := gothic.CompleteUserAuth(w, r); err == nil {
			log.Printf("Local store found")
			err = page.UserInfo(gothUser).Render(r.Context(), w)
			if err != nil {
				log.Printf("%v\n", err)
				w.WriteHeader(http.StatusNotFound)
				return
			}
		} else {
			log.Printf("No local store found, staring auth handler\n")
			gothic.BeginAuthHandler(w, r)
		}

	})

	go func() {
		log.Printf("Starting server at port %s...\n", port)
		err := server.ListenAndServe()
		if err != nil {
			log.Fatal(err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	log.Printf("Stopping Server...\n")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err = server.Shutdown(ctx)
	log.Printf("%v\n", err)
	os.Exit(0)

}
