package handlers

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/lease-management-system/core-service/internal/errcontract"
)

// writeCodedError is the HTTP adapter of the shared error contract
// (errcontract). It encodes a coded error as {"code","error","details"} and
// keeps the legacy HTTP status code the caller chooses — this batch adds
// codes without changing status semantics. `details` carries structured
// context (reason, evidence, limits); the message is always client-safe.
func writeCodedError(c *gin.Context, status int, code errcontract.Code, message string, details map[string]any) {
	body := gin.H{"code": code, "error": message}
	if len(details) > 0 {
		body["details"] = details
	}
	c.JSON(status, body)
}

// writeSystemFailure is the sanitized fallback for an unclassified error.
// The real error stays in server logs via Gin's error chain; the client gets
// a generic message so SQL fragments and internal paths never leak.
func writeSystemFailure(c *gin.Context, status int, err error) {
	if err != nil {
		_ = c.Error(err)
	}
	writeCodedError(c, status, errcontract.CodeSystemFailure, errcontract.SafeMessage(err), nil)
}

// writeCodedFailure classifies an arbitrary error against the contract and
// emits it with the chosen status. Contract errors (scope_denied and friends)
// pass through with their code and client-safe message; anything else is an
// unknown internal failure and is sanitized by writeSystemFailure.
func writeCodedFailure(c *gin.Context, status int, err error, details map[string]any) {
	var contractErr *errcontract.Error
	if errors.As(err, &contractErr) {
		writeCodedError(c, status, contractErr.Code, contractErr.Message, details)
		return
	}
	writeSystemFailure(c, status, err)
}
