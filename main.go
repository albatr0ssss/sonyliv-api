package main

import (
	"fmt"
	"net/http"
	"os"

	"resty.dev/v3"
)

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")
}

func seriesHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" { return }

	client := resty.New()
	resp, err := client.R().
		SetHeader("User-Agent", "Mozilla/5.0").
		Get("https://apiv3.sonyliv.com/AGL/2.6/SR/ENG/MWEB/BD/FL/CONTENT/DETAIL/BUNDLE/1500001077")

	if err != nil {
		http.Error(w, `{"error": "Failed"}`, 500)
		return
	}
	// এখানে ফিক্স করা হয়েছে (Body() এর বদলে Bytes())
	w.Write(resp.Bytes())
}

func playHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" { return }

	contentID := r.URL.Query().Get("id")
	targetURL := fmt.Sprintf("https://playcipher.bioscopelive.com/v1/manifest?providers_content_id=%s&provider=sonyliv&scheme=widevine", contentID)

	client := resty.New()
	resp, err := client.R().
		SetHeader("User-Agent", "Mozilla/5.0").
		Get(targetURL)

	if err != nil {
		http.Error(w, `{"error": "Failed"}`, 500)
		return
	}
	w.Write(resp.Bytes())
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" 
	}
	http.HandleFunc("/api/series", seriesHandler)
	http.HandleFunc("/api/play", playHandler)
	http.ListenAndServe(":"+port, nil)
}
