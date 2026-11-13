package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"

	"resty.dev/v3"
)

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func extractItems(data interface{}) []map[string]interface{} {
	var items []map[string]interface{}

	switch v := data.(type) {
	case []interface{}:
		for _, item := range v {
			items = append(items, extractItems(item)...)
		}
	case map[string]interface{}:
		if _, hasMeta := v["metadata"]; hasMeta {
			if _, hasId := v["id"]; hasId {
				items = append(items, v)
			}
		}
		for key, val := range v {
			if key == "containers" || key == "assets" || key == "episodes" || key == "items" {
				items = append(items, extractItems(val)...)
			}
		}
	}
	return items
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
			if isLive == "true" {
				cleanedURL = strings.Replace(cleanedURL, "std_mdh-800300010", "master", -1)
			}

			rawJSON = strings.Replace(rawJSON, `"`+originalURL+`"`, `"`+cleanedURL+`"`, 1)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(rawJSON))
}

func liveM3uHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	client := resty.New()
	pageURL := "https://apiv3.sonyliv.com/AGL/4.9/A/ENG/MWEB/BD/FL/PAGE/17378"

	resp, err := client.R().SetHeader("User-Agent", "Mozilla/5.0").Get(pageURL)
	if err != nil {
		http.Error(w, "Failed to fetch page API", 500)
		return
	}

	var pageData map[string]interface{}
	if err := json.Unmarshal(resp.Bytes(), &pageData); err != nil {
		http.Error(w, "Failed to parse API JSON", 500)
		return
	}

	resultObj, ok := pageData["resultObj"].(map[string]interface{})
	if !ok {
		http.Error(w, "Invalid API structure", 500)
		return
	}

	items := extractItems(resultObj)
	m3uContent := "#EXTM3U\n"
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, item := range items {
		metadata, ok := item["metadata"].(map[string]interface{})
		if !ok {
			continue
		}

		isLive, _ := metadata["isLive"].(bool)
		if !isLive {
			continue
		}

		contentIdF, ok := metadata["contentId"].(float64)
		if !ok {
			continue
		}
		contentID := fmt.Sprintf("%.0f", contentIdF)

		mainTitle, _ := metadata["title"].(string)
		epTitle, _ := metadata["episodeTitle"].(string)

		reDate := regexp.MustCompile(`(?i)\s*-\s*\d{1,2}\s+[a-z]{3}\s+\d{4}\s*$`)
		epTitle = reDate.ReplaceAllString(epTitle, "")

		var finalTitle string
		if epTitle != "" && mainTitle != "" && epTitle != mainTitle {
			finalTitle = epTitle + " - " + mainTitle
		} else if epTitle != "" {
			finalTitle = epTitle
		} else {
			finalTitle = mainTitle
		}

		finalTitle = strings.ReplaceAll(finalTitle, ",", " -")

		var logo string
		autoPlayID := ""

		if emf, ok := metadata["emfAttributes"].(map[string]interface{}); ok {
			if l, ok := emf["landscape_thumb"].(string); ok {
				logo = l
			} else if s, ok := emf["square_thumb"].(string); ok {
				logo = s
			}
			if apID, ok := emf["auto_play_id"].(float64); ok {
				autoPlayID = fmt.Sprintf("%.0f", apID)
			}
		}

		wg.Add(1)
		go func(cID, apID, t, l string) {
			defer wg.Done()

			fetchCipher := func(id string) string {
				if id == "" {
					return ""
				}
				playURL := fmt.Sprintf("https://playcipher.bioscopelive.com/v1/manifest?providers_content_id=%s&provider=sonyliv", id)
				playResp, err := client.R().SetHeader("User-Agent", "Mozilla/5.0").Get(playURL)
				if err == nil {
					var playData map[string]interface{}
					if err := json.Unmarshal(playResp.Bytes(), &playData); err == nil {
						if rawURL, ok := playData["url"].(string); ok {
							return rawURL
						}
					}
				}
				return ""
			}

			rawURL := fetchCipher(cID)
			if rawURL == "" {
				rawURL = fetchCipher(apID)
			}

			if rawURL != "" {
				parsableURL := strings.ReplaceAll(rawURL, `\u0026`, "&")
				u, err := url.Parse(parsableURL)
				if err == nil {
					hdnea := u.Query().Get("hdnea")
					u.RawQuery = ""
					if hdnea != "" {
						u.RawQuery = "hdnea=" + hdnea
					}
					cleanedURL := u.String()
					cleanedURL = strings.Replace(cleanedURL, "grameenphonedrm", "sonygdrm", -1)
					cleanedURL = strings.Replace(cleanedURL, "std_mdh-800300010", "master", -1)

					entry := fmt.Sprintf("#EXTINF:-1 tvg-logo=\"%s\" group-title=\"Live Sports\",%s\n%s\n", l, t, cleanedURL)

					mu.Lock()
					m3uContent += entry
					mu.Unlock()
				}
			}
		}(contentID, autoPlayID, finalTitle, logo)
	}

	wg.Wait()
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Content-Disposition", `attachment; filename="live.m3u"`)
	w.Write([]byte(m3uContent))
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/api/play", playHandler)
	http.HandleFunc("/live.m3u", liveM3uHandler)

	http.ListenAndServe(":"+port, nil)
}
