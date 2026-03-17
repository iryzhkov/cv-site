package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/iryzhkov/cv-site/middleware"
)

func Admin(w http.ResponseWriter, r *http.Request) {
	events := middleware.ReadAnalytics()
	tokens := middleware.GetTokens()
	filterCompany := r.URL.Query().Get("company")

	type companyVisit struct {
		Company  string
		Sessions int
		Views    int
	}

	companies := make(map[string]*companyVisit)
	companySessions := make(map[string]map[string]bool) // company -> set of session IDs
	pageCounts := make(map[string]int)
	filteredViews := 0
	allSessions := make(map[string]bool)
	filteredSessions := make(map[string]bool)

	for _, evt := range events {
		if evt.SessionID != "" {
			allSessions[evt.SessionID] = true
		}

		// Apply company filter
		if filterCompany != "" && evt.Company != filterCompany {
			continue
		}
		filteredViews++
		if evt.SessionID != "" {
			filteredSessions[evt.SessionID] = true
		}
		pageCounts[evt.Path]++
		if evt.Company != "" {
			cv, ok := companies[evt.Company]
			if !ok {
				cv = &companyVisit{Company: evt.Company}
				companies[evt.Company] = cv
				companySessions[evt.Company] = make(map[string]bool)
			}
			cv.Views++
			if evt.SessionID != "" && !companySessions[evt.Company][evt.SessionID] {
				companySessions[evt.Company][evt.SessionID] = true
				cv.Sessions++
			}
		}
	}

	type pageCount struct {
		Path  string
		Count int
	}
	var topPages []pageCount
	for path, count := range pageCounts {
		topPages = append(topPages, pageCount{path, count})
	}
	sort.Slice(topPages, func(i, j int) bool { return topPages[i].Count > topPages[j].Count })
	if len(topPages) > 20 {
		topPages = topPages[:20]
	}

	var companyList []companyVisit
	for _, cv := range companies {
		companyList = append(companyList, *cv)
	}
	sort.Slice(companyList, func(i, j int) bool { return companyList[i].Views > companyList[j].Views })

	// Build unique company list for the filter dropdown
	allCompanies := make(map[string]bool)
	for _, evt := range events {
		if evt.Company != "" {
			allCompanies[evt.Company] = true
		}
	}
	var companyNames []string
	for c := range allCompanies {
		companyNames = append(companyNames, c)
	}
	sort.Strings(companyNames)

	// Aggregate LLM token usage per model
	type modelUsage struct {
		Model        string
		InputTokens  int
		OutputTokens int
		Requests     int
	}
	modelUsages := make(map[string]*modelUsage)
	for _, evt := range events {
		if evt.Model == "" {
			continue
		}
		if filterCompany != "" && evt.Company != filterCompany {
			continue
		}
		mu, ok := modelUsages[evt.Model]
		if !ok {
			mu = &modelUsage{Model: evt.Model}
			modelUsages[evt.Model] = mu
		}
		mu.InputTokens += evt.InputTokens
		mu.OutputTokens += evt.OutputTokens
		mu.Requests++
	}
	var modelUsageList []modelUsage
	for _, mu := range modelUsages {
		modelUsageList = append(modelUsageList, *mu)
	}
	sort.Slice(modelUsageList, func(i, j int) bool {
		return modelUsageList[i].OutputTokens > modelUsageList[j].OutputTokens
	})

	Templates["admin.html"].ExecuteTemplate(w, "base", map[string]any{
		"TotalVisits":      filteredViews,
		"TotalSessions":    len(filteredSessions),
		"AllVisits":        len(events),
		"AllSessions":      len(allSessions),
		"TopPages":       topPages,
		"Companies":      companyList,
		"CompanyNames":   companyNames,
		"FilterCompany":  filterCompany,
		"Tokens":         tokens,
		"ModelUsage":     modelUsageList,
		"Active":         "admin",
	})
}

