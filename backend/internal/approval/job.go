package approval

import (
	"cloud.google.com/go/civil"
	"github.com/shopspring/decimal"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
)

type Job struct {
	title     string
	company   string
	city      string
	country   *posting.Country
	remote    bool
	salary    posting.Salary
	annualUSD decimal.Decimal
	lang      posting.Language
	postedOn  *civil.Date
}

func (j *Job) Title() string              { return j.title }
func (j *Job) Company() string            { return j.company }
func (j *Job) City() string               { return j.city }
func (j *Job) Country() *posting.Country  { return j.country }
func (j *Job) Remote() bool               { return j.remote }
func (j *Job) Salary() posting.Salary     { return j.salary }
func (j *Job) AnnualUSD() decimal.Decimal { return j.annualUSD }
func (j *Job) Lang() posting.Language     { return j.lang }
func (j *Job) PostedOn() *civil.Date      { return j.postedOn }
