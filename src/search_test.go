package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"html/template"
)

// Mock renderTemplate for testing searchPosts
func renderTemplate(w http.ResponseWriter, r *http.Request, filename string, data interface{}) {
	// For testing, just print out what would be rendered
	fmt.Fprintf(w, "Template: %s, Data: %+v", filename, data)
}

func TestSearchPosts(t *testing.T) {
	// Initialize a test database
	db, _ = initTestDB()
	defer db.Close()

	// Add some dummy posts to the test database
	insertTestPost(t, "Test Title 1", "This is the content of test post one.", "blog", "post")
	insertTestPost(t, "Another Title", "Content for another test post.", "blog", "post")
	insertTestPost(t, "Third Post", "Just some content here.", "blog", "post")
	insertTestPost(t, "Hidden Draft", "This is a draft and should not appear in search.", "blog", "draft")
	insertTestPost(t, "Secret Page", "This page has secret content.", "misc", "post")

	tests := []struct {
		name         string
		query        string
		expectedCode int
		expectedBody string // A substring to check in the response body
		expectedPosts int
	}{
		{
			name:         "Search for 'test'",
			query:        "test",
			expectedCode: http.StatusOK,
			expectedPosts: 3, // Should match "Test Title 1", "another test post", "secret content"
			expectedBody: "Search Results for 'test'",
		},
		{
			name:         "Search for 'content'",
			query:        "content",
			expectedCode: http.StatusOK,
			expectedPosts: 3, // Should match "content of test post one", "content for another test post", "some content here"
			expectedBody: "Search Results for 'content'",
		},
		{
			name:         "Search for 'nonexistent'",
			query:        "nonexistent",
			expectedCode: http.StatusOK,
			expectedPosts: 0,
			expectedBody: "No results found for 'nonexistent'",
		},
		{
			name:         "Empty query",
			query:        "",
			expectedCode: http.StatusSeeOther, // Redirect to home
			expectedBody: "",
		},
		{
			name:         "Search for 'draft' (should not find hidden draft)",
			query:        "draft",
			expectedCode: http.StatusOK,
			expectedPosts: 0,
			expectedBody: "No results found for 'draft'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest("GET", "/search?q="+url.QueryEscape(tt.query), nil)
			if err != nil {
				t.Fatalf("could not create request: %v", err)
			}
			rr := httptest.NewRecorder()
			searchPosts(rr, req)

			if rr.Code != tt.expectedCode {
				t.Errorf("expected status %d; got %d", tt.expectedCode, rr.Code)
			}

			if tt.expectedBody != "" && !strings.Contains(rr.Body.String(), tt.expectedBody) {
				t.Errorf("expected body to contain %q; got %q", tt.expectedBody, rr.Body.String())
			}

			// For simplicity, we'll parse the rendered template output to count posts
			// In a real scenario, you might want to mock renderTemplate to return a specific struct
			// and assert on that struct's content.
			if tt.expectedCode == http.StatusOK {
				var data struct {
					Posts []Post
					Query string
					Title template.HTML
				}
				// This is a very crude way to get the data, a proper mock for renderTemplate would be better
				// For now, we'll just check if the number of "<li>" tags (representing posts) matches
				postCount := strings.Count(rr.Body.String(), "<li>")
				if postCount != tt.expectedPosts {
					t.Errorf("expected %d posts; got %d posts in rendered template body: %s", tt.expectedPosts, postCount, rr.Body.String())
				}
			}
		})
	}
}

// Helper to initialize a temporary in-memory database for testing
func initTestDB() (*sql.DB, error) {
	testDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to open test database: %w", err)
	}

	_, err = testDB.Exec(`
		CREATE TABLE IF NOT EXISTS posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			date TEXT,
			author TEXT,
			type TEXT,
			title TEXT,
			slug TEXT UNIQUE,
			content TEXT,
			status TEXT,
			pass TEXT
		);
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to create posts table: %w", err)
	}
	return testDB, nil
}

// Helper to insert a test post
func insertTestPost(t *testing.T, title, content, postType, status string) {
	slug := slugify(title)
	_, err := db.Exec(`INSERT INTO posts (date, author, type, title, slug, content, status, pass) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		"2023-01-01", "testuser", postType, title, slug, content, status, "")
	if err != nil {
		t.Fatalf("failed to insert test post: %v", err)
	}
}

// Mock slugify function for testing (should ideally be in a shared utils file)
func slugify(title string) string {
	s := strings.ToLower(title)
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, s)
	return s
}

// Mock of checkSession for testing. Always return valid.
func checkSession(w http.ResponseWriter, r *http.Request) (int, bool) {
	return 1, true
}
