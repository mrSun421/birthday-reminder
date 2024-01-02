package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-co-op/gocron/v2"
	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/google"
)

var conn *pgx.Conn
var err error
var store *sessions.CookieStore

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{Addr: fmt.Sprintf(":%s", port)}
	conn, err = pgx.Connect(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}

	store = sessions.NewCookieStore([]byte(os.Getenv("SESSION_KEY")))
	store.MaxAge(int(time.Hour * 24 * 30))
	store.Options.Path = "/"
	store.Options.HttpOnly = true
	store.Options.Secure = false

	gothic.Store = store
	goth.UseProviders(
		google.New(os.Getenv("GOOGLE_OAUTH_KEY"), os.Getenv("GOOGLE_OAUTH_SECRET"), fmt.Sprintf("%s/auth/callback?provider=google", os.Getenv("CURRENT_URL"))),
	)

	sch, err := gocron.NewScheduler()
	if err != nil {
		log.Printf("%v\n", err)
		return
	}
	job, err := sch.NewJob(
		gocron.DailyJob(1, gocron.NewAtTimes(gocron.NewAtTime(11, 59, 0))),
		gocron.NewTask(
			sendMail,
		),
	)

	_ = job
	if err != nil {
		log.Printf("%v\n", err)
		return
	}
	sch.Start()

	http.HandleFunc("/", index)
	http.HandleFunc("/auth/callback", oAuthCallback)
	http.HandleFunc("/logout", logout)
	http.HandleFunc("/auth", beginOAuth)
	http.HandleFunc("/userPage", userPage)
	http.HandleFunc("/userPage/birthdayItem/edit/", editBirthday)
	http.HandleFunc("/userPage/birthdayItem/", requestBirthdayAction)
	http.HandleFunc("/userPage/newBirthdayItem/form", newBirthdayForm)
	http.HandleFunc("/userPage/newBirthdayItem/attemptAdd", attemptAddNewBirthday)

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
	if err != nil {
		log.Printf("%v\n", err)
	}
	err = sch.Shutdown()
	if err != nil {
		log.Printf("%v\n", err)
	}
	os.Exit(0)

}
