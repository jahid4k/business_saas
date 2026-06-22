// backend/internal/platform/contacts/repository.go
package contacts

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mridha/businesssaas/pkg/pagination"
)

// Repository defines the data access interface for contacts and companies.
//
// TENANT ISOLATION: every method takes orgID.
// Every query filters by org_id. A record from another org is treated as
// not found — callers cannot distinguish "wrong tenant" from "not exists".
//
// Tx variants accept a pgx.Tx so callers can compose multi-step operations
// into a single database transaction (e.g. lead conversion).
type Repository interface {
	// Contacts
	FindContacts(ctx context.Context, orgID string, p pagination.Params) ([]*Contact, error)
	FindContactByID(ctx context.Context, orgID, contactID string) (*Contact, error)
	FindContactsByCompany(ctx context.Context, orgID, companyID string) ([]*Contact, error)
	CreateContact(ctx context.Context, c *Contact) error
	// CreateContactTx inserts a contact inside an existing transaction.
	// Used by lead conversion to keep contact + deal + lead-status atomic.
	CreateContactTx(ctx context.Context, tx pgx.Tx, c *Contact) error
	UpdateContact(ctx context.Context, c *Contact) error
	SoftDeleteContact(ctx context.Context, orgID, contactID string) error
	CountContacts(ctx context.Context, orgID string) (int, error)

	// Companies
	FindCompanies(ctx context.Context, orgID string, p pagination.Params) ([]*Company, error)
	FindCompanyByID(ctx context.Context, orgID, companyID string) (*Company, error)
	CreateCompany(ctx context.Context, c *Company) error
	UpdateCompany(ctx context.Context, c *Company) error
	SoftDeleteCompany(ctx context.Context, orgID, companyID string) error
	CountCompanies(ctx context.Context, orgID string) (int, error)
}

type repoImpl struct {
	db *pgxpool.Pool
}

// NewRepository creates a new contacts repository.
func NewRepository(db *pgxpool.Pool) Repository {
	return &repoImpl{db: db}
}

// ============================================================
// Contacts
// ============================================================

const contactCols = `
	id, public_id, org_id, first_name, last_name, email, phone, title,
	company_id, source, status, owner_id, created_by, created_at, updated_at`

func scanContact(row interface{ Scan(...any) error }, c *Contact) error {
	return row.Scan(
		&c.ID, &c.PublicID, &c.OrgID, &c.FirstName, &c.LastName,
		&c.Email, &c.Phone, &c.Title, &c.CompanyID, &c.Source,
		&c.Status, &c.OwnerID, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
	)
}

func (r *repoImpl) FindContacts(ctx context.Context, orgID string, p pagination.Params) ([]*Contact, error) {
	q := `SELECT ` + contactCols + `
		FROM platform_contacts
		WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, q, orgID, p.Limit, p.Offset)
	if err != nil {
		return nil, fmt.Errorf("contacts: FindContacts: %w", err)
	}
	defer rows.Close()

	var out []*Contact
	for rows.Next() {
		c := &Contact{}
		if err := scanContact(rows, c); err != nil {
			return nil, fmt.Errorf("contacts: FindContacts: scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindContactByID(ctx context.Context, orgID, contactID string) (*Contact, error) {
	q := `SELECT ` + contactCols + `
		FROM platform_contacts
		WHERE org_id = $1 AND id = $2 AND deleted_at IS NULL`

	c := &Contact{}
	err := scanContact(r.db.QueryRow(ctx, q, orgID, contactID), c)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("contacts: FindContactByID: %w", err)
	}
	return c, nil
}

func (r *repoImpl) FindContactsByCompany(ctx context.Context, orgID, companyID string) ([]*Contact, error) {
	q := `SELECT ` + contactCols + `
		FROM platform_contacts
		WHERE org_id = $1 AND company_id = $2 AND deleted_at IS NULL
		ORDER BY created_at DESC`

	rows, err := r.db.Query(ctx, q, orgID, companyID)
	if err != nil {
		return nil, fmt.Errorf("contacts: FindContactsByCompany: %w", err)
	}
	defer rows.Close()

	var out []*Contact
	for rows.Next() {
		c := &Contact{}
		if err := scanContact(rows, c); err != nil {
			return nil, fmt.Errorf("contacts: FindContactsByCompany: scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

const insertContactSQL = `
	INSERT INTO platform_contacts
	    (org_id, first_name, last_name, email, phone, title,
	     company_id, source, status, owner_id, created_by)
	VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
	RETURNING id, public_id, created_at, updated_at`

func (r *repoImpl) CreateContact(ctx context.Context, c *Contact) error {
	return r.db.QueryRow(ctx, insertContactSQL,
		c.OrgID, c.FirstName, c.LastName, c.Email, c.Phone, c.Title,
		c.CompanyID, c.Source, c.Status, c.OwnerID, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.CreatedAt, &c.UpdatedAt)
}

// CreateContactTx inserts a contact within an existing pgx.Tx.
// The interface accepts pgx.Tx, not pgxpool.Pool, so callers own the transaction.
func (r *repoImpl) CreateContactTx(ctx context.Context, tx pgx.Tx, c *Contact) error {
	return tx.QueryRow(ctx, insertContactSQL,
		c.OrgID, c.FirstName, c.LastName, c.Email, c.Phone, c.Title,
		c.CompanyID, c.Source, c.Status, c.OwnerID, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *repoImpl) UpdateContact(ctx context.Context, c *Contact) error {
	const q = `
		UPDATE platform_contacts
		SET first_name = $1, last_name = $2, email = $3, phone = $4, title = $5,
		    company_id = $6, source = $7, status = $8, owner_id = $9, updated_at = NOW()
		WHERE org_id = $10 AND id = $11 AND deleted_at IS NULL
		RETURNING updated_at`

	err := r.db.QueryRow(ctx, q,
		c.FirstName, c.LastName, c.Email, c.Phone, c.Title,
		c.CompanyID, c.Source, c.Status, c.OwnerID,
		c.OrgID, c.ID,
	).Scan(&c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrContactNotFound
	}
	return err
}

func (r *repoImpl) SoftDeleteContact(ctx context.Context, orgID, contactID string) error {
	cmd, err := r.db.Exec(ctx,
		`UPDATE platform_contacts SET deleted_at = NOW() WHERE org_id = $1 AND id = $2 AND deleted_at IS NULL`,
		orgID, contactID,
	)
	if err != nil {
		return fmt.Errorf("contacts: SoftDeleteContact: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrContactNotFound
	}
	return nil
}

func (r *repoImpl) CountContacts(ctx context.Context, orgID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM platform_contacts WHERE org_id = $1 AND deleted_at IS NULL`,
		orgID,
	).Scan(&n)
	return n, err
}

