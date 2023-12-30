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
	store.Options.Secure = true

	gothic.Store = store

	goth.UseProviders(
		google.New(os.Getenv("GOOGLE_OAUTH_KEY"), os.Getenv("GOOGLE_OAUTH_SECRET"), "http://localhost:8080/auth/callback?provider=google"),
	)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Loading Index Page...\n")
		err = page.Index().Render(r.Context(), w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Beginning OAuth Callback...\n")

		user, err := gothic.CompleteUserAuth(w, r)
		if err != nil {
			log.Printf("response: %v, error: %v\n", w, err)
			return
		}
		session, _ := store.Get(r, "current-session")
		session.Values["user"] = user
		err = sessions.Save(r, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/userPage", http.StatusSeeOther)

	})

	http.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Starting logout from OAuth...\n")

		err = gothic.Logout(w, r)
		if err != nil {
			log.Printf("error: %v\n", err)
			return
		}
		log.Printf("attempting Redirect to Index page...\n")
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})

	http.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("starting stored login...")
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
	http.HandleFunc("/userPage", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Loading userPage...\n")
		session, err := store.Get(r, "current-session")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		user := session.Values["user"].(goth.User)
		if user.IDToken == "" {
			http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
			return
		}

		err = page.UserInfo(user).Render(r.Context(), w)
		if err != nil {
			log.Printf("%v\n", err)
			w.WriteHeader(http.StatusNotFound)
			return
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
