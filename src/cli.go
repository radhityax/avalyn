package main

import (
	"fmt"
	"bufio"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
	"os"
	"database/sql"
	"path/filepath"
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
	var dir string = "backup"
	err := os.MkdirAll(dir, 0755)
	if err != nil {
		return
	}

	var p Post
	db, err := sql.Open("sqlite", "./avalyn.db")
	if err != nil {
		return
	}
	defer db.Close()

	rows, err := db.Query(`SELECT type,title,content,slug FROM posts`)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		err := rows.Scan(&p.Type, &p.Title, &p.Content, &p.Slug)
		if err != nil {
			return
		}
		context := fmt.Sprintf("---\ntitle: %s\n---\n%s\n", p.Title, p.Content)
		err = os.WriteFile(filepath.Join(dir,p.Slug+".md"),[]byte(context), 0644)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	fmt.Println("backup success")
}
