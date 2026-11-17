package app

import (
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestDecodeJSONRequiresOneKnownJSONValue(t *testing.T) {
	t.Run("valid body", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("POST", "/", strings.NewReader(`{"email":"viv@example.com"}`))
		var body struct {
			Email string `json:"email"`
		}
		if err := decodeJSON(recorder, request, &body); err != nil {
			t.Fatalf("decodeJSON() error = %v", err)
		}
		if body.Email != "viv@example.com" {
			t.Fatalf("Email = %q, want viv@example.com", body.Email)
		}
	})

	t.Run("unknown field", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("POST", "/", strings.NewReader(`{"email":"viv@example.com","admin":true}`))
		var body struct {
			Email string `json:"email"`
		}
		if err := decodeJSON(recorder, request, &body); err == nil {
			t.Fatal("decodeJSON() accepted an unknown field")
		}
	})

	t.Run("trailing JSON", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest("POST", "/", strings.NewReader(`{"email":"viv@example.com"}{}`))
		var body struct {
			Email string `json:"email"`
		}
		if err := decodeJSON(recorder, request, &body); err == nil {
			t.Fatal("decodeJSON() accepted multiple JSON values")
		}
	})
}

func TestNormalizedOfferIDsDeduplicatesAndPreservesFirstSelection(t *testing.T) {
	items := []guestCartItem{
		{OfferID: " offer-a ", PriceSeenCents: 1000},
		{OfferID: "offer-b", PriceSeenCents: 2000},
		{OfferID: "offer-a", PriceSeenCents: 900},
		{OfferID: " ", PriceSeenCents: 0},
	}
	got := normalizedOfferIDs(items)
	want := []guestCartItem{
		{OfferID: "offer-a", PriceSeenCents: 1000},
		{OfferID: "offer-b", PriceSeenCents: 2000},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizedOfferIDs() = %#v, want %#v", got, want)
	}
}
