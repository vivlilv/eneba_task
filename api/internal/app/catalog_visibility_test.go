package app

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestSearchOmitsUnlistedProducts(t *testing.T) {
	app := newTestAppWithScratchDatabase(t)
	db := appDatabase(t, app)
	execStatements(t, db, []string{
		`INSERT INTO products (id, slug, title, edition, platform, region, category, description, listed) VALUES
			('p-listed', 'listed-game', 'Listed Game', 'Standard Edition', 'PC', 'Global', 'Action', 'fixture', TRUE),
			('p-unlisted', 'unlisted-game', 'Unlisted Game', 'Standard Edition', 'PC', 'Global', 'Action', 'fixture', FALSE)`,
	})

	recorder := getSearch(t, app, "", "100")
	if recorder.Code != http.StatusOK {
		t.Fatalf("search status = %d, want %d; body: %s", recorder.Code, http.StatusOK, recorder.Body)
	}

	var response searchResponseJSON
	if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode search response: %v", err)
	}
	if len(response.Products) != 1 || response.Products[0].Slug != "listed-game" {
		t.Fatalf("search products = %+v, want only listed-game", response.Products)
	}
}
