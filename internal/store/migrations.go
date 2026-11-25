// SPDX-License-Identifier: Elastic-2.0
package store

import _ "embed"

//go:embed migrations/001_initial.sql
var initial string

//go:embed migrations/002_catalogue.sql
var catalogue string

//go:embed migrations/003_metadata.sql
var metadataSchema string

//go:embed migrations/004_releases.sql
var releaseSchema string

//go:embed migrations/005_archive_members.sql
var archiveSchema string

//go:embed migrations/006_servers.sql
var serverSchema string

//go:embed migrations/007_wanted.sql
var wantedSchema string

//go:embed migrations/008_wanted_tracking.sql
var wantedTrackingSchema string

//go:embed migrations/009_wanted_schedule.sql
var wantedScheduleSchema string

//go:embed migrations/010_wanted_scope.sql
var wantedScopeSchema string
