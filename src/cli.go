package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/frontmatter"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
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
		err = os.WriteFile(filepath.Join(dir, p.Slug+".md"), []byte(context), 0644)
		if err != nil {
			fmt.Println(err)
			return
		}
	}
	fmt.Println("backup success")
}

type HugoPost struct {
	Title string    `yaml:"title"`
	Date  time.Time `yaml:"date"`
	Draft bool      `yaml:"draft"`
}

func migrateHugo(path string) {
	db, err := sql.Open("sqlite", "./avalyn.db")
	if err != nil {
		fmt.Println("Error opening database:", err)
		return
	}
	defer db.Close()

	var user string
	err = db.QueryRow("SELECT username FROM users LIMIT 1").Scan(&user)
	if err != nil {
		fmt.Println("No users found in the database. Please register a user first.")
		return
	}

	err = filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(info.Name(), ".md") {
			file, err := os.Open(filePath)
			if err != nil {
				fmt.Printf("Error opening file %s: %v\n", filePath, err)
				return nil // Continue with other files
			}
			defer file.Close()

			var matter HugoPost
			content, err := frontmatter.Parse(file, &matter)
			if err != nil {
				fmt.Printf("Error parsing frontmatter for %s: %v\n", filePath, err)
				return nil // Continue with other files
			}

			slug := slugify(matter.Title)
			status := "post"
			if matter.Draft {
				status = "draft"
			}
			postType := "blog"

			_, err = db.Exec(`INSERT INTO 
                posts(date, author, type, title, slug, content, status, pass)
                VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
				matter.Date.Format("2006-01-02"), user, postType, matter.Title, slug, string(content), status, "")
			if err != nil {
				fmt.Printf("Error inserting post '%s' into database: %v\n", matter.Title, err)
			} else {
				fmt.Printf("Migrated: %s\n", matter.Title)
			}
		}
		return nil
	})

	if err != nil {
		fmt.Println("Error walking through the content directory:", err)
	}
}