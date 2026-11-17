package app

import (
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
)

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func normalizedOfferIDs(items []guestCartItem) []guestCartItem {
	seen := make(map[string]struct{}, len(items))
	result := make([]guestCartItem, 0, len(items))
	for _, item := range items {
		item.OfferID = strings.TrimSpace(item.OfferID)
		if item.OfferID == "" {
			continue
		}
		if _, exists := seen[item.OfferID]; exists {
			continue
		}
		seen[item.OfferID] = struct{}{}
		result = append(result, item)
	}
	return result
}
