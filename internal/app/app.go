package app

import (
	"flag"
	"log"
	"net/http"
	"os"

	apphttp "pre-ipo-market-platform/internal/http"
	"pre-ipo-market-platform/internal/store"
)

func Run() {
	addr := flag.String("addr", ":8080", "HTTP server address")
	dbPath := flag.String("db", "preipo_demo.db", "SQLite database path")
	flag.Parse()

	db, err := store.Open(*dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		log.Fatalf("migrate database: %v", err)
	}
	if err := db.SeedDemoData(); err != nil {
		log.Fatalf("seed database: %v", err)
	}

	server := apphttp.NewServer(db)
	log.Printf("Pre-IPO MVP running on http://localhost%s", *addr)
	log.Printf("Demo accounts: admin@demo.local / investor@demo.local / seller@demo.local, password: demo123")
	if err := http.ListenAndServe(*addr, server.Routes()); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
