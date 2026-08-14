// Command bootstrap-admin creates the first administrator of an empty
// database. It is the only supported path into a freshly reset environment:
// public registration is disabled and creating a user requires an already
// authenticated admin, so without this command a new database can never be
// logged into.
//
// Credentials come exclusively from environment variables and are never
// embedded in the repository. The command refuses to run when any user
// already exists, which makes it safe to re-run and prevents it from being
// used to escalate privileges on a bootstrapped system.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/lease-management-system/core-service/internal/config"
	"github.com/lease-management-system/core-service/internal/db"
	"github.com/lease-management-system/core-service/internal/repository"
)

// Exit codes are part of the command's contract; they are documented in
// README §4.
const (
	exitOK                  = 0
	exitMissingEnv          = 2
	exitAlreadyBootstrapped = 3
	exitDatabaseError       = 4
)

func main() {
	os.Exit(run())
}

func run() int {
	username := os.Getenv("BOOTSTRAP_ADMIN_USERNAME")
	email := os.Getenv("BOOTSTRAP_ADMIN_EMAIL")
	password := os.Getenv("BOOTSTRAP_ADMIN_PASSWORD")
	if username == "" || email == "" || password == "" {
		fmt.Fprintln(os.Stderr, "bootstrap-admin: BOOTSTRAP_ADMIN_USERNAME, BOOTSTRAP_ADMIN_EMAIL and BOOTSTRAP_ADMIN_PASSWORD are all required")
		return exitMissingEnv
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap-admin: load config: %v\n", err)
		return exitDatabaseError
	}
	database, err := db.New(cfg.DatabaseURL())
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap-admin: connect to database: %v\n", err)
		return exitDatabaseError
	}
	defer database.Close()

	ctx := context.Background()

	// Refuse when any user exists: the system is already bootstrapped. This
	// also keeps the command idempotent and unusable as a privilege-escalation
	// tool against a live database.
	var anyUser bool
	if err := database.Pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users)`).Scan(&anyUser); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap-admin: check existing users: %v\n", err)
		return exitDatabaseError
	}
	if anyUser {
		fmt.Fprintln(os.Stderr, "bootstrap-admin: system already bootstrapped; refusing to create another user")
		return exitAlreadyBootstrapped
	}

	// Reuse the repository's Create so password hashing stays the single
	// bcrypt path used by the whole application (repository/user.go).
	userRepo := repository.NewUserRepository(database.Pool)
	user, err := userRepo.Create(ctx, username, email, password, "admin", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap-admin: create admin user: %v\n", err)
		return exitDatabaseError
	}

	// Resolve the admin role by code rather than hard-coding its id, and
	// leave assigned_by NULL: the first admin has no assigner.
	if _, err := database.Pool.Exec(ctx,
		`INSERT INTO user_roles (user_id, role_id, assigned_by)
		 VALUES ($1, (SELECT id FROM roles WHERE code = 'admin'), NULL)`,
		user.ID,
	); err != nil {
		fmt.Fprintf(os.Stderr, "bootstrap-admin: assign admin role: %v\n", err)
		return exitDatabaseError
	}

	fmt.Printf("bootstrap-admin: created first admin %q (id=%s)\n", user.Username, user.ID)
	return exitOK
}
