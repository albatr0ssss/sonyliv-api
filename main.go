package main
import ("encoding/json";"fmt";"net/http";"os";"regexp";"strings";"sync";"time";"resty.dev/v3")

const TOKEN, RAIL, UA = "EyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOiJodHRwczovL3RvZmZlZWxpdmUuY29tIiwiY291bnRyeSI6IkJEIiwiZF9pZCI6IjEwZmFiZDJlLWNmZjktNGViZi1iZDU5LTdkOWMyY2E1MzZlNiIsImV4cCI6MTc5NTI2MjcxNiwiaWF0IjoxNzc5NzEwNzE2LCJpc3MiOiJ0b2ZmZWVsaXZlLmNvbSIsImp0aSI6IjZhZTJjNDc2LTNjOTctNDcwYS1hMmZlLTU0OGFkMmQ3YjZlZl8xNzc5NzEwNzE2IiwicHJvdmlkZXIiOiJ0b2ZmZWUiLCJyX2lkIjoiOTBhNTg1MzUtNjIzYy00YzgwLTg4MDYtZjY1YjY5NGU2YjcyIiwic19pZCI6ImY4NjhiZjNkLWNiZjUtNDE5MS1hZWZiLWI3MTRjNGEwZTM1ZCIsInRva2VuIjoicmVmcmVzaCIsInR5cGUiOiJzdWJzY3JpYmVyIn0.xtWFphKUAgBYh8UfXLsMkJTxkN3lVWQLyQayFI2Fhm2qcGoi138SNAEUnJ-_luWVsTm4_Crq9qP94HOblsPNsg", "032cc9194378b850b2fec39c6386fd1f", "Mozilla/5.0 (Linux; Android 10; K)"
var kv sync.Map

func getC(k string) (interface{}, bool) { if i, ok := kv.Load(k); ok { if time.Now().Unix() < i.([]interface{})[1].(int64) { return i.([]interface{})[0], true }; kv.Delete(k) }; return nil, false }
func setC(k string, v interface{}, ttl int64) { kv.Store(k, []interface{}{v, time.Now().Unix() + ttl}) }

func getCat() map[string][]string {
	if c, ok := getC("CAT"); ok { return c.(map[string][]string) }
	ch, p, c := make(map[string][]string), 1, resty.New()
	for {
		r, e := c.R().SetHeader("User-Agent", UA).Get(fmt.Sprintf("https://content-prod.services.toffeelive.com/toffee/BD/DK/web/rail/generic/editorial-dynamic/%s?page=%d", RAIL, p))
		if e != nil || r.StatusCode() != 200 { break }
		var d map[string]interface{}; json.Unmarshal(r.Bytes(), &d); l, ok := d["list"].([]interface{}); if !ok || len(l) == 0 { break }
		for _, i := range l {
			im := i.(map[string]interface{}); id, t := im["id"].(string), im["title"].(string)
			if id != "" && !regexp.MustCompile(`(?i)somoy|zee|sony|ntv|channel i|toffee|saudi|news|trt|dw|russia|jamuna|france|cgtn|ekattor|cnn|islamic|&tv|b4u|hum|rajdhani|discovery|zing|bangla|ekhon|pogo|tlc|cartoon|animal|channel s|independent|global|investigation|ananda|bijoy|asian|nexus|desh|mohona`).MatchString(t) {
				lg, cat := "", "Live Sports"
				if imgs, ok := im["images"].([]interface{}); ok && len(imgs) > 0 { lg = "https://assets-prod.services.toffeelive.com/" + imgs[0].(map[string]interface{})["path"].(string) }
				if cats, ok := im["catogory"].([]interface{}); ok && len(cats) > 0 { cat = cats[0].(string) }
				ch[id] = []string{t, lg, cat}
			}
		}
		p++
	}
	setC("CAT", ch, 180); return ch
}

func getSt(id string) string {
	if s, ok := getC("ST_" + id); ok { return s.(string) }
	r, e := resty.New().R().SetHeader("Authorization", "Bearer "+TOKEN).SetHeader("Content-Type", "application/json").SetHeader("User-Agent", UA).SetBody("{}").Post("https://entitlement-prod.services.toffeelive.com/toffee/BD/DK/web/playback/" + id)
	if e != nil { fmt.Println("[ERROR] API Request Failed for ID:", id, "| Error:", e); return "" }
	var pb map[string]interface{}; json.Unmarshal(r.Bytes(), &pb)
	if d, ok := pb["playbackDetails"].(map[string]interface{}); ok {
		if dt, ok := d["data"].([]interface{}); ok && len(dt) > 0 {
			if u, ok := dt[0].(map[string]interface{})["url"].(string); ok && u != "" { setC("ST_"+id, u, 180); return u }
		}
	}
	fmt.Println("[WARNING] No URL in JSON for ID:", id, "| Raw Response:", string(r.Bytes()))
	return ""
}

func mh(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*"); if r.Method == "OPTIONS" { return }
	if r.URL.Path == "/toffee-epl.m3u" {
		if r.Header.Get("Sec-Fetch-Dest") == "document" || strings.Contains(r.Header.Get("Accept"), "text/html") { http.Redirect(w, r, "https://t.me/livesportsplay", 302); return }
		cat := getCat(); m3u := "#EXTM3U\n#\"name\": \"Toffee EPL & BFL - Owner: Md Sohanur Rahman Hady (HADY)\"\n\n"
		var wg sync.WaitGroup; var mu sync.Mutex
		for id, c := range cat {
			wg.Add(1)
			go func(id string, c []string) {
				defer wg.Done(); u := getSt(id)
				if u == "" { u = "https://t.me/livesportsplay/error_no_link_for_" + id + ".m3u8" }
				mu.Lock(); m3u += fmt.Sprintf("#EXTINF:-1 tvg-logo=\"%s\" group-title=\"%s\", %s\n%s\n\n", c[1], c[2], c[0], u); mu.Unlock()
			}(id, c)
		}
		wg.Wait(); w.Header().Set("Content-Type", "application/vnd.apple.mpegurl"); w.Write([]byte(m3u)); return
	}
	w.Write([]byte("Toffee Direct Link Playlist API Active"))
}
func main() { pt := os.Getenv("PORT"); if pt == "" { pt = "8080" }; http.HandleFunc("/", mh); http.ListenAndServe(":"+pt, nil) }
