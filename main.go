package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

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
	if r.Method == "OPTIONS" {
		return
	}

	client := resty.New()
	resp, err := client.R().
		SetHeader("User-Agent", "Mozilla/5.0").
		Get("https://apiv3.sonyliv.com/AGL/2.6/SR/ENG/MWEB/BD/FL/CONTENT/DETAIL/BUNDLE/1500001077")

	if err != nil {
		http.Error(w, `{"error": "Failed"}`, 500)
		return
	}
	w.Write(resp.Bytes())
}

func playHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	contentID := r.URL.Query().Get("id")
	isLive := r.URL.Query().Get("isLive")

	targetURL := fmt.Sprintf("https://playcipher.bioscopelive.com/v1/manifest?providers_content_id=%s&provider=sonyliv", contentID)

	if isLive != "true" {
		targetURL += "&scheme=widevine"
	}

	client := resty.New()
	resp, err := client.R().
		SetHeader("User-Agent", "Mozilla/5.0").
		Get(targetURL)

	if err != nil {
		http.Error(w, `{"error": "Failed"}`, 500)
		return
	}

	var playData map[string]interface{}
	if err := json.Unmarshal(resp.Bytes(), &playData); err == nil {
		if rawURL, ok := playData["url"].(string); ok {

			u, err := url.Parse(rawURL)
			if err == nil {
				hdnea := u.Query().Get("hdnea")

				u.RawQuery = ""

				if hdnea != "" {
					u.RawQuery = "hdnea=" + hdnea
				}

				cleanedURL := u.String()
				cleanedURL = strings.Replace(cleanedURL, "grameenphonedrm", "sonygdrm", -1)

				playData["url"] = cleanedURL
			}
		}

		modifiedJSON, _ := json.Marshal(playData)
		w.Write(modifiedJSON)
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
