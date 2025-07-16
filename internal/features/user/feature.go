package user

// User hold user feature exported interfaces for usage outside of this package.
type User struct {
	Service servicer
	DBStore dbStorer
}
