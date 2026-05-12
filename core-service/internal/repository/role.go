package repository

import (
	"context"
	"fmt"
	
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Role struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Permission struct {
	ID       string `json:"id"`
	RoleID   string `json:"role_id"`
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

type RoleRepository struct {
	db *pgxpool.Pool
}

func NewRoleRepository(db *pgxpool.Pool) *RoleRepository {
	return &RoleRepository{db: db}
}

func (r *RoleRepository) GetUserRoles(ctx context.Context, userID string) ([]*Role, error) {
	query := `
		SELECT r.id, r.name, r.description
		FROM roles r
		JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = $1
	`
	
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user roles: %w", err)
	}
	defer rows.Close()
	
	var roles []*Role
	for rows.Next() {
		role := &Role{}
		if err := rows.Scan(&role.ID, &role.Name, &role.Description); err != nil {
			return nil, err
		}
		roles = append(roles, role)
	}
	
	return roles, nil
}

func (r *RoleRepository) GetRolePermissions(ctx context.Context, roleID string) ([]*Permission, error) {
	query := `
		SELECT id, role_id, resource, action
		FROM permissions WHERE role_id = $1
	`
	
	rows, err := r.db.Query(ctx, query, roleID)
	if err != nil {
		return nil, fmt.Errorf("failed to get role permissions: %w", err)
	}
	defer rows.Close()
	
	var perms []*Permission
	for rows.Next() {
		p := &Permission{}
		if err := rows.Scan(&p.ID, &p.RoleID, &p.Resource, &p.Action); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	
	return perms, nil
}

func (r *RoleRepository) GetUserPermissions(ctx context.Context, userID string) ([]*Permission, error) {
	query := `
		SELECT p.id, p.role_id, p.resource, p.action
		FROM permissions p
		JOIN user_roles ur ON p.role_id = ur.role_id
		WHERE ur.user_id = $1
	`
	
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user permissions: %w", err)
	}
	defer rows.Close()
	
	var perms []*Permission
	for rows.Next() {
		p := &Permission{}
		if err := rows.Scan(&p.ID, &p.RoleID, &p.Resource, &p.Action); err != nil {
			return nil, err
		}
		perms = append(perms, p)
	}
	
	return perms, nil
}

func (r *RoleRepository) AssignRoleToUser(ctx context.Context, userID, roleID, assignedBy string) error {
	query := `
		INSERT INTO user_roles (user_id, role_id, assigned_by)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, role_id) DO NOTHING
	`
	_, err := r.db.Exec(ctx, query, userID, roleID, assignedBy)
	return err
}

type DataScope struct {
	ID         string `json:"id"`
	UserID     string `json:"user_id"`
	Dimension  string `json:"dimension"`
	TargetID   string `json:"target_id"`
	TargetName string `json:"target_name"`
}

func (r *RoleRepository) GetUserDataScopes(ctx context.Context, userID string) ([]*DataScope, error) {
	query := `
		SELECT id, user_id, dimension, target_id, target_name
		FROM user_data_scopes WHERE user_id = $1
	`
	
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get data scopes: %w", err)
	}
	defer rows.Close()
	
	var scopes []*DataScope
	for rows.Next() {
		s := &DataScope{}
		if err := rows.Scan(&s.ID, &s.UserID, &s.Dimension, &s.TargetID, &s.TargetName); err != nil {
			return nil, err
		}
		scopes = append(scopes, s)
	}
	
	return scopes, nil
}

func (r *RoleRepository) AddDataScope(ctx context.Context, userID, dimension, targetID, targetName string) error {
	query := `
		INSERT INTO user_data_scopes (user_id, dimension, target_id, target_name)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (user_id, dimension, target_id) DO NOTHING
	`
	_, err := r.db.Exec(ctx, query, userID, dimension, targetID, targetName)
	return err
}
