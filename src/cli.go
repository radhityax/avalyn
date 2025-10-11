package main

import (
	"fmt"
	"bufio"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
	"os"
)

/* register account */
func registerAccount() {
	fmt.Println("username:")
	getUser := bufio.NewScanner(os.Stdin)
	getUser.Scan()
	username := getUser.Text()

	fmt.Println("password:")
	getPass := bufio.NewScanner(os.Stdin)
	getPass.Scan()
	pass := getPass.Text()

	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		fmt.Println("hash error")
		return
	}

	_, err = db.Exec(`INSERT INTO users(username, password_hash) VALUES(?,?)`,
	username, hash)
}

func backup() {
}
