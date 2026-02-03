# Changelog

All notable changes to the SQL Server Kubernetes Operator will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- (List new features here)

### Changed
- (List changes to existing functionality)

### Deprecated
- (List features that will be removed in future versions)

### Removed
- (List features that were removed)

### Fixed
- (List bug fixes)

### Security
- (List security-related changes)

---

## [1.0.0] - 2026-02-XX

### Added
- Initial GA release of SQL Server Kubernetes Operator
- SQLServer CRD for managing SQL Server instances
- SQLServerAG CRD for managing Availability Groups
- OperatorConfiguration CRD for operator settings
- Automatic pod provisioning with StatefulSet
- HADR-enabled SQL Server deployments
- AG health monitoring via AG Helper
- Manual and automatic failover support
- AG Listener (Kubernetes Service VIP)
- Webhook validation for CRD resources
- Prometheus metrics integration
- Comprehensive documentation

### Security
- Non-root container execution
- Secret-based credential management
- SQL injection prevention in T-SQL generation
- Read-only root filesystem

---

## Version History

| Version | Release Date | Highlights |
|---------|--------------|------------|
| 1.0.0 | TBD | Initial GA release |

---

## Upgrade Notes

### Upgrading to 1.0.0

This is the initial release. No upgrade path required.

For future upgrades, see [Upgrade Guide](docs/operations/upgrades.md).
