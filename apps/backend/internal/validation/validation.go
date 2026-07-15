package validation

import "regexp"

var emailPattern = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)
var usernamePattern = regexp.MustCompile(`^[a-z0-9._]{3,30}$`)
var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

func ValidEmail(email string) bool {
	return emailPattern.MatchString(email)
}

func ValidUsername(username string) bool {
	return usernamePattern.MatchString(username)
}

func ValidUUID(value string) bool {
	return uuidPattern.MatchString(value)
}

func ValidPassword(p string) bool {
	return len(p) >= 8 && len(p) <= 128
}
