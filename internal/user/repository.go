package user

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pixperk/goiler/db/sqlc"
)

// Repository defines the interface for user data access
type Repository interface {
	Create(ctx context.Context, user *User) error
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, email string) (*User, error)
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, limit, offset int) ([]*User, int64, error)
}

// PostgresRepository implements Repository using PostgreSQL
type PostgresRepository struct {
	db      *pgxpool.Pool
	queries *sqlc.Queries
}

// NewPostgresRepository creates a new PostgreSQL repository
func NewPostgresRepository(db *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{
		db:      db,
		queries: sqlc.New(db),
	}
}

// Create creates a new user
func (r *PostgresRepository) Create(ctx context.Context, user *User) error {
	return r.queries.CreateUser(ctx, sqlc.CreateUserParams{
		ID:           user.ID,
		Email:        user.Email,
		Name:         stringToPgText(user.Name),
		PasswordHash: user.PasswordHash,
		Role:         user.Role,
	})
}

// GetByID retrieves a user by ID
func (r *PostgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	dbUser, err := r.queries.GetUserByID(ctx, id)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &User{
		ID:           dbUser.ID,
		Email:        dbUser.Email,
		Name:         pgTextToString(dbUser.Name),
		PasswordHash: dbUser.PasswordHash,
		Role:         dbUser.Role,
		CreatedAt:    dbUser.CreatedAt.Time,
		UpdatedAt:    dbUser.UpdatedAt.Time,
	}, nil
}

// GetByEmail retrieves a user by email
func (r *PostgresRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	dbUser, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return &User{
		ID:           dbUser.ID,
		Email:        dbUser.Email,
		Name:         pgTextToString(dbUser.Name),
		PasswordHash: dbUser.PasswordHash,
		Role:         dbUser.Role,
		CreatedAt:    dbUser.CreatedAt.Time,
		UpdatedAt:    dbUser.UpdatedAt.Time,
	}, nil
}

// Update updates a user
func (r *PostgresRepository) Update(ctx context.Context, user *User) error {
	return r.queries.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:           user.ID,
		Email:        user.Email,
		Name:         stringToPgText(user.Name),
		PasswordHash: user.PasswordHash,
	})
}

// Delete deletes a user
func (r *PostgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.queries.DeleteUser(ctx, id)
}

// List returns a paginated list of users
func (r *PostgresRepository) List(ctx context.Context, limit, offset int) ([]*User, int64, error) {
	dbUsers, err := r.queries.ListUsers(ctx, sqlc.ListUsersParams{
		Limit:  int32(limit),
		Offset: int32(offset),
	})
	if err != nil {
		return nil, 0, err
	}

	count, err := r.queries.CountUsers(ctx)
	if err != nil {
		return nil, 0, err
	}

	users := make([]*User, len(dbUsers))
	for i, dbUser := range dbUsers {
		users[i] = &User{
			ID:           dbUser.ID,
			Email:        dbUser.Email,
			Name:         pgTextToString(dbUser.Name),
			PasswordHash: dbUser.PasswordHash,
			Role:         dbUser.Role,
			CreatedAt:    dbUser.CreatedAt.Time,
			UpdatedAt:    dbUser.UpdatedAt.Time,
		}
	}

	return users, count, nil
}

// Helper functions for null string handling
func stringToPgText(s string) pgtype.Text {
	if s == "" {
		return pgtype.Text{Valid: false}
	}
	return pgtype.Text{String: s, Valid: true}
}

func pgTextToString(t pgtype.Text) string {
	if !t.Valid {
		return ""
	}
	return t.String
}
