package app

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestHomepageCheapGamesUsesAvailableOffersUnderTenEuros(t *testing.T) {
	app := newTestAppWithScratchDatabase(t)
	db := appDatabase(t, app)
	execStatements(t, db, []string{
		`INSERT INTO products (id, slug, title, edition, platform, region, category, description, listed) VALUES
			('p-cheap-a', 'cheap-a', 'Cheap A', 'Standard', 'PC', 'Global', 'Action', 'fixture', TRUE),
			('p-cheap-b', 'cheap-b', 'Cheap B', 'Standard', 'PC', 'Global', 'Action', 'fixture', TRUE),
			('p-limit', 'at-limit', 'At Limit', 'Standard', 'PC', 'Global', 'Action', 'fixture', TRUE),
			('p-expensive', 'expensive', 'Expensive', 'Standard', 'PC', 'Global', 'Action', 'fixture', TRUE),
			('p-no-stock', 'no-stock', 'No Stock', 'Standard', 'PC', 'Global', 'Action', 'fixture', TRUE),
			('p-unlisted', 'unlisted', 'Unlisted', 'Standard', 'PC', 'Global', 'Action', 'fixture', FALSE)`,
		`INSERT INTO sellers (id, name, rating) VALUES ('s-cheap-test', 'Test Seller', 4.80)`,
		`INSERT INTO offers (id, product_id, seller_id, price_cents, currency, active) VALUES
			('o-cheap-a', 'p-cheap-a', 's-cheap-test', 799, 'EUR', TRUE),
			('o-cheap-b', 'p-cheap-b', 's-cheap-test', 699, 'EUR', TRUE),
			('o-limit', 'p-limit', 's-cheap-test', 1000, 'EUR', TRUE),
			('o-expensive', 'p-expensive', 's-cheap-test', 1001, 'EUR', TRUE),
			('o-no-stock', 'p-no-stock', 's-cheap-test', 599, 'EUR', TRUE),
			('o-unlisted', 'p-unlisted', 's-cheap-test', 499, 'EUR', TRUE)`,
		`INSERT INTO digital_keys (offer_id, code) VALUES
			('o-cheap-a', 'CHEAP-A-KEY'),
			('o-cheap-b', 'CHEAP-B-KEY'),
			('o-limit', 'LIMIT-KEY'),
			('o-expensive', 'EXPENSIVE-KEY'),
			('o-unlisted', 'UNLISTED-KEY')`,
		`INSERT INTO homepage_blocks (id, title, kind, position) VALUES
			('b-hero', 'Hero', 'hero', 1),
			('b-cheap', 'Cheap games', 'cheap_games', 2)`,
	})

	recorder, body := getHomepage(t, app)
	if recorder.Code != http.StatusOK {
		t.Fatalf("homepage status = %d, want %d; body: %s", recorder.Code, http.StatusOK, body)
	}

	var response homepageResponseJSON
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode homepage response: %v", err)
	}
	if len(response.Blocks) != 2 || response.Blocks[1].ID != "b-cheap" {
		t.Fatalf("homepage blocks = %+v, want hero then cheap block", response.Blocks)
	}

	cheapGames := response.Blocks[1].Products
	if len(cheapGames) != 2 {
		t.Fatalf("cheap games count = %d, want 2; games: %+v", len(cheapGames), cheapGames)
	}
	if cheapGames[0].Slug != "cheap-b" || cheapGames[0].LowestPriceCents != 699 {
		t.Errorf("first cheap game = %+v, want cheap-b at 699 cents", cheapGames[0])
	}
	if cheapGames[1].Slug != "cheap-a" || cheapGames[1].LowestPriceCents != 799 {
		t.Errorf("second cheap game = %+v, want cheap-a at 799 cents", cheapGames[1])
	}
	for _, product := range cheapGames {
		if !product.Available {
			t.Errorf("cheap game %q has no available offer", product.Slug)
		}
	}
}
