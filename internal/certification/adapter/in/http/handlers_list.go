package http

import (
	"net/http"
	"strconv"

	"github.com/afwilto/certifications-mngr/internal/certification/app/port/in"
	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

func handleListCertifications(list in.ListCertifications) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query, err := decodeListQuery(r)
		if err != nil {
			translateError(w, err)
			return
		}
		page, err := list.Execute(r.Context(), query)
		if err != nil {
			translateError(w, err)
			return
		}
		items := make([]certificationDTO, 0, len(page.Items))
		for _, cert := range page.Items {
			items = append(items, toCertificationDTO(cert))
		}
		totalPages := (page.TotalItems + query.PageSize - 1) / query.PageSize
		writeList(w, http.StatusOK, items, listMetaDTO{
			Page:       query.Page,
			PageSize:   query.PageSize,
			TotalItems: page.TotalItems,
			TotalPages: totalPages,
		})
	}
}

func decodeListQuery(r *http.Request) (domain.ListQuery, error) {
	params := r.URL.Query()
	query := domain.ListQuery{Search: params.Get("search")}

	if raw := params.Get("page"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return query, domain.SingleViolation("page", "min", "page must be an integer of at least 1")
		}
		query.Page = n
	}
	if raw := params.Get("pageSize"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > domain.MaxPageSize {
			return query, domain.SingleViolation("pageSize", "range", "pageSize must be an integer between 1 and 100")
		}
		query.PageSize = n
	}
	for _, raw := range params["status"] {
		status, err := domain.ParseStatus(raw)
		if err != nil {
			return query, err
		}
		query.Statuses = append(query.Statuses, status)
	}
	if raw := params.Get("category"); raw != "" {
		path, err := domain.NewCategoryPath(raw)
		if err != nil {
			return query, err
		}
		query.CategoryPrefix = path.Normalized()
	}
	if raw := params.Get("year"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return query, domain.SingleViolation("year", "integer", "year must be an integer")
		}
		query.Year = n
	}
	if raw := params.Get("sort"); raw != "" {
		sort, err := domain.ParseSortField(raw)
		if err != nil {
			return query, err
		}
		query.Sort = sort
	}
	if raw := params.Get("order"); raw != "" {
		order, err := domain.ParseSortOrder(raw)
		if err != nil {
			return query, err
		}
		query.Order = order
	}

	query = query.WithDefaults()
	if err := query.Validate(); err != nil {
		return query, err
	}
	return query, nil
}

func handleListCategories(list in.ListCategories) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		categories, err := list.Execute(r.Context())
		if err != nil {
			translateError(w, err)
			return
		}
		items := make([]categoryDTO, 0, len(categories))
		for _, c := range categories {
			count := c.Count
			dto := toCategoryDTO(c.Category)
			dto.Count = &count
			items = append(items, dto)
		}
		writeData(w, http.StatusOK, items)
	}
}
