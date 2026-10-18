package httpapi

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/lefi8/job-ingestion-and-search/backend/internal/postgres"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/posting"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/ptr"
	"github.com/lefi8/job-ingestion-and-search/backend/internal/search"
)

const (
	defaultPage               = 1
	defaultPerPage            = 20
	statusClientClosedRequest = 499
)

type JobSearcher interface {
	Search(ctx context.Context, q search.Query) (search.Page, error)
}

type JobReader interface {
	ApprovedJob(ctx context.Context, id uuid.UUID) (postgres.Job, error)
}

type handler struct {
	searcher JobSearcher
	jobs     JobReader
	timeout  time.Duration
}

func (h handler) GetJobs(c *gin.Context, params GetJobsParams) {
	query := search.Query{
		Q:         ptr.ValueOr(params.Q, ""),
		Countries: ptr.ValueOr(params.Country, nil),
		Sort:      search.Sort(ptr.ValueOr(params.Sort, "")),
		Page:      ptr.ValueOr(params.Page, defaultPage),
		PerPage:   ptr.ValueOr(params.PerPage, defaultPerPage),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()
	page, err := h.searcher.Search(ctx, query)
	if err != nil {
		if c.Request.Context().Err() != nil {
			c.AbortWithStatus(statusClientClosedRequest)
			return
		}
		_ = c.Error(err)
		writeProblem(c, http.StatusServiceUnavailable, "Search is temporarily unavailable. Try again later.")
		return
	}

	items := make([]JobCard, 0, len(page.Items))
	for _, hit := range page.Items {
		items = append(items, toJobCard(hit))
	}
	facets := make([]Facet, 0, len(page.Facets))
	for _, f := range page.Facets {
		facets = append(facets, Facet{Value: f.Value, Count: f.Count})
	}
	c.JSON(http.StatusOK, JobsPage{
		Items:   items,
		Total:   page.Total,
		Page:    query.Page,
		PerPage: query.PerPage,
		Facets:  Facets{Country: facets},
	})
}

func (h handler) GetJob(c *gin.Context, id uuid.UUID) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.timeout)
	defer cancel()
	job, err := h.jobs.ApprovedJob(ctx, id)
	if errors.Is(err, postgres.ErrNotFound) {
		writeProblem(c, http.StatusNotFound, "Job not found.")
		return
	}
	if err != nil {
		if c.Request.Context().Err() != nil {
			c.AbortWithStatus(statusClientClosedRequest)
			return
		}
		_ = c.Error(err)
		writeProblem(c, http.StatusServiceUnavailable, "Job is temporarily unavailable. Try again later.")
		return
	}
	detail, err := toJobDetail(job)
	if err != nil {
		_ = c.Error(err)
		writeProblem(c, http.StatusInternalServerError, "")
		return
	}
	c.JSON(http.StatusOK, detail)
}

func toJobDetail(job postgres.Job) (JobDetail, error) {
	display, err := posting.SalaryDisplay(job.SalaryAmount, job.SalaryCurrency, posting.PayUnit(job.SalaryUnit))
	if err != nil {
		return JobDetail{}, fmt.Errorf("posting %s: %w", job.PostingID, err)
	}
	companyType := JobDetailCompanyType(job.CompanyType)
	if !companyType.Valid() {
		return JobDetail{}, fmt.Errorf("posting %s: invalid company type %q", job.PostingID, job.CompanyType)
	}
	detail := JobDetail{
		Id:               job.PostingID,
		Title:            job.Title,
		Company:          job.Company,
		City:             ptr.NilIfZero(job.City),
		Region:           ptr.NilIfZero(job.Region),
		Country:          ptr.NilIfZero(job.Country),
		Remote:           job.Remote,
		SalaryDisplay:    display,
		SalaryUsdAnnual:  job.SalaryUSDAnnual,
		CurrencyInferred: job.CurrencyInferred,
		Description:      job.Description,
		CompanyType:      companyType,
	}
	if job.PostedOn != nil {
		detail.PostedOn = ptr.NilIfZero(job.PostedOn.String())
	}
	return detail, nil
}

func toJobCard(hit search.Hit) JobCard {
	card := JobCard{
		Id:               hit.JobID,
		Title:            hit.Title,
		Company:          hit.Company,
		City:             ptr.NilIfZero(hit.City),
		Country:          ptr.NilIfZero(hit.Country),
		Remote:           hit.Remote,
		SalaryDisplay:    hit.SalaryDisplay,
		SalaryUsdAnnual:  hit.SalaryUSDAnnual,
		CurrencyInferred: hit.CurrencyInferred,
		PostedOn:         ptr.NilIfZero(hit.PostedOn),
	}
	if len(hit.Highlight) > 0 {
		segments := make([]HighlightSegment, len(hit.Highlight))
		for i, s := range hit.Highlight {
			segments[i] = HighlightSegment{Text: s.Text, Match: s.Match}
		}
		card.TitleHighlight = &segments
	}
	return card
}
