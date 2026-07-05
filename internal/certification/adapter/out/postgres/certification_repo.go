package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/afwilto/certifications-mngr/internal/certification/domain"
)

const uniqueViolation = "23505"

type CertificationRepository struct {
	pool *pgxpool.Pool
}

func NewCertificationRepository(pool *pgxpool.Pool) *CertificationRepository {
	return &CertificationRepository{pool: pool}
}

const certColumns = `c.id, c.name, c.goal, c.status, c.study_start_date, c.expected_exam_date,
	c.achieved_date, c.credly_url, c.official_url, c.created_at, c.updated_at, c.deleted_at,
	k.id, k.path, k.normalized_path`

func (r *CertificationRepository) Save(ctx context.Context, cert *domain.Certification) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	_, err = tx.Exec(ctx, `
		INSERT INTO certifications
			(id, category_id, name, goal, status, study_start_date, expected_exam_date,
			 achieved_date, credly_url, official_url, created_at, updated_at, deleted_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (id) DO UPDATE SET
			category_id = EXCLUDED.category_id,
			name = EXCLUDED.name,
			goal = EXCLUDED.goal,
			status = EXCLUDED.status,
			study_start_date = EXCLUDED.study_start_date,
			expected_exam_date = EXCLUDED.expected_exam_date,
			achieved_date = EXCLUDED.achieved_date,
			credly_url = EXCLUDED.credly_url,
			official_url = EXCLUDED.official_url,
			updated_at = EXCLUDED.updated_at,
			deleted_at = EXCLUDED.deleted_at`,
		cert.ID(), cert.Category().ID, cert.Name().String(), cert.Goal(), string(cert.Status()),
		cert.Period().Start(), cert.Period().Exam(),
		cert.Evidence().AchievedDate(), cert.Evidence().CredlyURL(), cert.Evidence().OfficialURL(),
		cert.CreatedAt(), cert.UpdatedAt(), cert.DeletedAt())
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == uniqueViolation {
			return domain.SingleViolation("name", "unique", "a certification with this name already exists")
		}
		return err
	}

	// The aggregate is the source of truth for attachment metadata: replace
	// the rows wholesale inside the same transaction.
	if _, err := tx.Exec(ctx, `DELETE FROM attachments WHERE certification_id = $1`, cert.ID()); err != nil {
		return err
	}
	for _, a := range cert.Attachments() {
		if _, err := tx.Exec(ctx, `
			INSERT INTO attachments (id, certification_id, kind, file_name, content_type, size_bytes, storage_key, created_at)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
			a.ID, cert.ID(), string(a.Kind), a.FileName, a.ContentType, a.SizeBytes, a.StorageKey, a.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (r *CertificationRepository) FindByID(ctx context.Context, id string) (*domain.Certification, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domain.ErrNotFound
	}
	row := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT %s FROM certifications c
		JOIN categories k ON k.id = c.category_id
		WHERE c.id = $1 AND c.deleted_at IS NULL`, certColumns), id)

	cr, err := scanCertRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	attachments, err := r.attachmentsFor(ctx, []string{cr.ID})
	if err != nil {
		return nil, err
	}
	return cr.toDomain(attachments[cr.ID])
}

func (r *CertificationRepository) FindByAttachmentID(ctx context.Context, attachmentID string) (*domain.Certification, error) {
	if _, err := uuid.Parse(attachmentID); err != nil {
		return nil, domain.ErrNotFound
	}
	row := r.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT %s FROM certifications c
		JOIN categories k ON k.id = c.category_id
		JOIN attachments a ON a.certification_id = c.id
		WHERE a.id = $1 AND c.deleted_at IS NULL`, certColumns), attachmentID)

	cr, err := scanCertRow(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrNotFound
		}
		return nil, err
	}
	attachments, err := r.attachmentsFor(ctx, []string{cr.ID})
	if err != nil {
		return nil, err
	}
	return cr.toDomain(attachments[cr.ID])
}

func (r *CertificationRepository) ExistsByName(ctx context.Context, name string, excludeID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM certifications
			WHERE lower(name) = lower($1) AND deleted_at IS NULL AND id::text <> $2)`,
		name, excludeID).Scan(&exists)
	return exists, err
}

var sortColumns = map[domain.SortField]string{
	domain.SortName:             "c.name",
	domain.SortStudyStartDate:   "c.study_start_date",
	domain.SortExpectedExamDate: "c.expected_exam_date",
	domain.SortAchievedDate:     "c.achieved_date",
	domain.SortCreatedAt:        "c.created_at",
}

