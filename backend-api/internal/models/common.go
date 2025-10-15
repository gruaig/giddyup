package models

import (
	"database/sql"
	"fmt"
)

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Total   int         `json:"total"`
	Page    int         `json:"page"`
	PerPage int         `json:"per_page"`
	Data    interface{} `json:"data"`
}

// StatsSplit represents performance statistics for a category
type StatsSplit struct {
	Category string   `json:"category" db:"category"`
	Runs     int      `json:"runs" db:"runs"`
	Wins     int      `json:"wins" db:"wins"`
	Places   int      `json:"places,omitempty" db:"places"`
	SR       float64  `json:"sr" db:"sr"`
	ROI      *float64 `json:"roi,omitempty" db:"roi"`
	AvgRPR   *float64 `json:"avg_rpr,omitempty" db:"avg_rpr"`
	AvgOR    *float64 `json:"avg_or,omitempty" db:"avg_or"`
}

// TrendPoint represents a data point in a time series
type TrendPoint struct {
	Date    string  `json:"date" db:"date"`
	RPR     *int    `json:"rpr,omitempty" db:"rpr"`
	OR      *int    `json:"or,omitempty" db:"or"`
	Class   *string `json:"class,omitempty" db:"class"`
	WinFlag bool    `json:"win_flag,omitempty" db:"win_flag"`
}

// NullFloat64 represents a nullable float64
type NullFloat64 struct {
	sql.NullFloat64
}

func (nf NullFloat64) MarshalJSON() ([]byte, error) {
	if !nf.Valid {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("%.2f", nf.Float64)), nil
}

// NullInt64 represents a nullable int64
type NullInt64 struct {
	sql.NullInt64
}

func (ni NullInt64) MarshalJSON() ([]byte, error) {
	if !ni.Valid {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("%d", ni.Int64)), nil
}

// NullString represents a nullable string
type NullString struct {
	sql.NullString
}

func (ns NullString) MarshalJSON() ([]byte, error) {
	if !ns.Valid {
		return []byte("null"), nil
	}
	return []byte(fmt.Sprintf("\"%s\"", ns.String)), nil
}
