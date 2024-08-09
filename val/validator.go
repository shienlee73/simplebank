package val

import (
	"fmt"
	"net/mail"
	"regexp"
)

var (
	isValidUsername = regexp.MustCompile(`^[a-z0-9_]+$`).MatchString
	isValidFullName = regexp.MustCompile(`^[a-zA-Z\s]+$`).MatchString
)

func ValidateString(value string, minLength int, maxLength int) error {
	n := len(value)
	if n < minLength || n > maxLength {
		return fmt.Errorf("must contain from %d-%d characters", minLength, maxLength)
	}
	return nil
}

func ValidateUsername(username string) error {
	if err := ValidateString(username, 3, 100); err != nil {
		return err
	}
	if !isValidUsername(username) {
		return fmt.Errorf("must contain only lowercase letters, digits, or underscore")
	}
	return nil
}

func ValidateFullName(fullname string) error {
	if err := ValidateString(fullname, 3, 100); err != nil {
		return err
	}
	if !isValidFullName(fullname) {
		return fmt.Errorf("must contain only letters or spaces")
	}
	return nil
}

func ValidatePassword(password string) error {
	return ValidateString(password, 6, 100)
}

func ValidateEmail(email string) error {
	if err := ValidateString(email, 3, 200); err != nil {
		return err
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("is not a valid email address")
	}
	return nil
}

func ValidateEmailId(emailId int64) error {
	if emailId < 0 {
		return fmt.Errorf("must be a positive integer")
	}
	return nil
}

func ValidateSecretCode(secretCode string) error {
	return ValidateString(secretCode, 32, 128)
}
