package common

import "github.com/seidu626/subscription-manager/common/errors"

// Pagination is used to paginate results.
//
// Usage:
//
//	Pagination{
//		Limit: limit,
//		Offset: (page - 1) * limit
//	}
type Pagination struct {
	// Limit is the maximum number of results to return on this page.
	Limit int

	// Offset is the number of results to skip from the beginning of the results.
	// Typically: (page number - 1) * limit.
	Offset int
}

// Validate pagination.
func (p *Pagination) Validate() error {
	if p.Limit < 1 {
		return errors.ValidationError("", "Paginate", "pagination limit must be at least 1")
	}
	if p.Offset < 0 {
		return errors.ValidationError("", "Paginate", "pagination offset cannot be negative")
	}
	return nil
}
