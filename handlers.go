package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/mrSun421/birthday-reminder/page"
)

func index(w http.ResponseWriter, r *http.Request) {
	log.Printf("Loading Index Page...\n")
	err = page.Index().Render(r.Context(), w)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func oAuthCallback(w http.ResponseWriter, r *http.Request) {
	log.Printf("Beginning OAuth Callback...\n")

	user, err := gothic.CompleteUserAuth(w, r)
	if err != nil {
		log.Printf("response: %v, error: %v\n", w, err)
		return
	}

	_, err = conn.Exec(context.Background(), "INSERT INTO users (userid,email) VALUES ($1, $2) ON CONFLICT (userid) DO UPDATE SET email=$2 WHERE users.userid=$1", user.UserID, user.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

}

func logout(w http.ResponseWriter, r *http.Request) {
	log.Printf("Starting logout from OAuth...\n")

	err = gothic.Logout(w, r)
	if err != nil {
		log.Printf("error: %v\n", err)
		return
	}
	log.Printf("attempting Redirect to Index page...\n")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func beginOAuth(w http.ResponseWriter, r *http.Request) {
	log.Printf("starting stored login...")
	if gothUser, err := gothic.CompleteUserAuth(w, r); err == nil {
		log.Printf("Local store found")
		session, _ := store.Get(r, "current-session")
		session.Values["user"] = gothUser
		err = sessions.Save(r, w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/userPage", http.StatusSeeOther)

	} else {
		log.Printf("No local store found, staring auth handler\n")
		gothic.BeginAuthHandler(w, r)
	}

}

func userPage(w http.ResponseWriter, r *http.Request) {
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

	rows, _ := conn.Query(context.Background(), "SELECT * FROM birthdays where userid=$1", user.UserID)
	birthdays, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (page.BirthdayItem, error) {
		var userId string
		var firstName string
		var lastName string
		var birthday time.Time
		var id int
		err := row.Scan(&userId, &firstName, &lastName, &birthday, &id)
		return page.BirthdayItem{UserId: userId, FirstName: firstName, LastName: lastName, Birthday: birthday, Id: id}, err
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = page.UserPage(birthdays).Render(r.Context(), w)
	if err != nil {
		log.Printf("%v\n", err)
		w.WriteHeader(http.StatusNotFound)
		return
	}
}

func editBirthday(w http.ResponseWriter, r *http.Request) {
	log.Printf("Begin edit of birthdayItem...\n")
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

	segments := strings.Split(r.URL.Path, "/")
	id, err := strconv.Atoi(segments[len(segments)-1])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("Editing item %v\n", id)
	var userId string
	var firstName string
	var lastName string
	var birthday time.Time

	err = conn.QueryRow(context.Background(), "SELECT userid, personfirstname, personlastname,birthday FROM birthdays WHERE id=$1 AND userid=$2", id, user.UserID).Scan(&userId, &firstName, &lastName, &birthday)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = page.BirthdayForm(page.BirthdayItem{UserId: userId, FirstName: firstName, LastName: lastName, Birthday: birthday, Id: id}).Render(r.Context(), w)
	if err != nil {

		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func requestBirthdayAction(w http.ResponseWriter, r *http.Request) {
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

	segments := strings.Split(r.URL.Path, "/")
	id, err := strconv.Atoi(segments[len(segments)-1])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Printf("birthdayItemId: %v\n", id)
	switch r.Method {
	case http.MethodGet:
		var userId string
		var firstName string
		var lastName string
		var birthday time.Time

		err = conn.QueryRow(context.Background(), "SELECT userid, personfirstname, personlastname,birthday FROM birthdays WHERE id=$1 AND userid=$2", id, user.UserID).Scan(&userId, &firstName, &lastName, &birthday)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = page.BirthdayInfo(page.BirthdayItem{UserId: userId, FirstName: firstName, LastName: lastName, Birthday: birthday, Id: id}).Render(r.Context(), w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	case http.MethodPut:
		err = r.ParseForm()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		firstName := r.FormValue("firstName")
		lastName := r.FormValue("lastName")
		birthday := r.FormValue("birthday")
		commandTag, err := conn.Exec(context.Background(), "UPDATE birthdays SET personfirstname = $1, personlastname = $2, birthday = $3 WHERE id = $4 AND userid = $5", firstName, lastName, birthday, id, user.UserID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if commandTag.RowsAffected() < 1 {
			log.Printf("No Rows Affected")
		}

		parsedBirthday, err := time.Parse("2006-01-02", birthday)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		err = page.BirthdayInfo(page.BirthdayItem{UserId: user.UserID, FirstName: firstName, LastName: lastName, Birthday: parsedBirthday}).Render(r.Context(), w)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	case http.MethodDelete:

	default:
		w.WriteHeader(http.StatusNotFound)
	}

}
