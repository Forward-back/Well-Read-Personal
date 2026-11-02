package repository

import "gorm.io/gorm"

// bumpTenantACLEpoch is a no-op in the personal edition: there is no
// per-tenant ACL cache to invalidate. The call sites remain because member and
// folder mutations still run through transactions that used to bump it.
func bumpTenantACLEpoch(_ *gorm.DB, _ uint64) error { return nil }
