package main

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"

	"resty.dev/v3"
)

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "application/json")
}

func proxyHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	target := r.URL.Query().Get("url")
	if target == "" {
		http.Error(w, `{"error": "Missing url parameter"}`, 400)
		return
	}

	client := resty.New()
	resp, err := client.R().
		SetHeader("User-Agent", "Mozilla/5.0").
		Get(target)

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

	rawJSON := string(resp.Bytes())

	re := regexp.MustCompile(`"url"\s*:\s*"([^"]+)"`)
	matches := re.FindStringSubmatch(rawJSON)

	if len(matches) > 1 {
		originalURL := matches[1]
		
		parsableURL := strings.ReplaceAll(originalURL, `\u0026`, "&")
		
		u, err := url.Parse(parsableURL)
		
		if err == nil {
			hdnea := u.Query().Get("hdnea")
			
			u.RawQuery = ""
			
			if hdnea != "" {
				u.RawQuery = "hdnea=" + hdnea
			}
			
			cleanedURL := u.String()
			cleanedURL = strings.Replace(cleanedURL, "grameenphonedrm", "sonygdrm", -1)

			rawJSON = strings.Replace(rawJSON, `"`+originalURL+`"`, `"`+cleanedURL+`"`, 1)
		}
	}

	w.Write([]byte(rawJSON))
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.HandleFunc("/api/proxy", proxyHandler)
	http.HandleFunc("/api/play", playHandler)
	http.ListenAndServe(":"+port, nil)
}
