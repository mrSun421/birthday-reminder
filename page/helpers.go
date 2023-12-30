package page

import "time"

type BirthdayItem struct {
	UserId    string
	FirstName string
	LastName  string
	Birthday  time.Time
	Id        int
}