// ============================================================
// Companies
// ============================================================

const companyCols = `
	id, public_id, org_id, name, domain, industry, website, phone,
	address, country, status, owner_id, created_by, created_at, updated_at`

func scanCompany(row interface{ Scan(...any) error }, c *Company) error {
	return row.Scan(
		&c.ID, &c.PublicID, &c.OrgID, &c.Name, &c.Domain, &c.Industry,
		&c.Website, &c.Phone, &c.Address, &c.Country, &c.Status,
		&c.OwnerID, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
	)
}

func (r *repoImpl) FindCompanies(ctx context.Context, orgID string, p pagination.Params) ([]*Company, error) {
	q := `SELECT ` + companyCols + `
		FROM platform_companies
		WHERE org_id = $1 AND deleted_at IS NULL
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`

	rows, err := r.db.Query(ctx, q, orgID, p.Limit, p.Offset)
	if err != nil {
		return nil, fmt.Errorf("contacts: FindCompanies: %w", err)
	}
	defer rows.Close()

	var out []*Company
	for rows.Next() {
		c := &Company{}
		if err := scanCompany(rows, c); err != nil {
			return nil, fmt.Errorf("contacts: FindCompanies: scan: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (r *repoImpl) FindCompanyByID(ctx context.Context, orgID, companyID string) (*Company, error) {
	q := `SELECT ` + companyCols + `
		FROM platform_companies
		WHERE org_id = $1 AND id = $2 AND deleted_at IS NULL`

	c := &Company{}
	err := scanCompany(r.db.QueryRow(ctx, q, orgID, companyID), c)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("contacts: FindCompanyByID: %w", err)
	}
	return c, nil
}

func (r *repoImpl) CreateCompany(ctx context.Context, c *Company) error {
	const q = `
		INSERT INTO platform_companies
		    (org_id, name, domain, industry, website, phone, address, country, status, owner_id, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, public_id, created_at, updated_at`

	return r.db.QueryRow(ctx, q,
		c.OrgID, c.Name, c.Domain, c.Industry, c.Website,
		c.Phone, c.Address, c.Country, c.Status, c.OwnerID, c.CreatedBy,
	).Scan(&c.ID, &c.PublicID, &c.CreatedAt, &c.UpdatedAt)
}

func (r *repoImpl) UpdateCompany(ctx context.Context, c *Company) error {
	const q = `
		UPDATE platform_companies
		SET name = $1, domain = $2, industry = $3, website = $4, phone = $5,
		    address = $6, country = $7, status = $8, owner_id = $9, updated_at = NOW()
		WHERE org_id = $10 AND id = $11 AND deleted_at IS NULL
		RETURNING updated_at`

	err := r.db.QueryRow(ctx, q,
		c.Name, c.Domain, c.Industry, c.Website, c.Phone,
		c.Address, c.Country, c.Status, c.OwnerID,
		c.OrgID, c.ID,
	).Scan(&c.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrCompanyNotFound
	}
	return err
}

func (r *repoImpl) SoftDeleteCompany(ctx context.Context, orgID, companyID string) error {
	cmd, err := r.db.Exec(ctx,
		`UPDATE platform_companies SET deleted_at = NOW() WHERE org_id = $1 AND id = $2 AND deleted_at IS NULL`,
		orgID, companyID,
	)
	if err != nil {
		return fmt.Errorf("contacts: SoftDeleteCompany: %w", err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrCompanyNotFound
	}
	return nil
}

func (r *repoImpl) CountCompanies(ctx context.Context, orgID string) (int, error) {
	var n int
	err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM platform_companies WHERE org_id = $1 AND deleted_at IS NULL`,
		orgID,
	).Scan(&n)
	return n, err
}
