package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	_ "github.com/go-sql-driver/mysql" // MariaDB/MySQL driver
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// --- GLOBAL VARIABLES ---
var db *sql.DB
var googleOauthConfig *oauth2.Config

// --- INIT & SETUP ---
func init() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found (Deploying environment variables will be used)")
	}

	googleOauthConfig = &oauth2.Config{
		RedirectURL:  "http://localhost:4000/auth/callback",
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email", "https://www.googleapis.com/auth/userinfo.profile"},
		Endpoint:     google.Endpoint,
	}
}

func initDB() {
	dsn := "codm_web:pass123@tcp(127.0.0.1:3306)/codm_hub?parseTime=true"
	var err error
	db, err = sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Database open karne mein error: ", err)
	}

	if err = db.Ping(); err != nil {
		log.Fatal("Database Ping Error: ", err)
	}

	log.Println("[DB] MariaDB se connection ekdum SUCCESSFUL hai! 🎉")
}

// --- MAIN FUNCTION ---
func main() {
	initDB()
	defer db.Close()

	mux := http.NewServeMux()

	// Static files (CSS, Images, JS, aur Uploaded files)
	fileServer := http.FileServer(http.Dir("./ui/static/"))
	mux.Handle("/static/", http.StripPrefix("/static", fileServer))

	// Page Routes
	mux.HandleFunc("/", home)
	mux.HandleFunc("/events", events)
	mux.HandleFunc("/team-up", teamUp)
	mux.HandleFunc("/report-bug", reportBug)
	mux.HandleFunc("/report-player", reportPlayer)
	mux.HandleFunc("/suggestions", suggestions)
	mux.HandleFunc("/profile", profile)

	// --- GUNSMITH ROUTES (6 Separate Pages) ---
	mux.HandleFunc("/gunsmith", gunsmith) // Main category menu

	// 🔥 Naye direct routes
	mux.HandleFunc("/gunsmith/assault", gunsmithAssault)
	mux.HandleFunc("/gunsmith/smg", gunsmithSMG)
	mux.HandleFunc("/gunsmith/sniper", gunsmithSniper)
	mux.HandleFunc("/gunsmith/lmg", gunsmithLMG)
	mux.HandleFunc("/gunsmith/shotgun", gunsmithShotgun)
	mux.HandleFunc("/gunsmith/marksman", gunsmithMarksman)

	mux.HandleFunc("/gunsmith/upload", gunsmithUpload) // Image upload handler

	// Action Routes
	mux.HandleFunc("/chat/send", chatSend)
	mux.HandleFunc("/add-event", handleAddEvent)

	// OAuth Routes
	mux.HandleFunc("/auth/google", googleLogin)
	mux.HandleFunc("/auth/callback", googleCallback)

	log.Println("Starting CODM server on http://localhost:4000")
	err := http.ListenAndServe(":4000", mux)
	log.Fatal(err)
}
