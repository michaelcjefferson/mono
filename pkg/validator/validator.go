package validator

import (
	"regexp"
	"slices"
	"strconv"
)

var (
	// This regex allows email addresses with no dot in the DNS, eg. user@gmail, because there are some valid DNS's without a dot so it is technically a valid email address.
	EmailRX = regexp.MustCompile("^[a-zA-Z0-9.!#$%&'*+\\/=?^_`{|}~-]+@[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(?:\\.[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$")
	// ASCII letters + numbers, start with a letter, single separators (_ or .)
	// UsernameSafeRX = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9]*(?:[._][A-Za-z0-9]+)*$`)
	// Unicode allowed, singular separators (_ . -), start and end with character
	UsernameRX = regexp.MustCompile(`^[\p{L}\p{N}]+(?:[._-][\p{L}\p{N}]+)*$`)
	// Unicode, spaces, emojis, prevents leading/trailing junk
	// UsernameFlexRX = regexp.MustCompile(`^[\p{L}\p{N}](?:[\p{L}\p{N} ._-]*[\p{L}\p{N}])?$`)
)

type Validator struct {
	Errors map[string]any
}

// Function to create new Validator
func New() *Validator {
	return &Validator{Errors: make(map[string]any)}
}

// If the Validator conatins no errors, the value is valid
func (v *Validator) Valid() bool {
	return len(v.Errors) == 0
}

// If an error doesn't already exist at the provided key, create one with the provided message
func (v *Validator) AddError(key, message string) {
	if _, exists := v.Errors[key]; !exists {
		v.Errors[key] = message
	}
}

func (v *Validator) Check(ok bool, key, message string) {
	if !ok {
		v.AddError(key, message)
	}
}

// Same as Python "if _ in []:"
func In(value string, list ...string) bool {
	// for i := range list {
	// 	if value == list[i] {
	// 		return true
	// 	}
	// }
	if slices.Contains(list, value) {
		return true
	}

	return false
}

// Returns true if value matches regex pattern
func Matches(value string, rx *regexp.Regexp) bool {
	return rx.MatchString(value)
}

// If there are duplicate values in the provided slice of strings, the duplicate value(s) will just be set twice in uniqueValues rather than creating a new key-value pair, so the length of uniqueValues will be shorter than the length of values
func Unique(values []string) bool {
	uniqueValues := make(map[string]bool)

	for _, value := range values {
		uniqueValues[value] = true
	}

	return len(values) == len(uniqueValues)
}

// Instead of checking whether string is empty, it is likely that later we will implement the In function to test provided genres against a slice of valid genres. For now, this is a lovely little proof of concept
func NoEmptyString(values []string) bool {
	// for _, value := range values {
	// 	if value == "" {
	// 		return false
	// 	}
	// }
	if slices.Contains(values, "") {
		return false
	}

	return true
}

// The following functions check whether a provided string can be converted to the datatype named in the function signature
func StringIsInt(value string) bool {
	if _, err := strconv.Atoi(value); err != nil {
		return false
	}
	return true
}

func StringIsBool(value string) bool {
	if _, err := strconv.ParseBool(value); err != nil {
		return false
	}
	return true
}
