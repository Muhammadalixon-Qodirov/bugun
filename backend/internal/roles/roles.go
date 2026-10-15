// Package roles names the user roles (users.role) in one place.
//
// A person chooses one of two roles at onboarding:
//   - Entrepreneur ("Tadbirkor") both sells (listings, prices, RFQ offers,
//     channels) and buys (requests for quotes); approved at once.
//   - MarketAdmin ("Bozor ma'muriyati") only oversees a market and its
//     viloyat; an admin must confirm the role first.
//
// Admin is the system role and is never chosen by the user. A NULL role
// means the user has not finished onboarding.
package roles

const (
	Entrepreneur = "entrepreneur"
	MarketAdmin  = "market_admin"
	Admin        = "admin"
)

// Legacy roles merged into Entrepreneur by migration 00045. The database
// still accepts them (and the retired "org_analyst") for one release so a
// code rollback keeps working; IsEntrepreneur treats a seller or buyer
// written meanwhile as an entrepreneur until the values are dropped.
const (
	legacySeller = "seller"
	legacyBuyer  = "buyer"
)

// IsEntrepreneur reports whether the role may sell and buy.
func IsEntrepreneur(role string) bool {
	switch role {
	case Entrepreneur, legacySeller, legacyBuyer:
		return true
	}
	return false
}

// NeedsApproval reports whether an admin must confirm the role before it
// takes effect.
func NeedsApproval(role string) bool { return role == MarketAdmin }
