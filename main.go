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
	w.Header().Set("Content-Type", "application/json")
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
		http.Error(w, `{"error":"failed"}`, 500)
		return
	}

	var pageData map[string]interface{}
	if err := json.Unmarshal(resp.Bytes(), &pageData); err != nil {
		http.Error(w, `{"error":"parse"}`, 500)
		return
	}

	m3uContent := "#EXTM3U\n"
	resultObj, ok := pageData["resultObj"].(map[string]interface{})
	if !ok {
		return
	}

	trays, ok := resultObj["containers"].([]interface{})
	if !ok {
		return
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, trayIf := range trays {
		tray, ok := trayIf.(map[string]interface{})
		if !ok {
			continue
		}
		assets, ok := tray["assets"].(map[string]interface{})
		if !ok {
			continue
		}
		items, ok := assets["containers"].([]interface{})
		if !ok {
			continue
		}

		for _, itemIf := range items {
			item, ok := itemIf.(map[string]interface{})
			if !ok {
				continue
			}
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

			title, _ := metadata["episodeTitle"].(string)
			if title == "" {
				title, _ = metadata["title"].(string)
			}
			
			title = strings.ReplaceAll(title, ",", " -")

			var logo string
			if emf, ok := metadata["emfAttributes"].(map[string]interface{}); ok {
				if l, ok := emf["landscape_thumb"].(string); ok {
					logo = l
				} else if s, ok := emf["square_thumb"].(string); ok {
					logo = s
				}
			}

			wg.Add(1)
			go func(cID, t, l string) {
				defer wg.Done()
				playURL := fmt.Sprintf("https://playcipher.bioscopelive.com/v1/manifest?providers_content_id=%s&provider=sonyliv", cID)
				playResp, err := client.R().SetHeader("User-Agent", "Mozilla/5.0").Get(playURL)

				if err == nil {
					var playData map[string]interface{}
					if err := json.Unmarshal(playResp.Bytes(), &playData); err == nil {
						if rawURL, ok := playData["url"].(string); ok {
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
					}
				}
			}(contentID, title, logo)
		}
	}

	wg.Wait()
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Content-Disposition", `attachment; filename="live.m3u"`)
	w.Write([]byte(m3uContent))
}

func vodM3uHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	client := resty.New()
	pageURL := "https://apiv3.sonyliv.com/AGL/4.8/SR/ENG/MWEB/BD/FL/PAGE/1111_9014359?packageId=liv_premium_GP_1m&kids_safe=false&from=0&to=50"

	resp, err := client.R().SetHeader("User-Agent", "Mozilla/5.0").Get(pageURL)
	if err != nil {
		http.Error(w, `{"error":"failed"}`, 500)
		return
	}

	var pageData map[string]interface{}
	if err := json.Unmarshal(resp.Bytes(), &pageData); err != nil {
		http.Error(w, `{"error":"parse"}`, 500)
		return
	}

	m3uContent := "#EXTM3U\n"
	resultObj, ok := pageData["resultObj"].(map[string]interface{})
	if !ok {
		return
	}

	trays, ok := resultObj["containers"].([]interface{})
	if !ok {
		return
	}

	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, trayIf := range trays {
		tray, ok := trayIf.(map[string]interface{})
		if !ok {
			continue
		}
		assets, ok := tray["assets"].(map[string]interface{})
		if !ok {
			continue
		}
		items, ok := assets["containers"].([]interface{})
		if !ok {
			continue
		}

		for _, itemIf := range items {
			item, ok := itemIf.(map[string]interface{})
			if !ok {
				continue
			}
			metadata, ok := item["metadata"].(map[string]interface{})
			if !ok {
				continue
			}

			isLive, _ := metadata["isLive"].(bool)
			if isLive {
				continue
			}

			contentIdF, ok := metadata["contentId"].(float64)
			if !ok {
				continue
			}
			contentID := fmt.Sprintf("%.0f", contentIdF)

			title, _ := metadata["episodeTitle"].(string)
			if title == "" {
				title, _ = metadata["title"].(string)
			}
			
			title = strings.ReplaceAll(title, ",", " -")

			var logo string
			if emf, ok := metadata["emfAttributes"].(map[string]interface{}); ok {
				if l, ok := emf["landscape_thumb"].(string); ok {
					logo = l
				} else if s, ok := emf["square_thumb"].(string); ok {
					logo = s
				}
			}

			wg.Add(1)
			go func(cID, t, l string) {
				defer wg.Done()
				playURL := fmt.Sprintf("https://playcipher.bioscopelive.com/v1/manifest?providers_content_id=%s&provider=sonyliv&scheme=widevine", cID)
				playResp, err := client.R().SetHeader("User-Agent", "Mozilla/5.0").Get(playURL)

				if err == nil {
					var playData map[string]interface{}
					if err := json.Unmarshal(playResp.Bytes(), &playData); err == nil {
						rawURL, urlOk := playData["url"].(string)
						licenseURL, licOk := playData["drm_license_url"].(string)
						
						if urlOk && licOk {
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

								entry := fmt.Sprintf("#EXTINF:-1 tvg-logo=\"%s\" group-title=\"VOD Sports\",%s\n", l, t)
								entry += "#KODIPROP:inputstream=inputstream.adaptive\n"
								entry += "#KODIPROP:inputstream.adaptive.manifest_type=mpd\n"
								entry += "#KODIPROP:inputstream.adaptive.license_type=com.widevine.alpha\n"
								entry += fmt.Sprintf("#KODIPROP:inputstream.adaptive.license_key=%s\n", licenseURL)
								entry += fmt.Sprintf("%s\n", cleanedURL)

								mu.Lock()
								m3uContent += entry
								mu.Unlock()
							}
						}
					}
				}
			}(contentID, title, logo)
		}
	}

	wg.Wait()
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Content-Disposition", `attachment; filename="vod.m3u"`)
	w.Write([]byte(m3uContent))
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/api/series", seriesHandler)
	http.HandleFunc("/api/play", playHandler)
	http.HandleFunc("/live.m3u", liveM3uHandler)
	http.HandleFunc("/vod.m3u", vodM3uHandler)

	http.ListenAndServe(":"+port, nil)
}
