package main

import (
	"fmt"
	"net/http"
	"strings"
)

func searchPosts(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	var posts []Post
	searchQuery := "%" + strings.ToLower(query) + "%"

		rows, err := db.Query(`SELECT id, date, author, type, title, slug, content, status 
			FROM posts 
			WHERE (LOWER(title) LIKE ? OR LOWER(content) LIKE ?) 
			AND status = 'post' 
			ORDER BY date DESC, id DESC`, 
			searchQuery, searchQuery)
	if err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var p Post
		err := rows.Scan(&p.ID, &p.Date, &p.Author, &p.Type, &p.Title, &p.Slug, &p.Content, &p.Status)
		if err != nil {
			fmt.Printf("Error scanning post: %v\n", err)
			continue
		}
		posts = append(posts, p)
	}

	data := struct {
		Posts []Post
		Query string
		Title string
	}{
		Posts: posts,
		Query: query,
		Title: "Search Results for '" + query + "'",
	}

	renderTemplate(w, r, "search.html", data)
}
