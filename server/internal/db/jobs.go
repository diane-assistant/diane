package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateJob creates a new job in the database
func (db *DB) CreateJob(name, command, schedule string) (*Job, error) {
	result, err := db.conn.Exec(
		`INSERT INTO jobs (name, command, schedule, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)`,
		name, command, schedule, time.Now(), time.Now(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create job: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return db.GetJob(id)
}

// GetJob retrieves a job by ID
func (db *DB) GetJob(id int64) (*Job, error) {
	job := &Job{}
	err := db.conn.QueryRow(
		`SELECT id, name, command, schedule, enabled, created_at, updated_at
		 FROM jobs WHERE id = ?`,
		id,
	).Scan(&job.ID, &job.Name, &job.Command, &job.Schedule, &job.Enabled, &job.CreatedAt, &job.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	return job, nil
}

// GetJobByName retrieves a job by name
func (db *DB) GetJobByName(name string) (*Job, error) {
	job := &Job{}
	err := db.conn.QueryRow(
		`SELECT id, name, command, schedule, enabled, created_at, updated_at
		 FROM jobs WHERE name = ?`,
		name,
	).Scan(&job.ID, &job.Name, &job.Command, &job.Schedule, &job.Enabled, &job.CreatedAt, &job.UpdatedAt)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("job not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	return job, nil
}

// ListJobs retrieves all jobs, optionally filtered by enabled status
func (db *DB) ListJobs(enabledOnly bool) ([]*Job, error) {
	query := `SELECT id, name, command, schedule, enabled, created_at, updated_at FROM jobs`
	if enabledOnly {
		query += ` WHERE enabled = 1`
	}
	query += ` ORDER BY name`

	rows, err := db.conn.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		job := &Job{}
		if err := rows.Scan(&job.ID, &job.Name, &job.Command, &job.Schedule, &job.Enabled, &job.CreatedAt, &job.UpdatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		jobs = append(jobs, job)
	}

	return jobs, rows.Err()
}

// UpdateJob updates a job's properties
func (db *DB) UpdateJob(id int64, command, schedule *string, enabled *bool) error {
	job, err := db.GetJob(id)
	if err != nil {
		return err
	}

	if command != nil {
		job.Command = *command
	}
	if schedule != nil {
		job.Schedule = *schedule
	}
	if enabled != nil {
		job.Enabled = *enabled
	}

	_, err = db.conn.Exec(
		`UPDATE jobs SET command = ?, schedule = ?, enabled = ?, updated_at = ?
		 WHERE id = ?`,
		job.Command, job.Schedule, job.Enabled, time.Now(), id,
	)
	if err != nil {
		return fmt.Errorf("failed to update job: %w", err)
	}

	return nil
}

// DeleteJob deletes a job by ID
func (db *DB) DeleteJob(id int64) error {
	result, err := db.conn.Exec(`DELETE FROM jobs WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("failed to delete job: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("job not found")
	}

	return nil
}
