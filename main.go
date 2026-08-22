package main

import (
"fmt"
"log"
"net/http"
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
w.WriteHeader(http.StatusOK)
fmt.Fprintln(w, "Hello Kubernetes CI/CD")
}

func main() {
http.HandleFunc("/", helloHandler)

log.Println("server listening on :8080")
log.Fatal(http.ListenAndServe(":8080", nil))
}
