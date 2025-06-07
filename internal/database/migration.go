package database

import (
	_ "embed"
)

//go:embed 000001_create_jobs_table.up.sql
var createJobsTableSQL string
