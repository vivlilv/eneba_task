package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestSearchReturnsFullCatalogWithPartialAndTypoMatches(t *testing.T) {
	app := newTestAppWithScratchDatabase(t)
	db := appDatabase(t, app)
	execStatements(t, db, []string{
		`INSERT INTO products (id, slug, title, edition, platform, region, category, description) VALUES
			('p-hollow', 'hollow-knight', 'Hollow Knight', 'Standard Edition', 'PC', 'Global', 'Metroidvania', 'desc'),
			('p-silksong', 'silksong', 'Hollow Knight: Silksong', 'Standard Edition', 'PC', 'Global', 'Metroidvania', 'desc'),
			('p-adventure', 'adventure', 'Adventure', 'Standard Edition', 'PC', 'Global', 'Adventure', 'desc'),
			('p-minecraft', 'minecraft', 'Minecraft', 'Java Edition', 'PC', 'Global', 'Sandbox', 'desc'),
			('p-terraria', 'terraria', 'Terraria', 'Standard Edition', 'PC', 'Global', 'Sandbox', 'desc')`,
	})

	partial := getSearch(t, app, "Hol", "5")
	if partial.Code != http.StatusOK {
		t.Fatalf("partial search status = %d, want %d; body: %s", partial.Code, http.StatusOK, partial.Body)
	}
	var partialResponse searchResponseJSON
	if err := json.Unmarshal(partial.Body.Bytes(), &partialResponse); err != nil {
		t.Fatalf("decode partial search response: %v", err)
	}
	if len(partialResponse.Products) != 2 {
		t.Fatalf("partial search returned %d products, want 2; body: %s", len(partialResponse.Products), partial.Body)
	}

	typo := getSearch(t, app, "adenture", "5")
	var typoResponse searchResponseJSON
	if err := json.Unmarshal(typo.Body.Bytes(), &typoResponse); err != nil {
		t.Fatalf("decode typo search response: %v", err)
	}
	if len(typoResponse.Products) != 1 || typoResponse.Products[0].Slug != "adventure" {
		t.Fatalf("typo search products = %+v, want only Adventure; body: %s", typoResponse.Products, typo.Body)
	}

	all := getSearch(t, app, "", "100")
	var allResponse searchResponseJSON
	if err := json.Unmarshal(all.Body.Bytes(), &allResponse); err != nil {
		t.Fatalf("decode full-catalog response: %v", err)
	}
	if len(allResponse.Products) != 5 {
		t.Fatalf("empty-query search returned %d products, want all 5; body: %s", len(allResponse.Products), all.Body)
	}
}

func TestSearchValidatesQueryAndLimit(t *testing.T) {
	app := newTestAppWithScratchDatabase(t)

	tooLong := getSearch(t, app, strings.Repeat("x", 101), "5")
	if tooLong.Code != http.StatusBadRequest {
		t.Fatalf("long query status = %d, want %d", tooLong.Code, http.StatusBadRequest)
	}

	badLimit := getSearch(t, app, "game", "101")
	if badLimit.Code != http.StatusBadRequest {
		t.Fatalf("invalid limit status = %d, want %d", badLimit.Code, http.StatusBadRequest)
	}
}

type searchProductJSON struct {
	Slug  string `json:"slug"`
	Title string `json:"title"`
}

type searchResponseJSON struct {
	Products []searchProductJSON `json:"products"`
}

func getSearch(t *testing.T, app *App, query, limit string) *httptest.ResponseRecorder {
	t.Helper()
	values := url.Values{"limit": []string{limit}}
	if query != "" {
		values.Set("q", query)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/search?"+values.Encode(), nil)
	recorder := httptest.NewRecorder()
	app.Handler().ServeHTTP(recorder, request)
	return recorder
}
