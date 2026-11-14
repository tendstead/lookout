// SPDX-License-Identifier: Elastic-2.0
package store

import _ "embed"

//go:embed migrations/001_initial.sql
var initial string

//go:embed migrations/002_catalogue.sql
var catalogue string

//go:embed migrations/003_metadata.sql
var metadataSchema string
