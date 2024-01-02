package main

import (
	"bytes"
	"context"
	"log"
	"os"

	"github.com/Shopify/gomail"
	"github.com/jackc/pgx/v5"
)

var fromEmail string = "birthdayremindersun1@gmail.com"
var fromEmailPassword string = os.Getenv("BIRTHDAYEMAIL_PASSWORD")

func sendMail() {

	sendOneWeekMail()
	sendDayBeforeMail()
	sendOnDayMail()
}

func sendOneWeekMail() {
	log.Printf("attempting to send email for one week before...\n")
	emailToNames, err := getNamesAndEmail(7)
	if err != nil {
		log.Printf("%v\n", err)
		return
	}
	for email, names := range emailToNames {
		msg := gomail.NewMessage()
		msg.SetHeader("From", fromEmail)
		msg.SetHeader("To", email)
		msg.SetHeader("Subject", "You have birthdays in a week!")

		var msgBody bytes.Buffer
		err := oneWeek(names).Render(context.Background(), &msgBody)
		msg.SetBody("text/html", msgBody.String())
		if err != nil {
			log.Printf("%v\n", err)
			return
		}
		dialer := gomail.NewDialer("smtp.gmail.com", 587, fromEmail, fromEmailPassword)
		if err := dialer.DialAndSend(msg); err != nil {
			log.Fatal(err)
		}
	}

}

func sendDayBeforeMail() {
	log.Printf("attempting to send email for the day before...\n")
	emailToNames, err := getNamesAndEmail(1)
	if err != nil {
		log.Printf("%v\n", err)
		return
	}
	for email, names := range emailToNames {
		msg := gomail.NewMessage()
		msg.SetHeader("From", fromEmail)
		msg.SetHeader("To", email)
		msg.SetHeader("Subject", "You have birthdays tomorrow!")

		var msgBody bytes.Buffer
		err := oneDay(names).Render(context.Background(), &msgBody)
		msg.SetBody("text/html", msgBody.String())
		if err != nil {
			log.Printf("%v\n", err)
			return
		}
		dialer := gomail.NewDialer("smtp.gmail.com", 587, fromEmail, fromEmailPassword)
		if err := dialer.DialAndSend(msg); err != nil {
			log.Fatal(err)
		}
	}
}

func sendOnDayMail() {
	log.Printf("attempting to send email on day of...\n")
	emailToNames, err := getNamesAndEmail(0)
	if err != nil {
		log.Printf("%v\n", err)
		return
	}
	for email, names := range emailToNames {
		msg := gomail.NewMessage()
		msg.SetHeader("From", fromEmail)
		msg.SetHeader("To", email)
		msg.SetHeader("Subject", "You have birthdays today!")

		var msgBody bytes.Buffer
		err := dayOf(names).Render(context.Background(), &msgBody)
		msg.SetBody("text/html", msgBody.String())
		if err != nil {
			log.Printf("%v\n", err)
			return
		}
		dialer := gomail.NewDialer("smtp.gmail.com", 587, fromEmail, fromEmailPassword)
		if err := dialer.DialAndSend(msg); err != nil {
			log.Fatal(err)
		}
	}
}

func getNamesAndEmail(daystil int) (map[string][]string, error) {
	var email string
	var name string
	emailToName := make(map[string][]string)

	rows, _ := conn.Query(context.Background(), "WITH birthdaycalc AS (SELECT email, personfirstname, personlastname, birthday, (( BIRTHDAY + MAKE_INTERVAL(YEARS => ((EXTRACT(YEAR FROM now()) - EXTRACT(YEAR FROM BIRTHDAY))::INTEGER)))::DATE - NOW()::DATE) AS DAYSTIL FROM users JOIN birthdays ON birthdays.userid = users.userid) SELECT email, personfirstname|| ' ' ||personlastname AS fullname FROM birthdaycalc WHERE daystil = $1", daystil)
	_, err := pgx.ForEachRow(rows, []any{&email, &name}, func() error {
		emailToName[email] = append(emailToName[email], name)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return emailToName, nil

}
