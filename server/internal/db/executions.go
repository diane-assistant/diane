package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateJobExecution creates a new job execution log entry
func (db *DB) CreateJobExecution(jobID int64) (int64, error) {
	result, err := db.conn.Exec(
		`INSERT INTO job_executions (job_id, started_at) VALUES (?, ?)`,
		jobID, time.Now(),
	)
	if err != nil {
		return 0, fmt.Errorf("failed to create job execution: %w", err)
	}

	return result.LastInsertId()
}

// UpdateJobExecution updates a job execution with results
func (db *DB) UpdateJobExecution(id int64, exitCode int, stdout, stderr string, execErr error) error {
	now := time.Now()
	var errStr *string
	if execErr != nil {
		s := execErr.Error()
		errStr = &s
	}

	_, err := db.conn.Exec(
		`UPDATE job_executions
		 SET ended_at = ?, exit_code = ?, stdout = ?, stderr = ?, error = ?
		 WHERE id = ?`,
		now, exitCode, stdout, stderr, errStr, id,
	)
	if err != nil {
		return fmt.Errorf("failed to update job execution: %w", err)
	}

	return nil
}

// GetJobExecution retrieves a single job execution by ID
func (db *DB) GetJobExecution(id int64) (*JobExecution, error) {
	exec := &JobExecution{}
	var errStr sql.NullString

	err := db.conn.QueryRow(
		`SELECT id, job_id, started_at, ended_at, exit_code, stdout, stderr, error
		 FROM job_executions WHERE id = ?`,
		id,
	).Scan(&exec.ID, &exec.JobID, &exec.StartedAt, &exec.EndedAt, &exec.ExitCode, &exec.Stdout, &exec.Stderr, &errStr)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job execution not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get job execution: %w", err)
	}

	if errStr.Valid {
		exec.Error = &errStr.String
	}

	return exec, nil
}

// ListJobExecutions retrieves job executions with optional filters
func (db *DB) ListJobExecutions(jobID *int64, limit, offset int) ([]*JobExecution, error) {
	query := `SELECT id, job_id, started_at, ended_at, exit_code, stdout, stderr, error
	          FROM job_executions`

	args := []interface{}{}
	if jobID != nil {
		query += ` WHERE job_id = ?`
		args = append(args, *jobID)
	}

	query += ` ORDER BY started_at DESC`

	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
		if offset > 0 {
			query += ` OFFSET ?`
			args = append(args, offset)
		}
	}

	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list job executions: %w", err)
	}
	defer rows.Close()

	var executions []*JobExecution
	for rows.Next() {
		exec := &JobExecution{}
		var errStr sql.NullString

		if err := rows.Scan(&exec.ID, &exec.JobID, &exec.StartedAt, &exec.EndedAt, &exec.ExitCode, &exec.Stdout, &exec.Stderr, &errStr); err != nil {
			return nil, fmt.Errorf("failed to scan job execution: %w", err)
		}

		if errStr.Valid {
			exec.Error = &errStr.String
		}

		executions = append(executions, exec)
	}

	return executions, rows.Err()
}

// DeleteOldExecutions deletes job executions older than the specified duration
func (db *DB) DeleteOldExecutions(retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	result, err := db.conn.Exec(
		`DELETE FROM job_executions WHERE started_at < ?`,
		cutoff,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to delete old executions: %w", err)
	}

	return result.RowsAffected()
}
