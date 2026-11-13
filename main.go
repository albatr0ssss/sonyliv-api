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
		http.Error(w, "Failed to fetch page", 500)
		return
	}

	var pageData map[string]interface{}
	if err := json.Unmarshal(resp.Bytes(), &pageData); err != nil {
		http.Error(w, "Failed to parse JSON", 500)
		return
	}

	m3uContent := "#EXTM3U\n"
	resultObj, ok := pageData["resultObj"].(map[string]interface{})
	if !ok {
		http.Error(w, "Invalid resultObj", 500)
		return
	}

	trays, ok := resultObj["containers"].([]interface{})
	if !ok {
		return
	}

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

			var logo string
			if emf, ok := metadata["emfAttributes"].(map[string]interface{}); ok {
				if l, ok := emf["landscape_thumb"].(string); ok {
					logo = l
				} else if s, ok := emf["square_thumb"].(string); ok {
					logo = s
				}
			}

			playURL := fmt.Sprintf("https://playcipher.bioscopelive.com/v1/manifest?providers_content_id=%s&provider=sonyliv", contentID)
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
							m3uContent += fmt.Sprintf("#EXTINF:-1 tvg-logo=\"%s\" group-title=\"Live Sports\",%s\n%s\n", logo, title, cleanedURL)
						}
					}
				}
			}
		}
	}

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
	pageURL := "https://apiv3.sonyliv.com/AGL/4.9/A/ENG/MWEB/BD/FL/PAGE/17378"

	resp, err := client.R().SetHeader("User-Agent", "Mozilla/5.0").Get(pageURL)
	if err != nil {
		http.Error(w, "Failed to fetch page", 500)
		return
	}

	var pageData map[string]interface{}
	if err := json.Unmarshal(resp.Bytes(), &pageData); err != nil {
		http.Error(w, "Failed to parse JSON", 500)
		return
	}

	m3uContent := "#EXTM3U\n"
	resultObj, ok := pageData["resultObj"].(map[string]interface{})
	if !ok {
		http.Error(w, "Invalid resultObj", 500)
		return
	}

	trays, ok := resultObj["containers"].([]interface{})
	if !ok {
		return
	}

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

			var logo string
			if emf, ok := metadata["emfAttributes"].(map[string]interface{}); ok {
				if l, ok := emf["landscape_thumb"].(string); ok {
					logo = l
				} else if s, ok := emf["square_thumb"].(string); ok {
					logo = s
				}
			}

			playURL := fmt.Sprintf("https://playcipher.bioscopelive.com/v1/manifest?providers_content_id=%s&provider=sonyliv&scheme=widevine", contentID)
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

							m3uContent += fmt.Sprintf("#EXTINF:-1 tvg-logo=\"%s\" group-title=\"VOD Sports\",%s\n", logo, title)
							m3uContent += "#KODIPROP:inputstream=inputstream.adaptive\n"
							m3uContent += "#KODIPROP:inputstream.adaptive.manifest_type=mpd\n"
							m3uContent += "#KODIPROP:inputstream.adaptive.license_type=com.widevine.alpha\n"
							m3uContent += fmt.Sprintf("#KODIPROP:inputstream.adaptive.license_key=%s\n", licenseURL)
							m3uContent += fmt.Sprintf("%s\n", cleanedURL)
						}
					}
				}
			}
		}
	}

	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Content-Disposition", `attachment; filename="vod.m3u"`)
	w.Write([]byte(m3uContent))
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/live.m3u", liveM3uHandler)
	http.HandleFunc("/vod.m3u", vodM3uHandler)

	http.ListenAndServe(":"+port, nil)
}
