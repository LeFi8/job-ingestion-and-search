package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

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

type handler struct {
	searcher      JobSearcher
	searchTimeout time.Duration
}

func (h handler) GetJobs(c *gin.Context, params GetJobsParams) {
	query := search.Query{
		Q:         ptr.ValueOr(params.Q, ""),
		Countries: ptr.ValueOr(params.Country, nil),
		Sort:      search.Sort(ptr.ValueOr(params.Sort, "")),
		Page:      ptr.ValueOr(params.Page, defaultPage),
		PerPage:   ptr.ValueOr(params.PerPage, defaultPerPage),
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), h.searchTimeout)
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
