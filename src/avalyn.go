package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"log"
	_ "modernc.org/sqlite"
	"net/http"
	"os"
	"strings"
	"time"
)

var db *sql.DB

type Post struct {
	ID      int
	Date    string
	Title   string
	Type    string
	Author  string
	Content string
	Slug    string
	Status  string
	HTML    string
	Pass    string
}

func main() {
	var err error
	db, err = sql.Open("sqlite", "./avalyn.db")
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE,
		password_hash TEXT
	)`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		id TEXT PRIMARY KEY,
		user_id INTEGER,
		expiry DATETIME
	)`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		date TEXT,
		author TEXT,
		type TEXT,
		title TEXT,
		slug TEXT UNIQUE,
		content TEXT,
		status TEXT,
		pass TEXT
	)`)
	if err != nil {
		log.Fatal(err)
	}

	if err != nil {
		log.Fatal(err)
	}

	if len(os.Args) == 1 {

		return
	} else if os.Args[1] == "-s" {
		http.HandleFunc("/", indexPage)

		http.HandleFunc("/login",
			func(w http.ResponseWriter, r *http.Request) {
				if _, err := r.Cookie("session"); err == nil {
					http.Redirect(w, r, "/dashboard", 302)
				} else {
					loginPage(w, r)
				}
			})

		http.HandleFunc("/register",
			func(w http.ResponseWriter, r *http.Request) {
				if _, err := r.Cookie("session"); err == nil {
					http.Redirect(w, r, "/dashboard", 302)
				} else {
					if register_browser_mode > 0 {
						signupPage(w, r)
					} else {
						http.Redirect(w, r, "/", 302)
					}
				}
			})

		http.HandleFunc("/logout", logoutHandler)
		http.HandleFunc("/dashboard", authMiddleware(dashboardPage))

		http.HandleFunc("/search", searchPosts)
		http.HandleFunc("/blog/", pageRouter(1))
		http.HandleFunc("/new", authMiddleware(newPage))
		http.HandleFunc("/edit/", authMiddleware(editPage))
		http.HandleFunc("/delete/", authMiddleware(deletePage))

		http.HandleFunc("/misc/", pageRouter(2))

		staticDir := fmt.Sprintf("themes/%s/static", theme)
		http.Handle("/static/", http.StripPrefix("/static/",
			http.FileServer(http.Dir(staticDir))))

		fmt.Println("avalyn started at http://localhost:1112")
		err := http.ListenAndServe(":1112", nil)
		if err != nil {
			log.Fatal(err)
			return
		}
		return
	} else if os.Args[1] == "-b" {
		backup()
		return
	} else if os.Args[1] == "-v" {
		fmt.Printf("avalyn - %s\n", version)
		fmt.Println("github.com/radhityax/avalyn")
		return
	} else if os.Args[1] == "-r" {
		registerAccount()
		return
	} else if os.Args[1] == "-m" {
		if len(os.Args) < 3 {
			fmt.Println("not enough")
			return
		}
		migrateHugo(os.Args[2])
		return
	} else {
		printHelp()
		return
	}
}

func createSession(w http.ResponseWriter, userID int) {
	sessionID := generateSessionID()
	expiry := time.Now().Add(1 * time.Hour)
	_, err := db.Exec(`INSERT INTO sessions(id, user_id, expiry) VALUES (?, ?, ?)`,
		sessionID, userID, expiry)

	if err != nil {
		log.Println("create session error:", err)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		Expires:  expiry,
	})
}

func checkSession(w http.ResponseWriter, r *http.Request) (int, bool) {
	cookie, err := r.Cookie("session")
	if err != nil || cookie.Value == "" {
		return 0, false
	}

	var userID int
	var expiry time.Time

	err = db.QueryRow(`SELECT user_id, expiry FROM sessions WHERE id = ?`,
		cookie.Value).Scan(&userID, &expiry)

	if err != nil || time.Now().After(expiry) {
		http.SetCookie(w, &http.Cookie{
			Name:   "session",
			Value:  "",
			Path:   "/",
			MaxAge: -1,
		})
		return 0, false
	}
	return userID, true
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, valid := checkSession(w, r)
		if !valid {
			http.Redirect(w, r, "/login", 303)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func generateSessionID() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		log.Fatal(err)
	}
	return hex.EncodeToString(b)
}

func getUsername(w http.ResponseWriter, r *http.Request) string {
	if xxx, err := r.Cookie("session"); err == nil {
		var id int
		var user string
		err := db.QueryRow("SELECT user_id FROM sessions WHERE id=?", xxx.Value).Scan(&id)
		if err != nil {
			return ""
		}

		err = db.QueryRow("SELECT username FROM users WHERE id=?",
			id).Scan(&user)
		return user
	}
	return ""
}

func hashPassword(s string) (string, error) {
	if s == "" {
		return "", nil
	}
	b, err := bcrypt.GenerateFromPassword([]byte(s), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func checkPasswordHash(hash, pass string) bool {
	if hash == "" {
		return true
	}
	if pass == "" {
		return false
	}
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(pass))
	return err == nil
}

func isUnlocked(r *http.Request, slug string) bool {
	c, err := r.Cookie("unlocked_" + slug)
	if err != nil {
		return false
	}
	return c.Value == "1"
}

func setUnlockedCookie(w http.ResponseWriter, slug string) {
	cookie := &http.Cookie{
		Name:   "unlocked_" + slug,
		Value:  "1",
		Path:   "/",
		MaxAge: 3600 * 1,
	}
	http.SetCookie(w, cookie)
}

func pageRouter(option int) http.HandlerFunc {
	var path string
	return func(w http.ResponseWriter, r *http.Request) {
		if option == 1 {
			path = strings.TrimPrefix(r.URL.Path, "/blog/")
		} else if option == 2 {
			path = strings.TrimPrefix(r.URL.Path, "/misc/")
		}
		if strings.HasSuffix(path, "/unlock") {
			if option == 1 {
				pageUnlock(1)(w, r)
			} else if option == 2 {
				pageUnlock(2)(w, r)
			}
			return
		}
		if option == 1 {
			Page(1)(w, r)
		} else if option == 2 {
			Page(2)(w, r)
		}
	}
}
