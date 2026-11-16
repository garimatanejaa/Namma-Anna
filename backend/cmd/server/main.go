package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"riderl/internal/api"
	"riderl/web"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := flag.String("addr", ":"+port, "listen address")
	flag.Parse()
	s, err := api.New()
	if err != nil {
		log.Fatal(err)
	}
	srv := &http.Server{Addr: *addr, Handler: s.Handler(web.FS()), ReadHeaderTimeout: 10 * time.Second}
	log.Printf("RideRL Bengaluru: %d wards, listening on %s", len(s.City.Wards), *addr)
	log.Fatal(srv.ListenAndServe())
}
