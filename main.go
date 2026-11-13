package main

import (
	"encoding/json"; "fmt"; "net/http"; "net/url"; "os"; "regexp"; "strings"; "sync"; "resty.dev/v3"
)

func extItm(d interface{}) []map[string]interface{} {
	var res []map[string]interface{}
	switch v := d.(type) {
	case []interface{}:
		for _, i := range v { res = append(res, extItm(i)...) }
	case map[string]interface{}:
		if _, ok1 := v["metadata"]; ok1 { if _, ok2 := v["id"]; ok2 { res = append(res, v) } }
		for k, val := range v {
			if k == "containers" || k == "assets" || k == "episodes" || k == "items" { res = append(res, extItm(val)...) }
		}
	}
	return res
}

func playH(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*"); if r.Method == "OPTIONS" { return }
	id, isL := r.URL.Query().Get("id"), r.URL.Query().Get("isLive")
	tURL := "https://playcipher.bioscopelive.com/v1/manifest?providers_content_id=" + id + "&provider=sonyliv"
	if isL != "true" { tURL += "&scheme=widevine" }
	resp, err := resty.New().R().SetHeader("User-Agent", "Mozilla/5.0").Get(tURL)
	if err != nil { http.Error(w, `{"error": "Failed"}`, 500); return }
	j := string(resp.Bytes())
	if m := regexp.MustCompile(`"url"\s*:\s*"([^"]+)"`).FindStringSubmatch(j); len(m) > 1 {
		u, _ := url.Parse(strings.ReplaceAll(m[1], `\u0026`, "&"))
		if hdnea := u.Query().Get("hdnea"); hdnea != "" { u.RawQuery = "hdnea=" + hdnea } else { u.RawQuery = "" }
		cU := strings.Replace(u.String(), "grameenphonedrm", "sonygdrm", -1)
		if isL == "true" { cU = strings.Replace(cU, "std_mdh-800300010", "master", -1) }
		j = strings.Replace(j, `"`+m[1]+`"`, `"`+cU+`"`, 1)
	}
	w.Header().Set("Content-Type", "application/json"); w.Write([]byte(j))
}

func liveM3uH(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*"); if r.Method == "OPTIONS" { return }
	rsp, err := resty.New().R().SetHeader("User-Agent", "Mozilla/5.0").Get("https://apiv3.sonyliv.com/AGL/4.9/A/ENG/MWEB/BD/FL/PAGE/17378")
	if err != nil { return }
	var d map[string]interface{}; json.Unmarshal(rsp.Bytes(), &d)
	ro, ok := d["resultObj"].(map[string]interface{}); if !ok { return }
	
	items, m3u, seen := extItm(ro), "#EXTM3U\n", make(map[string]bool)
	var wg sync.WaitGroup; var mu sync.Mutex
	
	for _, i := range items {
		meta, ok := i["metadata"].(map[string]interface{}); if !ok { continue }
		
		// Fix: Safely check boolean to prevent panic
		isLive, okL := meta["isLive"].(bool); if !okL || !isLive { continue }
		
		cID := fmt.Sprintf("%.0f", meta["contentId"].(float64))
		if seen[cID] { continue } 
		seen[cID] = true
		
		mTitle, _ := meta["title"].(string); epTitle, _ := meta["episodeTitle"].(string)
		epTitle = regexp.MustCompile(`(?i)\s*-\s*\d{1,2}\s+[a-z]{3}\s+\d{4}\s*$`).ReplaceAllString(epTitle, "")
		fT := mTitle; if epTitle != "" && epTitle != mTitle { fT = epTitle + " - " + mTitle } else if epTitle != "" { fT = epTitle }
		fT = strings.ReplaceAll(fT, ",", " -"); lg, apID := "", ""
		
		if emf, ok := meta["emfAttributes"].(map[string]interface{}); ok {
			if l, k := emf["landscape_thumb"].(string); k { lg = l } else if s, k := emf["square_thumb"].(string); k { lg = s }
			if a, k := emf["auto_play_id"].(float64); k { apID = fmt.Sprintf("%.0f", a) }
		}
		
		wg.Add(1)
		go func(id, ap, t, l string) {
			defer wg.Done()
			fC := func(i string) string {
				if i == "" { return "" }
				r, e := resty.New().R().SetHeader("User-Agent", "Mozilla/5.0").Get("https://playcipher.bioscopelive.com/v1/manifest?providers_content_id=" + i + "&provider=sonyliv")
				if e == nil { var pd map[string]interface{}; json.Unmarshal(r.Bytes(), &pd); if u, ok := pd["url"].(string); ok { return u } }
				return ""
			}
			rU := fC(id); if rU == "" { rU = fC(ap) }
			if rU != "" {
				u, _ := url.Parse(strings.ReplaceAll(rU, `\u0026`, "&"))
				if h := u.Query().Get("hdnea"); h != "" { u.RawQuery = "hdnea=" + h } else { u.RawQuery = "" }
				cU := strings.Replace(strings.Replace(u.String(), "grameenphonedrm", "sonygdrm", -1), "std_mdh-800300010", "master", -1)
				mu.Lock(); m3u += fmt.Sprintf("#EXTINF:-1 tvg-logo=\"%s\" group-title=\"Live Sports\",%s\n%s\n", l, t, cU); mu.Unlock()
			}
		}(cID, apID, fT, lg)
	}
	wg.Wait(); w.Header().Set("Content-Type", "application/vnd.apple.mpegurl"); w.Write([]byte(m3u))
}

func main() {
	p := os.Getenv("PORT"); if p == "" { p = "8080" }
	http.HandleFunc("/api/play", playH); http.HandleFunc("/live.m3u", liveM3uH)
	http.ListenAndServe(":"+p, nil)
}
