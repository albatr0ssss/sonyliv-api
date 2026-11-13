package main
import ("crypto/hmac";"crypto/sha256";"encoding/hex";"encoding/json";"fmt";"net/http";"net/url";"os";"regexp";"strings";"sync";"time";"resty.dev/v3")

const TOKEN, RAIL, UA, TK_SEC = "eyJhbGciOiJFUzI1NiIsInR5cCI6IkpXVCJ9.eyJhdWQiOiJodHRwczovL3RvZmZlZWxpdmUuY29tIiwiY291bnRyeSI6IkJEIiwiZF9pZCI6ImRkMzIzYWJkLWQwMjUtNDk0MS1iMDBiLWYyZTEyZTk2N2M1MSIsImV4cCI6MTc5NzIyNjYzMywiaWF0IjoxNzk0NTk2ODMzLCJpc3MiOiJ0b2ZmZWVsaXZlLmNvbSIsImp0aSI6IjgyZWZiNzQ4LTllNmYtNDc4NS05NGM1LWQ4MzczMTE1NWRiM18xNzk0NTk2ODMzIiwicHJvdmlkZXIiOiJ0b2ZmZWUiLCJyX2lkIjoiZGQzMjNhYmQtZDAyNS00OTQxLWIwMGItZjJlMTJlOTY3YzUxIiwic19pZCI6ImRkMzIzYWJkLWQwMjUtNDk0MS1iMDBiLWYyZTEyZTk2N2M1MSIsInRva2VuIjoiYWNjZXNzIiwidHlwZSI6ImRldmljZSJ9.SRNBuOF56fCGI1DAk-H1P2vetDatWUoenY9fJXbbuiQQ_RIwvuaRaN53DgU_1OcZ6CLncYJncWg2O_hYyyzjTA", "032cc9194378b850b2fec39c6386fd1f", "Mozilla/5.0 (Linux; Android 10; K)", "HadyAkamaiCloneKey26"
var kv sync.Map

func getC(k string) (interface{}, bool) { if i, ok := kv.Load(k); ok { if time.Now().Unix() < i.([]interface{})[1].(int64) { return i.([]interface{})[0], true }; kv.Delete(k) }; return nil, false }
func setC(k string, v interface{}, ttl int64) { kv.Store(k, []interface{}{v, time.Now().Unix() + ttl}) }

func genTk() string {
	e := time.Now().Unix() + 3600; h := hmac.New(sha256.New, []byte(TK_SEC)); h.Write([]byte(fmt.Sprint(e)))
	return fmt.Sprintf("exp=%d~hmac=%s", e, hex.EncodeToString(h.Sum(nil)))
}
func valTk(t string) bool {
	m := regexp.MustCompile(`exp=(\d+)~hmac=(.+)`).FindStringSubmatch(t); if len(m) < 3 { return false }; var e int64; fmt.Sscanf(m[1], "%d", &e); if time.Now().Unix() > e { return false }
	h := hmac.New(sha256.New, []byte(TK_SEC)); h.Write([]byte(m[1])); return m[2] == hex.EncodeToString(h.Sum(nil))
}

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
	if e == nil {
		var pb map[string]interface{}; json.Unmarshal(r.Bytes(), &pb)
		if d, ok := pb["playbackDetails"].(map[string]interface{}); ok {
			if dt, ok := d["data"].([]interface{}); ok && len(dt) > 0 {
				if u, ok := dt[0].(map[string]interface{})["url"].(string); ok && u != "" { setC("ST_"+id, u, 180); return u }
			}
		}
	}
	return ""
}

func rw(b, tg string) string {
	u, _ := url.Parse(tg); d := tg[:strings.LastIndex(tg, "/")+1]; out := []string{}; re := regexp.MustCompile(`URI=["']([^"']+)["']`)
	for _, l := range strings.Split(b, "\n") {
		t := strings.TrimSpace(l)
		if t != "" && !strings.HasPrefix(t, "#") {
			if strings.HasPrefix(t, "http") { out = append(out, t) } else if strings.HasPrefix(t, "/") { out = append(out, u.Scheme+"://"+u.Host+t) } else { out = append(out, d+t) }
		} else if strings.Contains(t, "URI=") {
			out = append(out, re.ReplaceAllStringFunc(t, func(m string) string {
				p := re.FindStringSubmatch(m)[1]
				if strings.HasPrefix(p, "http") { return `URI="`+p+`"` } else if strings.HasPrefix(p, "/") { return `URI="`+u.Scheme+"://"+u.Host+p+`"` }
				return `URI="`+d+p+`"`
			}))
		} else { out = append(out, l) }
	}
	return strings.Join(out, "\n")
}

func mh(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*"); if r.Method == "OPTIONS" { return }; p, o, tk := r.URL.Path, "https://"+r.Host, r.URL.Query().Get("token")
	if p == "/toffee-epl.m3u" {
		if r.Header.Get("Sec-Fetch-Dest") == "document" || strings.Contains(r.Header.Get("Accept"), "text/html") { http.Redirect(w, r, "https://t.me/livesportsplay", 302); return }
		m, cTk := "#EXTM3U\n#\"name\": \"Toffee EPL & BFL - Owner: Md Sohanur Rahman Hady (HADY)\"\n\n", genTk()
		for id, c := range getCat() { m += fmt.Sprintf("#EXTINF:-1 tvg-logo=\"%s\" group-title=\"%s\", %s\n%s/play/%s.m3u8?token=%s\n\n", c[1], c[2], c[0], o, id, cTk) }
		w.Header().Set("Content-Type", "application/vnd.apple.mpegurl"); w.Write([]byte(m)); return
	}
	if strings.HasPrefix(p, "/play/") {
		if !valTk(tk) { http.Error(w, "Token Expired or Invalid", 403); return }
		id := strings.ReplaceAll(strings.ReplaceAll(p, "/play/", ""), ".m3u8", ""); st := getSt(id)
		if st == "" { http.Error(w, "Stream Not Found or Locked", 404); return }
		rs, _ := resty.New().R().SetHeader("User-Agent", UA).Get(st); w.Header().Set("Content-Type", "application/vnd.apple.mpegurl"); w.Write([]byte(rw(string(rs.Bytes()), st))); return
	}
	w.Write([]byte("Toffee Live Proxy Active"))
}
func main() { pt := os.Getenv("PORT"); if pt == "" { pt = "8080" }; http.HandleFunc("/", mh); http.ListenAndServe(":"+pt, nil) }