// AdminChartData returns time-series JSON for the analytics chart.
func AdminChartData(w http.ResponseWriter, r *http.Request) {
	events := middleware.ReadAnalytics()
	filterCompany := r.URL.Query().Get("company")
	period := r.URL.Query().Get("period")
	if period == "" {
		period = "24h"
	}

	// Determine time range
	now := time.Now().UTC()
	var since time.Time
	var bucketFormat string
	switch period {
	case "24h":
		since = now.Add(-24 * time.Hour)
		bucketFormat = "15:00" // hourly
	case "7d":
		since = now.Add(-7 * 24 * time.Hour)
		bucketFormat = "Jan 02" // daily
	case "30d":
		since = now.Add(-30 * 24 * time.Hour)
		bucketFormat = "Jan 02" // daily
	case "all":
		since = time.Time{}
		bucketFormat = "Jan 02"
	default:
		since = now.Add(-7 * 24 * time.Hour)
		bucketFormat = "Jan 02"
	}

	// Bucket events by time
	buckets := make(map[string]int)
	pageBuckets := make(map[string]map[string]int)   // page -> bucket -> count
	inputTokBuckets := make(map[string]int)           // bucket -> input tokens
	outputTokBuckets := make(map[string]int)          // bucket -> output tokens

	for _, evt := range events {
		ts, err := time.Parse(time.RFC3339, evt.Timestamp)
		if err != nil {
			continue
		}
		if !since.IsZero() && ts.Before(since) {
			continue
		}
		if filterCompany != "" && evt.Company != filterCompany {
			continue
		}

		bucket := ts.Format(bucketFormat)
		buckets[bucket]++

		if _, ok := pageBuckets[evt.Path]; !ok {
			pageBuckets[evt.Path] = make(map[string]int)
		}
		pageBuckets[evt.Path][bucket]++

		if evt.InputTokens > 0 || evt.OutputTokens > 0 {
			inputTokBuckets[bucket] += evt.InputTokens
			outputTokBuckets[bucket] += evt.OutputTokens
		}
	}

	// Sort bucket labels chronologically
	var labels []string
	for label := range buckets {
		labels = append(labels, label)
	}
	sort.Strings(labels)

	// Build total visits series
	totalData := make([]int, len(labels))
	for i, label := range labels {
		totalData[i] = buckets[label]
	}

	// Build top 5 page series
	type pageSeries struct {
		Page string `json:"page"`
		Data []int  `json:"data"`
	}

	// Find top pages by total count
	type pc struct {
		path  string
		count int
	}
	var pageTotals []pc
	for path, bkts := range pageBuckets {
		total := 0
		for _, c := range bkts {
			total += c
		}
		pageTotals = append(pageTotals, pc{path, total})
	}
	sort.Slice(pageTotals, func(i, j int) bool { return pageTotals[i].count > pageTotals[j].count })

	var pages []pageSeries
	limit := 5
	if len(pageTotals) < limit {
		limit = len(pageTotals)
	}
	for _, pt := range pageTotals[:limit] {
		data := make([]int, len(labels))
		for i, label := range labels {
			data[i] = pageBuckets[pt.path][label]
		}
		pages = append(pages, pageSeries{Page: pt.path, Data: data})
	}

	// Build token usage series
	inputTokData := make([]int, len(labels))
	outputTokData := make([]int, len(labels))
	for i, label := range labels {
		inputTokData[i] = inputTokBuckets[label]
		outputTokData[i] = outputTokBuckets[label]
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"labels":     labels,
		"total":      totalData,
		"pages":      pages,
		"inputTok":   inputTokData,
		"outputTok":  outputTokData,
		"period":     period,
		"company":    filterCompany,
	})
}

func AdminCreateToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	company := r.FormValue("company")
	if company == "" {
		http.Error(w, "company required", http.StatusBadRequest)
		return
	}

	b := make([]byte, 4)
	rand.Read(b)
	token := hex.EncodeToString(b)

	middleware.AddToken(token, company)
	middleware.SaveTokens()

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func AdminDeleteToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.FormValue("token")
	if token == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}

	middleware.DeleteToken(token)
	middleware.SaveTokens()

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

func AdminRevokeToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	token := r.FormValue("token")
	if token == "" {
		http.Error(w, "token required", http.StatusBadRequest)
		return
	}

	middleware.RevokeToken(token)
	middleware.SaveTokens()

	http.Redirect(w, r, "/admin", http.StatusSeeOther)
}

// --- REST API (local network only, for automation / Claude) ---

// APIListTokens returns all tokens as JSON.
// GET /api/admin/tokens
func APIListTokens(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(middleware.GetTokens())
}

// APICreateToken creates a new token and returns it as JSON.
// POST /api/admin/tokens { "company": "name" }
func APICreateToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Company string `json:"company"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Company == "" {
		http.Error(w, `{"error":"company required"}`, http.StatusBadRequest)
		return
	}

	b := make([]byte, 4)
	rand.Read(b)
	token := hex.EncodeToString(b)

	middleware.AddToken(token, req.Company)
	middleware.SaveTokens()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"token":   token,
		"company": req.Company,
		"url":     "https://cv.ryzhkov.dev/?t=" + token,
	})
}

// APIRevokeToken revokes a token.
// POST /api/admin/tokens/revoke { "token": "xxx" }
func APIRevokeToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		http.Error(w, `{"error":"token required"}`, http.StatusBadRequest)
		return
	}

	middleware.RevokeToken(req.Token)
	middleware.SaveTokens()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}

// APIDeleteToken deletes a token.
// POST /api/admin/tokens/delete { "token": "xxx" }
func APIDeleteToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		http.Error(w, `{"error":"token required"}`, http.StatusBadRequest)
		return
	}

	middleware.DeleteToken(req.Token)
	middleware.SaveTokens()

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"ok":true}`))
}
