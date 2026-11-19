package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/vivlilv/eneba_task/api/internal/migrations"
)

// These handler tests exercise PostgreSQL-specific SQL (BOOL_OR, FILTER,
// ::int) against a real server. Each test gets an isolated scratch database,
// applies the real migrations, and replaces demo data with controlled
// fixtures. Database-backed tests skip when PostgreSQL is unavailable.

const defaultTestDBURL = "postgres://eneba:eneba_dev@127.0.0.1:5432/eneba?sslmode=disable"

// newTestAppWithScratchDatabase returns an App backed by a fresh scratch
// database. It fails if a reachable server cannot set up the database and
// skips if PostgreSQL is unavailable.
func newTestAppWithScratchDatabase(t *testing.T) *App {
	t.Helper()

	adminURL := defaultTestDBURL
	if override := os.Getenv("TEST_DATABASE_URL"); override != "" {
		adminURL = override
	}

	adminDB, err := sql.Open("pgx", adminURL)
	if err != nil {
		t.Fatalf("open admin database: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := adminDB.PingContext(ctx); err != nil {
		adminDB.Close()
		t.Skipf("PostgreSQL not reachable at %s (%v); skipping database-backed handler test", adminURL, err)
	}

	scratchDBName := newScratchDBName(t)
	if _, err := adminDB.Exec(`CREATE DATABASE ` + scratchDBName); err != nil {
		adminDB.Close()
		t.Fatalf("create scratch database: %v", err)
	}
	t.Cleanup(func() {
		_, _ = adminDB.Exec(`DROP DATABASE IF EXISTS ` + scratchDBName + ` WITH (FORCE)`)
		_ = adminDB.Close()
	})

	testURL, err := replaceDatabaseName(adminURL, scratchDBName)
	if err != nil {
		t.Fatalf("build scratch database URL: %v", err)
	}
	db, err := sql.Open("pgx", testURL)
	if err != nil {
		t.Fatalf("open scratch database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	db.SetMaxOpenConns(4)
	if err := migrations.Up(ctx, db); err != nil {
		t.Fatalf("apply migrations to scratch database: %v", err)
	}
	clearTestTables(t, db)
	return New(db, false)
}

func newScratchDBName(t *testing.T) string {
	t.Helper()
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatalf("generate scratch database name: %v", err)
	}
	return "eneba_app_test_" + hex.EncodeToString(suffix[:])
}

func replaceDatabaseName(rawURL, name string) (string, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	parsed.Path = "/" + name
	return parsed.String(), nil
}

// clearTestTables removes demo seed data so each test seeds exactly the rows
// it asserts against.
func clearTestTables(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
		TRUNCATE TABLE order_items, payment_attempts, orders, digital_keys, offers,
		                  cart_items, sessions, users, homepage_block_products,
		                  homepage_blocks, sellers, products
		RESTART IDENTITY CASCADE
	`)
	if err != nil {
		t.Fatalf("clear scratch database tables: %v", err)
	}
}

func execStatements(t *testing.T, db *sql.DB, statements []string) {
	t.Helper()
	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			t.Fatalf("seed statement failed (%q): %v", statement, err)
		}
	}
}

func getHomepage(t *testing.T, target *App) (*httptest.ResponseRecorder, []byte) {
	t.Helper()
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/api/homepage", nil)
	target.Handler().ServeHTTP(recorder, request)
	body := recorder.Body.Bytes()
	return recorder, body
}

// Wire-format mirrors of the homepage response. They deliberately live here,
// not by reusing production types, so the tests pin the JSON contract.
type homepageProductJSON struct {
	ID               string `json:"id"`
	Slug             string `json:"slug"`
	Title            string `json:"title"`
	Edition          string `json:"edition"`
	Platform         string `json:"platform"`
	Region           string `json:"region"`
	Category         string `json:"category"`
	LowestPriceCents int64  `json:"lowestPriceCents"`
	Currency         string `json:"currency"`
	Available        bool   `json:"available"`
	OfferCount       int    `json:"offerCount"`
}

type homepageBlockJSON struct {
	ID       string                `json:"id"`
	Title    string                `json:"title"`
	Kind     string                `json:"kind"`
	Eyebrow  string                `json:"eyebrow"`
	Body     string                `json:"body"`
	CTALabel string                `json:"ctaLabel"`
	CTAURL   string                `json:"ctaUrl"`
	Products []homepageProductJSON `json:"products"`
}

type homepageResponseJSON struct {
	Blocks []homepageBlockJSON `json:"blocks"`
}

func TestHomepageAssemblesBlocksProductsAndAggregates(t *testing.T) {
	target := newTestAppWithScratchDatabase(t)
	db := appDatabase(t, target)

	execStatements(t, db, []string{
		`INSERT INTO products (id, slug, title, edition, platform, region, category, description) VALUES
			('p-multi',    'multi',    'Multi Offer Game',  'Standard',  'PC',      ' Europe ', 'Adventure', 'd1'),
			('p-soldout',  'soldout',  'Sold Out Game',     'Standard',  'Console', 'Europe',  'Racing',    'd2'),
			('p-nooffers', 'nooffers', 'No Offers Game',    'Deluxe',    'PC',      'Global',  'Puzzle',    'd3')`,
		`INSERT INTO sellers (id, name, rating) VALUES
			('s-one', 'Seller One', 4.90),
			('s-two', 'Seller Two', 4.50)`,
		`INSERT INTO offers (id, product_id, seller_id, price_cents, currency, active) VALUES
			('o-a', 'p-multi',   's-one', 2500, 'EUR', TRUE),
			('o-b', 'p-multi',   's-two', 2000, 'EUR', TRUE),
			('o-c', 'p-soldout', 's-one', 1500, 'EUR', TRUE),
			('o-d', 'p-soldout', 's-two',  999, 'EUR', FALSE)`,
		`INSERT INTO digital_keys (offer_id, code) VALUES
			('o-a', 'KEY-A-1'),
			('o-c', 'KEY-C-1')`,
		`INSERT INTO homepage_blocks (id, title, kind, eyebrow, body, cta_label, cta_url, position) VALUES
			('b-hero',     'Hero Banner',     'hero',          'HERO EYEBROW', 'Hero body.',      'Shop now', '#featured', 1),
			('b-featured', 'Featured Games', 'featured',      'CURATED',      'Featured body.',  '',          '',          2),
			('b-nooffers', 'Quiet Corner',   'product_rail',  '',             '',                '',          '',          3)`,
		`INSERT INTO homepage_block_products (block_id, product_id, position) VALUES
			('b-featured', 'p-multi',    1),
			('b-featured', 'p-soldout',  2),
			('b-nooffers', 'p-nooffers', 1)`,
		// Sell the only key of o-c so p-soldout is unavailable.
		`INSERT INTO users (email, password_hash) VALUES ('buyer@example.com', 'x')`,
		`INSERT INTO orders (id, user_id, status, currency, total_cents, checkout_key)
			VALUES ('11111111-1111-1111-1111-111111111111', (SELECT id FROM users WHERE email = 'buyer@example.com'),
			        'paid', 'EUR', 1500, 'ck-homepage-test')`,
		`INSERT INTO order_items (order_id, product_id, offer_id, product_title_snapshot, seller_name_snapshot, unit_price_cents, digital_key_id)
			SELECT o.id, 'p-soldout', 'o-c', 'Sold Out Game', 'Seller One', 1500, dk.id
			FROM orders o JOIN digital_keys dk ON dk.offer_id = 'o-c'
			WHERE o.checkout_key = 'ck-homepage-test'`,
	})

	recorder, body := getHomepage(t, target)

	if code := recorder.Code; code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", code, http.StatusOK, body)
	}
	if ct := recorder.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want application/json; charset=utf-8", ct)
	}

	var response homepageResponseJSON
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode response body (%s): %v", body, err)
	}
	if len(response.Blocks) != 3 {
		t.Fatalf("len(blocks) = %d, want 3; body: %s", len(response.Blocks), body)
	}

	hero, featured, nooffers := response.Blocks[0], response.Blocks[1], response.Blocks[2]

	if hero.ID != "b-hero" || hero.Title != "Hero Banner" || hero.Kind != "hero" ||
		hero.Eyebrow != "HERO EYEBROW" || hero.Body != "Hero body." ||
		hero.CTALabel != "Shop now" || hero.CTAURL != "#featured" {
		t.Fatalf("hero block fields = %+v", hero)
	}
	// Blocks with no products must serialize products as [], not null.
	if hero.Products == nil || len(hero.Products) != 0 {
		t.Fatalf("hero block products = %#v, want empty non-nil slice", hero.Products)
	}

	if featured.ID != "b-featured" {
		t.Fatalf("block order = [%s, %s, %s], want b-hero, b-featured, b-nooffers",
			response.Blocks[0].ID, response.Blocks[1].ID, response.Blocks[2].ID)
	}
	if len(featured.Products) != 2 {
		t.Fatalf("featured block has %d products, want 2; featured = %+v", len(featured.Products), featured)
	}

	multi := featured.Products[0]
	if multi.ID != "p-multi" || multi.Slug != "multi" || multi.Title != "Multi Offer Game" ||
		multi.Edition != "Standard" || multi.Platform != "PC" || multi.Category != "Adventure" {
		t.Fatalf("p-multi identity fields = %+v", multi)
	}
	// Region comes back trimmed even though it was stored with padding.
	if multi.Region != "Europe" {
		t.Fatalf("p-multi region = %q, want trimmed %q", multi.Region, "Europe")
	}
	// Lowest price over both active offers, not the first one.
	if multi.LowestPriceCents != 2000 {
		t.Fatalf("p-multi lowestPriceCents = %d, want 2000", multi.LowestPriceCents)
	}
	if multi.Currency != "EUR" {
		t.Fatalf("p-multi currency = %q, want EUR", multi.Currency)
	}
	if !multi.Available {
		t.Fatal("p-multi available = false, want true (o-a still has an unsold key)")
	}
	if multi.OfferCount != 2 {
		t.Fatalf("p-multi offerCount = %d, want 2 (only active offers count)", multi.OfferCount)
	}

	soldOut := featured.Products[1]
	if soldOut.ID != "p-soldout" {
		t.Fatalf("featured product order = [%s, %s], want p-multi first, p-soldout second",
			featured.Products[0].ID, featured.Products[1].ID)
	}
	if soldOut.LowestPriceCents != 1500 {
		t.Fatalf("p-soldout lowestPriceCents = %d, want 1500 (inactive 999 offer must be excluded)", soldOut.LowestPriceCents)
	}
	if soldOut.Available {
		t.Fatal("p-soldout available = true, want false (its only key was sold)")
	}
	if soldOut.OfferCount != 1 {
		t.Fatalf("p-soldout offerCount = %d, want 1 (inactive offer must not count)", soldOut.OfferCount)
	}

	if len(nooffers.Products) != 1 {
		t.Fatalf("b-nooffers has %d products, want 1; block = %+v", len(nooffers.Products), nooffers)
	}
	noOfferProduct := nooffers.Products[0]
	if noOfferProduct.ID != "p-nooffers" {
		t.Fatalf("b-nooffers product id = %q, want p-nooffers", noOfferProduct.ID)
	}
	// Products with zero offers still appear, with the SQL COALESCE defaults.
	if noOfferProduct.LowestPriceCents != 0 {
		t.Fatalf("p-nooffers lowestPriceCents = %d, want 0", noOfferProduct.LowestPriceCents)
	}
	if noOfferProduct.Currency != "EUR" {
		t.Fatalf("p-nooffers currency = %q, want default EUR", noOfferProduct.Currency)
	}
	if noOfferProduct.Available || noOfferProduct.OfferCount != 0 {
		t.Fatalf("p-nooffers available/offerCount = %v/%d, want false/0", noOfferProduct.Available, noOfferProduct.OfferCount)
	}
}

func TestHomepageEmptyDatabaseReturnsEmptyBlocksArray(t *testing.T) {
	target := newTestAppWithScratchDatabase(t)

	recorder, body := getHomepage(t, target)

	if code := recorder.Code; code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", code, http.StatusOK, body)
	}
	var response homepageResponseJSON
	if err := json.Unmarshal(body, &response); err != nil {
		t.Fatalf("decode response body (%s): %v", body, err)
	}
	// The contract is "blocks": [], not null.
	if response.Blocks == nil || len(response.Blocks) != 0 {
		t.Fatalf("blocks = %#v, want empty non-nil array", response.Blocks)
	}
	if !jsonValidField(body, "blocks") {
		t.Fatalf("response does not contain a blocks field: %s", body)
	}
}

func TestHomepageReturnsInternalServerErrorWhenDatabaseUnavailable(t *testing.T) {
	// Port 1 never accepts connections, so the first QueryContext fails.
	db, err := sql.Open("pgx", "postgres://eneba:eneba_dev@127.0.0.1:1/eneba?sslmode=disable")
	if err != nil {
		t.Fatalf("open unavailable database: %v", err)
	}
	defer db.Close()
	target := New(db, false)

	recorder, body := getHomepage(t, target)

	if code := recorder.Code; code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body: %s", code, http.StatusInternalServerError, body)
	}
	var apiErr struct {
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(body, &apiErr); err != nil {
		t.Fatalf("decode error body (%s): %v", body, err)
	}
	if apiErr.Error != "could not load homepage" {
		t.Fatalf("error message = %q, want %q", apiErr.Error, "could not load homepage")
	}
	if apiErr.Code != "internal_error" {
		t.Fatalf("error code = %q, want %q", apiErr.Code, "internal_error")
	}
}

// appDatabase retrieves the *sql.DB from an App for fixture seeding. It relies
// on the field being unexported but same-package accessible in tests.
func appDatabase(t *testing.T, target *App) *sql.DB {
	t.Helper()
	if target.db == nil {
		t.Fatal("App has no database handle")
	}
	return target.db
}

func jsonValidField(body []byte, field string) bool {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return false
	}
	_, ok := raw[field]
	return ok
}
