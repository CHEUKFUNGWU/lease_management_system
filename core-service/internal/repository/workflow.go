package repository

import (
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

// ErrInvalidWorkflowTransition is returned when a workflow update did not
// match the expected current state. Conditional updates keep concurrent or
// repeated requests from skipping approval stages.
var ErrInvalidWorkflowTransition = errors.New("invalid workflow transition")

func requireWorkflowTransition(result pgconn.CommandTag, err error, subject, id string) error {
	if err != nil {
		return err
	}
	if result.RowsAffected() != 1 {
		return fmt.Errorf("%w: %s %s is not in the required state", ErrInvalidWorkflowTransition, subject, id)
	}
	return nil
}
