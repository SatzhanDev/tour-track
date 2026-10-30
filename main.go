package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Tour-track is running!")
	})
	log.Println("Server started on port 8084")
	err := http.ListenAndServe(":8084", nil)
	if err != nil {
		log.Fatal(err)
	}
}
