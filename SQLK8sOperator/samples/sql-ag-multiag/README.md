# SQL Server Multi-AG Sample

Deploys **three Availability Groups** on a single set of SQL Server replicas,
each configured for a different workload pattern.

## Scenario Overview

| AG | Database | Purpose | Replicas | Sync Mode | Failover |
|----|----------|---------|----------|-----------|----------|
| ProductionAG | AppDB | OLTP | 3 (all) | 3 synchronous | Automatic |
| ReportingAG | ReportingDB | Read scale-out | 3 (all) | 1 sync + 2 async | Manual |
| DisasterRecoveryAG | CriticalDB | Geo-DR | 2 (0,1) | 1 sync + 1 async | Manual |

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                     mssql namespace                         │
│                                                             │
│  ┌───────────┐    ┌───────────┐    ┌───────────┐           │
│  │ sql-ag-0  │    │ sql-ag-1  │    │ sql-ag-2  │           │
│  │ (PRIMARY) │    │(SECONDARY)│    │(SECONDARY)│           │
│  ├───────────┤    ├───────────┤    ├───────────┤           │
│  │ AppDB     │◄──►│ AppDB     │◄──►│ AppDB     │  Prod AG │
│  │ RepDB     │◄──►│ RepDB     │◄──►│ RepDB     │  Rep  AG │
│  │ CritDB    │◄──►│ CritDB    │    │           │  DR   AG │
│  └───────────┘    └───────────┘    └───────────┘           │
│                                                             │
│  Services:                                                  │
│    production-ag-primary   :1433  ──► OLTP writes           │
│    production-ag-secondary :1434  ──► OLTP reads            │
│    reporting-ag-primary    :2433  ──► (internal)             │
│    reporting-ag-secondary  :2434  ──► Reporting queries      │
│    dr-ag-primary           :3433  ──► (internal)             │
│    dr-ag-secondary         :3434  ──► (internal)             │
└─────────────────────────────────────────────────────────────┘
```

## Customization

> **Passwords:** The sample manifests and scripts ship with placeholder passwords (e.g. `YourStrong@Passw0rd!`). Before deploying, open `ag-deploy.yaml` and change the SA password and AG Helper password in the Secret resources. Then update the matching values at the top of `ag-configure.sh` (`SA_PASSWORD`, `AG_HELPER_PASSWORD`, `MASTER_KEY_PASSWORD`, `REPLICA_LOGIN_PASSWORD`), or in the manual T-SQL steps in `ag-configure.md`.

> **Instance and AG names:** You can rename the SQLServer resource, pod prefix, AG names (`ProductionAG`, `ReportingAG`, `DisasterRecoveryAG`), and database names (`AppDB`, `ReportingDB`, `CriticalDB`). If you do, update them consistently in:
> - `ag-deploy.yaml` — SQLServer `.metadata.name`, each SQLServerAG `.metadata.name` and `.spec.availabilityGroup.name`, per-replica Service names and port mappings
> - `ag-configure.sh` — the `AG_NAMES`, `AG_DATABASES`, `AG_RESOURCE_NAMES` arrays and replica variables at the top
> - `ag-configure.md` — the pod names, AG names, and database names referenced in every T-SQL command

## Quick Start

```bash
# 1. Deploy infrastructure
kubectl apply -f ag-deploy.yaml

# 2. Wait for pods
kubectl -n mssql wait --for=condition=ready pod -l app=mssql --timeout=300s

# 3. Configure all 3 AGs (automated)
chmod +x ag-configure.sh
./ag-configure.sh all

# 4. Verify
./ag-configure.sh verify
```

## Manual Configuration

Follow [ag-configure.md](ag-configure.md) for step-by-step T-SQL instructions.

## Port Mapping

| Port | Service | Purpose |
|------|---------|---------|
| 1433 | production-ag-primary | OLTP writes |
| 1434 | production-ag-secondary | OLTP read replicas |
| 2433 | reporting-ag-primary | Internal (reporting primary) |
| 2434 | reporting-ag-secondary | Reporting queries (read scale-out) |
| 3433 | dr-ag-primary | Internal (DR primary) |
| 3434 | dr-ag-secondary | Internal (DR secondary) |

## Useful Commands

```bash
# Check all AG statuses
kubectl -n mssql exec sql-ag-0 -- /opt/mssql-tools18/bin/sqlcmd \
  -S localhost -U sa -P 'YourStrong@Passw0rd!' -C \
  -Q "SELECT ag.name, rs.role_desc, rs.synchronization_health_desc \
      FROM sys.dm_hadr_availability_replica_states rs \
      JOIN sys.availability_groups ag ON rs.group_id = ag.group_id \
      ORDER BY ag.name, rs.role_desc"

# Failover ProductionAG (automatic — handled by operator)
# Failover ReportingAG or DR AG (manual):
kubectl -n mssql exec sql-ag-1 -- /opt/mssql-tools18/bin/sqlcmd \
  -S localhost -U sa -P 'YourStrong@Passw0rd!' -C \
  -Q "ALTER AVAILABILITY GROUP [ReportingAG] FORCE_FAILOVER_ALLOW_DATA_LOSS"
```

## Cleanup

```bash
kubectl delete -f ag-deploy.yaml
kubectl delete namespace mssql
```
