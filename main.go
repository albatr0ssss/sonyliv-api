package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"resty.dev/v3"
)

const (
	TOKEN  = "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOiJodHRwczovL3RvZmZlZWxpdmUuY29tIiwiY291bnRyeSI6IkJEIiwiZF9pZCI6ImRkMzIzYWJkLWQwMjUtNDk0MS1iMDBiLWYyZTEyZTk2N2M1MSIsImV4cCI6MTc5NzIyNjYzMywiaWF0IjoxNzk0NTk2ODMzLCJpc3MiOiJ0b2ZmZWVsaXZlLmNvbSIsImp0aSI6IjgyZWZiNzQ4LTllNmYtNDc4NS05NGM1LWQ4MzczMTE1NWRiM18xNzk0NTk2ODMzIiwicHJvdmlkZXIiOiJ0b2ZmZWUiLCJyX2lkIjoiZGQzMjNhYmQtZDAyNS00OTQxLWIwMGItZjJlMTJlOTY3YzUxIiwic19pZCI6ImRkMzIzYWJkLWQwMjUtNDk0MS1iMDBiLWYyZTEyZTk2N2M1MSIsInRva2VuIjoiYWNjZXNzIiwidHlwZSI6ImRldmljZSJ9.SRNBuOF56fCGI1DAk-H1P2vetDatWUoenY9fJXbbuiQQ_RIwvuaRaN53DgU_1OcZ6CLncYJncWg2O_hYyyzjTA"
	RAIL   = "032cc9194378b850b2fec39c6386fd1f"
	UA     = "Mozilla/5.0 (Linux; Android 10; K)"
	TK_SEC = "HadyAkamaiCloneKey26"
)

// In-Memory Cache
type CacheItem struct {
	Data      interface{}
	ExpiresAt int64
}

var kvStore sync.Map

func getCache(key string) (interface{}, bool) {
	if item, found := kvStore.Load(key); found {
		cItem := item.(CacheItem)
		if time.Now().UnixMilli() < cItem.ExpiresAt {
			return cItem.Data, true
		}
		kvStore.Delete(key)
	}
	return nil, false
}

func setCache(key string, data interface{}, ttlMillis int64) {
	kvStore.Store(key, CacheItem{
		Data:      data,
		ExpiresAt: time.Now().UnixMilli() + ttlMillis,
	})
}

// Token Generation & Validation (For Protecting Your Own URLs)
func genTk() string {
	exp := time.Now().Unix() + 3600
	msg := fmt.Sprintf("%d", exp)
	h := hmac.New(sha256.New, []byte(TK_SEC))
	h.Write([]byte(msg))
	signature := h.Sum(nil)
	hash := base64.RawURLEncoding.EncodeToString(signature)
	return fmt.Sprintf("exp=%d~hmac=%s", exp, hash)
}

func valTk(t string) bool {
	if t == "" {
		return false
	}
	re := regexp.MustCompile(`exp=(\d+)~hmac=(.+)`)
	m := re.FindStringSubmatch(t)
	if len(m) < 3 {
		return false
	}
	expStr, providedHash := m[1], m[2]
	
	var exp int64
	fmt.Sscanf(expStr, "%d", &exp)
	if time.Now().Unix() > exp {
		return false
	}

	h := hmac.New(sha256.New, []byte(TK_SEC))
	h.Write([]byte(expStr))
	signature := h.Sum(nil)
	expectedHash := base64.RawURLEncoding.EncodeToString(signature)

	return providedHash == expectedHash
}

// Catalog Fetcher
type Channel struct {
	Name string
	Logo string
	Cat  string
}

func getCatalogCached() map[string]Channel {
	if cached, ok := getCache("LIVE_CATALOG"); ok {
		return cached.(map[string]Channel)
	}

	ch := make(map[string]Channel)
	client := resty.New()
	p := 1
	blockedKeywords := []string{"somoy", "zee", "sony", "ntv", "channel i", "toffee tv", "toffee movies", "saudi", "news asia", "recap", "trt", "dw", "russia today", "jamuna", "france 24", "cgtn", "ekattor", "cnn", "islamic", "&tv", "b4u", "hum", "rajdhani", "& pictures", "discovery", "zing", "bangla tv", "ekhon", "pogo", "tlc", "cartoon", "animal planet", "channel s", "independent", "global", "investigation", "ananda", "bijoy", "asian", "nexus", "desh", "movie bangla", "mohona"}

	for {
		url := fmt.Sprintf("https://content-prod.services.toffeelive.com/toffee/BD/DK/web/rail/generic/editorial-dynamic/%s?page=%d", RAIL, p)
		resp, err := client.R().SetHeader("User-Agent", UA).Get(url)
		if err != nil || resp.StatusCode() != 200 {
			break
		}

		var d map[string]interface{}
		if err := json.Unmarshal(resp.Bytes(), &d); err != nil {
			break
		}

		list, ok := d["list"].([]interface{})
		if !ok || len(list) == 0 {
			break
		}

		for _, itemIf := range list {
			item := itemIf.(map[string]interface{})
			cid, _ := item["id"].(string)
			title, _ := item["title"].(string)
			t := strings.ToLower(title)

			isBlocked := false
			for _, kw := range blockedKeywords {
				if strings.Contains(t, kw) {
					isBlocked = true
					break
				}
			}

			if !isBlocked && cid != "" {
				lg := ""
				if images, ok := item["images"].([]interface{}); ok && len(images) > 0 {
					if img, ok := images[0].(map[string]interface{}); ok {
						if path, ok := img["path"].(string); ok {
							lg = "https://assets-prod.services.toffeelive.com/" + path
						}
					}
				}
				cat := "Live Sports"
				if catList, ok := item["catogory"].([]interface{}); ok && len(catList) > 0 {
					cat, _ = catList[0].(string)
				}
				ch[cid] = Channel{Name: title, Logo: lg, Cat: cat}
			}
		}
		p++
	}

	setCache("LIVE_CATALOG", ch, 180000) // 3 mins cache
	return ch
}

