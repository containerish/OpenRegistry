package v2

import (
	"errors"
	"fmt"
)

var (
	ErrDBWrite  = errors.New("ERR_DB_WRITE")
	ErrDBRead   = errors.New("ERR_DB_READ")
	ErrDBUpdate = errors.New("ERR_DB_UPDATE")
	ErrDBDelete = errors.New("ERR_DB_DELETE")
)

type DatabaseError struct {
	Message string                `json:"message"`
	Cause   DatabaseOperationType `json:"cause"`
}

func (e *DatabaseError) Error() string {
	return fmt.Sprintf("Cause=%v Message=%s", e.Cause, e.Message)
}

type DatabaseOperationType = int8

const (
	DatabaseOperationWrite  DatabaseOperationType = 1
	DatabaseOperationRead   DatabaseOperationType = 2
	DatabaseOperationDelete DatabaseOperationType = 3
	DatabaseOperationUpdate DatabaseOperationType = 4
)

func WrapDatabaseError(baseErr error, opType DatabaseOperationType) error {
	return &DatabaseError{Cause: opType, Message: baseErr.Error()}
}