func (r *CertificationRepository) FindPage(ctx context.Context, query domain.ListQuery) (domain.Page, error) {
	where := []string{"c.deleted_at IS NULL"}
	args := []any{}
	arg := func(v any) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if query.Search != "" {
		pattern := "%" + escapeLike(query.Search) + "%"
		p := arg(pattern)
		where = append(where, fmt.Sprintf("(c.name ILIKE %s OR c.goal ILIKE %s)", p, p))
	}
	if len(query.Statuses) > 0 {
		statuses := make([]string, len(query.Statuses))
		for i, s := range query.Statuses {
			statuses[i] = string(s)
		}
		where = append(where, fmt.Sprintf("c.status = ANY(%s)", arg(statuses)))
	}
	if query.CategoryPrefix != "" {
		exact := arg(query.CategoryPrefix)
		prefix := arg(escapeLike(query.CategoryPrefix) + "/%")
		where = append(where, fmt.Sprintf("(k.normalized_path = %s OR k.normalized_path LIKE %s)", exact, prefix))
	}
	if query.Year != 0 {
		yearStart := arg(fmt.Sprintf("%04d-01-01", query.Year))
		yearEnd := arg(fmt.Sprintf("%04d-12-31", query.Year))
		where = append(where, fmt.Sprintf(`(
			(c.study_start_date IS NOT NULL OR c.expected_exam_date IS NOT NULL)
			AND COALESCE(c.study_start_date, c.expected_exam_date) <= %s::date
			AND COALESCE(c.expected_exam_date, c.study_start_date) >= %s::date)`, yearEnd, yearStart))
	}

	direction := "DESC"
	if query.Order == domain.OrderAsc {
		direction = "ASC"
	}
	orderBy := fmt.Sprintf("%s %s NULLS LAST, c.created_at DESC, c.id", sortColumns[query.Sort], direction)

	limit := arg(query.PageSize)
	offset := arg((query.Page - 1) * query.PageSize)

	sql := fmt.Sprintf(`
		SELECT %s, count(*) OVER() AS total
		FROM certifications c
		JOIN categories k ON k.id = c.category_id
		WHERE %s
		ORDER BY %s
		LIMIT %s OFFSET %s`,
		certColumns, strings.Join(where, " AND "), orderBy, limit, offset)

	rows, err := r.pool.Query(ctx, sql, args...)
	if err != nil {
		return domain.Page{}, err
	}
	defer rows.Close()

	var certRows []certRow
	total := 0
	for rows.Next() {
		var cr certRow
		if err := rows.Scan(
			&cr.ID, &cr.Name, &cr.Goal, &cr.Status, &cr.StudyStartDate, &cr.ExpectedExamDate,
			&cr.AchievedDate, &cr.CredlyURL, &cr.OfficialURL, &cr.CreatedAt, &cr.UpdatedAt, &cr.DeletedAt,
			&cr.CategoryID, &cr.CategoryPath, &cr.CategoryNorm, &total); err != nil {
			return domain.Page{}, err
		}
		certRows = append(certRows, cr)
	}
	if err := rows.Err(); err != nil {
		return domain.Page{}, err
	}

	ids := make([]string, len(certRows))
	for i, cr := range certRows {
		ids[i] = cr.ID
	}
	attachments, err := r.attachmentsFor(ctx, ids)
	if err != nil {
		return domain.Page{}, err
	}

	items := make([]*domain.Certification, 0, len(certRows))
	for _, cr := range certRows {
		cert, err := cr.toDomain(attachments[cr.ID])
		if err != nil {
			return domain.Page{}, err
		}
		items = append(items, cert)
	}
	return domain.Page{Items: items, TotalItems: total}, nil
}

func (r *CertificationRepository) attachmentsFor(ctx context.Context, certIDs []string) (map[string][]attachmentRow, error) {
	out := map[string][]attachmentRow{}
	if len(certIDs) == 0 {
		return out, nil
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, certification_id, kind, file_name, content_type, size_bytes, storage_key, created_at
		FROM attachments WHERE certification_id = ANY($1) ORDER BY kind`, certIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var a attachmentRow
		if err := rows.Scan(&a.ID, &a.CertificationID, &a.Kind, &a.FileName, &a.ContentType, &a.SizeBytes, &a.StorageKey, &a.CreatedAt); err != nil {
			return nil, err
		}
		out[a.CertificationID] = append(out[a.CertificationID], a)
	}
	return out, rows.Err()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanCertRow(row rowScanner) (certRow, error) {
	var cr certRow
	err := row.Scan(
		&cr.ID, &cr.Name, &cr.Goal, &cr.Status, &cr.StudyStartDate, &cr.ExpectedExamDate,
		&cr.AchievedDate, &cr.CredlyURL, &cr.OfficialURL, &cr.CreatedAt, &cr.UpdatedAt, &cr.DeletedAt,
		&cr.CategoryID, &cr.CategoryPath, &cr.CategoryNorm)
	return cr, err
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}