// Entitlement Fetcher (Uses Your Bearer Token Safely in Backend)
func getFreshStream(cid string) string {
	cacheKey := "STREAM_" + cid
	if cached, ok := getCache(cacheKey); ok {
		return cached.(string)
	}

	client := resty.New()
	url := fmt.Sprintf("https://entitlement-prod.services.toffeelive.com/toffee/BD/DK/web/playback/%s", cid)
	resp, err := client.R().
		SetHeader("Authorization", "Bearer "+TOKEN).
		SetHeader("Content-Type", "application/json").
		SetHeader("User-Agent", UA).
		SetBody("{}").
		Post(url)

	if err == nil {
		var pb map[string]interface{}
		if err := json.Unmarshal(resp.Bytes(), &pb); err == nil {
			if details, ok := pb["playbackDetails"].(map[string]interface{}); ok {
				if data, ok := details["data"].([]interface{}); ok && len(data) > 0 {
					if first, ok := data[0].(map[string]interface{}); ok {
						if st, ok := first["url"].(string); ok && st != "" {
							setCache(cacheKey, st, 180000) // 3 mins cache
							return st
						}
					}
				}
			}
		}
	}
	return ""
}

// Direct M3U8 Rewriter (Points directly to Toffee CDN)
func rewriteM3u8Direct(body, target string) string {
	tUrl, _ := url.Parse(target)
	tDir := target[:strings.LastIndex(target, "/")+1]
	lines := strings.Split(body, "\n")
	var out []string

	reURI := regexp.MustCompile(`URI=["']([^"']+)["']`)

	for _, l := range lines {
		t := strings.TrimSpace(l)
		if t != "" && !strings.HasPrefix(t, "#") {
			var abs string
			if strings.HasPrefix(t, "http") {
				abs = t
			} else if strings.HasPrefix(t, "/") {
				abs = tUrl.Scheme + "://" + tUrl.Host + t
			} else {
				abs = tDir + t
			}
			out = append(out, abs)
			
		} else if strings.Contains(t, "URI=") {
			r := reURI.ReplaceAllStringFunc(t, func(m string) string {
				p1 := reURI.FindStringSubmatch(m)[1]
				var abs string
				if strings.HasPrefix(p1, "http") {
					abs = p1
				} else if strings.HasPrefix(p1, "/") {
					abs = tUrl.Scheme + "://" + tUrl.Host + p1
				} else {
					abs = tDir + p1
				}
				return fmt.Sprintf(`URI="%s"`, abs)
			})
			out = append(out, r)
		} else {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
}

func mainHandler(w http.ResponseWriter, r *http.Request) {
	enableCORS(w)
	if r.Method == "OPTIONS" {
		return
	}

	p := r.URL.Path
	origin := "https://" + r.Host

	// 1. Playlist Endpoint
	if p == "/toffee-epl.m3u" {
		reqAccept := r.Header.Get("Accept")
		secFetchDest := r.Header.Get("Sec-Fetch-Dest")
		if secFetchDest == "document" || strings.Contains(reqAccept, "text/html") {
			http.Redirect(w, r, "https://t.me/livesportsplay", http.StatusFound)
			return
		}

		data := getCatalogCached()
		tk := genTk()

		m3u := "#EXTM3U\n#\"name\": \"Toffee EPL & BFL - Owner: Md Sohanur Rahman Hady (HADY)\"\n\n"
		for cid, c := range data {
			m3u += fmt.Sprintf(`#EXTINF:-1 tvg-logo="%s" group-title="%s", %s`+"\n", c.Logo, c.Cat, c.Name)
			m3u += fmt.Sprintf("%s/play/%s.m3u8?token=%s\n\n", origin, cid, tk)
		}

		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Write([]byte(m3u))
		return
	}

	urlTk := r.URL.Query().Get("token")

	// 2. Play Endpoint (Main M3U8 Proxy)
	if strings.HasPrefix(p, "/play/") {
		if !valTk(urlTk) {
			http.Error(w, "Token Expired or Invalid", http.StatusForbidden)
			return
		}

		cid := strings.Replace(p, "/play/", "", 1)
		cid = strings.Replace(cid, ".m3u8", "", 1)

		streamUrl := getFreshStream(cid)
		if streamUrl == "" {
			http.Error(w, "Stream Not Found or Locked", http.StatusNotFound)
			return
		}

		client := resty.New()
		resp, err := client.R().SetHeader("User-Agent", UA).Get(streamUrl)
		if err != nil {
			http.Error(w, "Error fetching stream", http.StatusInternalServerError)
			return
		}

		// Returns original Toffee CDN links directly to the player
		rx := rewriteM3u8Direct(string(resp.Bytes()), streamUrl)
		
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
		w.Write([]byte(rx))
		return
	}

	w.Write([]byte("Toffee Live Proxy Active - Built with Go & Resty"))
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	http.HandleFunc("/", mainHandler)
	http.ListenAndServe(":"+port, nil)
}
