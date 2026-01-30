package hashgen

import (
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"
)

// Use to generate hashes for passwords that can be stored in eg. a mock database - provide passwords to be hashed as parameter, copy and paste results where needed
func GeneratePasswordHashes(passwords []string) {
	for _, password := range passwords {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), 12)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("hash for %s: %s\n", password, hash)
	}
}
