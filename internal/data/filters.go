package data

import (
	"math"
	"strings"

	"placeholder_project_tag/pkg/validator"
)

type Filters struct {
	Page         int
	PageSize     int
	Sort         string
	SortSafeList []string
	Metadata     FilterMetadata
}

type FilterMetadata struct {
	CurrentPage   int    `json:"current_page,omitempty"`
	PageSize      int    `json:"page_size,omitempty"`
	FirstPage     int    `json:"first_page,omitempty"`
	LastPage      int    `json:"last_page,omitempty"`
	Sort          string `json:"sort,omitempty"`
	SortDirection string `json:"sort_direction,omitempty"`
	TotalRecords  int    `json:"total_records,omitempty"`
}

type LogFilters struct {
	Filters
	LogsMetadata LogsMetadata
	Level        []string
	Message      string
	UserID       []int
}

type LogsMetadata struct {
	FilterMetadata
	Levels map[string]int `json:"levels"`
	Users  map[int]int    `json:"users"`
}

// Initialise a LogsMetadata struct with maps ready to be written to
func NewLogsMetadata() LogsMetadata {
	return LogsMetadata{
		Levels: make(map[string]int),
		Users:  make(map[int]int),
	}
}

type UserFilters struct {
	Filters
	Email    string
	UserIDs  []int64
	UserName string
}

// If f.Sort matches something in the SortSafeList, return it after removing the hyphen prefix if it exists. Otherwise, throw a panic, because it means there's potential for SQL injection. It should not however be possible to trigger this panic in the first place, as the validator should already have returned a user error if the sort query doesn't match something in the safe list - this is just a fail-safe.
func (f *Filters) sortColumn() string {
	for _, safeValue := range f.SortSafeList {
		if f.Sort == safeValue {
			return strings.TrimPrefix(f.Sort, "-")
		}
	}

	panic("unsafe sort parameter: " + f.Sort)
}

func (f *Filters) sortDirection() string {
	if strings.HasPrefix(f.Sort, "-") {
		return "DESC"
	}

	return "ASC"
}

func (f *Filters) limit() int {
	return f.PageSize
}

// OFFSET skips the number of rows provided in the OFFSET query, so to calculate the correct offset, use the formula below. The validation of page and page size prevent this integer from ever being too large.
func (f *Filters) offset() int {
	return (f.Page - 1) * f.PageSize
}

func calculateMetadata(totalRecords, page, pageSize int) FilterMetadata {
	if totalRecords == 0 {
		return FilterMetadata{}
	}

	// The result of division to find last page could be a decimal, so round it up and convert to an int to get an appropriate last page value
	return FilterMetadata{
		CurrentPage:  page,
		PageSize:     pageSize,
		FirstPage:    1,
		LastPage:     int(math.Ceil(float64(totalRecords) / float64(pageSize))),
		TotalRecords: totalRecords,
	}
}

func ValidateFilters(v *validator.Validator, f Filters) {
	v.Check(f.Page > 0, "page", "must be greater than zero")
	v.Check(f.Page <= 10_000_000, "page", "must be a maximum of 10 million")
	v.Check(f.PageSize > 0, "page_size", "must be greater than zero")
	v.Check(f.PageSize <= 100, "page_size", "must be a maximum of 100")

	v.Check(validator.In(f.Sort, f.SortSafeList...), "sort", "invalid sort value")
}
