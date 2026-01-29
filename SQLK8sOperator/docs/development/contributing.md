# Contributing

[← Back to Development](../README.md) | [Documentation Home](../README.md)

Guidelines for contributing to the SQL Server Kubernetes Operator.

## Table of Contents

- [Getting Started](#getting-started)
- [Development Process](#development-process)
- [Code Standards](#code-standards)
- [Pull Request Process](#pull-request-process)
- [Issue Guidelines](#issue-guidelines)
- [Community](#community)

## Getting Started

### Prerequisites

1. Read the [Architecture Documentation](../architecture/overview.md)
2. Set up [Local Development](local-development.md) environment
3. Understand the [Building](building.md) process
4. Review the [Testing](testing.md) guide

### Fork and Clone

```bash
# Fork the repository on GitHub

# Clone your fork
git clone https://github.com/YOUR_USERNAME/sql-server-k8s-operator.git
cd sql-server-k8s-operator

# Add upstream remote
git remote add upstream https://github.com/yourorg/sql-server-k8s-operator.git
```

### Create Feature Branch

```bash
# Sync with upstream
git fetch upstream
git checkout main
git merge upstream/main

# Create feature branch
git checkout -b feature/my-feature
```

## Development Process

### Workflow

```
┌─────────────────────────────────────────────────────────────────────┐
│                     Contribution Workflow                            │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  1. Issue                                                            │
│     └─ Create or find an issue                                      │
│     └─ Discuss approach in comments                                 │
│                                                                      │
│  2. Branch                                                           │
│     └─ Create feature/fix branch from main                          │
│                                                                      │
│  3. Develop                                                          │
│     └─ Write code following standards                               │
│     └─ Add/update tests                                             │
│     └─ Update documentation                                         │
│                                                                      │
│  4. Test                                                             │
│     └─ Run make test                                                │
│     └─ Run make lint                                                │
│     └─ Manual testing                                               │
│                                                                      │
│  5. Pull Request                                                     │
│     └─ Create PR with description                                   │
│     └─ Link to issue                                                │
│     └─ Request review                                               │
│                                                                      │
│  6. Review                                                           │
│     └─ Address feedback                                             │
│     └─ Update as needed                                             │
│                                                                      │
│  7. Merge                                                            │
│     └─ Squash and merge                                             │
│     └─ Delete branch                                                │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### Types of Contributions

| Type | Branch Prefix | Example |
|------|---------------|---------|
| Feature | `feature/` | `feature/multi-ag-support` |
| Bug fix | `fix/` | `fix/statefulset-update` |
| Documentation | `docs/` | `docs/ag-deployment` |
| Refactor | `refactor/` | `refactor/controller-structure` |
| Chore | `chore/` | `chore/update-dependencies` |

## Code Standards

### Go Code Style

Follow standard Go conventions:

```go
// Good: Clear, idiomatic Go
func (r *SQLServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    log := log.FromContext(ctx)
    
    var sqlServer mssqlv1alpha1.SQLServer
    if err := r.Get(ctx, req.NamespacedName, &sqlServer); err != nil {
        if apierrors.IsNotFound(err) {
            return ctrl.Result{}, nil
        }
        return ctrl.Result{}, err
    }
    
    // Reconcile logic here
    return ctrl.Result{}, nil
}

// Bad: Unclear, non-idiomatic
func (r *SQLServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    var s mssqlv1alpha1.SQLServer
    e := r.Get(ctx, req.NamespacedName, &s)
    if e != nil {
        if apierrors.IsNotFound(e) { return ctrl.Result{}, nil }
        return ctrl.Result{}, e
    }
    return ctrl.Result{}, nil
}
```

### Naming Conventions

| Element | Convention | Example |
|---------|------------|---------|
| Package | lowercase | `controller` |
| Interface | PascalCase + -er | `Reconciler` |
| Struct | PascalCase | `SQLServerReconciler` |
| Function | PascalCase (exported) | `BuildStatefulSet` |
| Function | camelCase (private) | `validateSpec` |
| Constants | SCREAMING_SNAKE or PascalCase | `MaxRetries` |

### Comments and Documentation

```go
// SQLServerReconciler reconciles a SQLServer object.
// It manages the lifecycle of SQL Server instances in Kubernetes,
// including StatefulSet, Services, and ConfigMaps.
type SQLServerReconciler struct {
    client.Client
    Scheme *runtime.Scheme
    // Recorder is used to record events for the custom resource.
    Recorder record.EventRecorder
}

// Reconcile handles the reconciliation loop for SQLServer resources.
// It ensures the actual state matches the desired state defined in the spec.
//
// Returns:
//   - ctrl.Result{}: Successful reconciliation
//   - ctrl.Result{RequeueAfter: duration}: Requeue after delay
//   - error: Failed reconciliation, will be requeued
func (r *SQLServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // Implementation
}
```

### Error Handling

```go
// Good: Wrap errors with context
if err := r.Create(ctx, statefulSet); err != nil {
    return ctrl.Result{}, fmt.Errorf("failed to create StatefulSet: %w", err)
}

// Good: Use sentinel errors
var ErrInvalidEdition = errors.New("invalid SQL Server edition")

func validateEdition(edition string) error {
    validEditions := []string{"Developer", "Express", "Standard", "Enterprise"}
    for _, valid := range validEditions {
        if edition == valid {
            return nil
        }
    }
    return fmt.Errorf("%w: %s", ErrInvalidEdition, edition)
}
```

### Testing Requirements

Every PR should include:

- [ ] Unit tests for new functions
- [ ] Integration tests for controller changes
- [ ] Updated existing tests if behavior changes
- [ ] Minimum 70% coverage for new code

```go
func TestNewFunction(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        {"success case", "input", "output", false},
        {"error case", "bad", "", true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := NewFunction(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("NewFunction() error = %v, wantErr %v", err, tt.wantErr)
            }
            if got != tt.want {
                t.Errorf("NewFunction() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

### Linting

```bash
# Run linter
make lint

# Fix auto-fixable issues
golangci-lint run --fix

# Lint configuration in .golangci.yml
```

## Pull Request Process

### Before Submitting

- [ ] Tests pass: `make test`
- [ ] Linter passes: `make lint`
- [ ] Code formatted: `make fmt`
- [ ] Manifests generated: `make manifests`
- [ ] Documentation updated

### PR Title Format

Use conventional commits format:

```
<type>(<scope>): <description>

Examples:
feat(controller): add multi-AG support
fix(validation): correct memory validation regex
docs(user-guide): add troubleshooting section
chore(deps): update controller-runtime to v0.17.0
```

### PR Description Template

```markdown
## Description
Brief description of changes.

## Type of Change
- [ ] Bug fix (non-breaking change fixing an issue)
- [ ] New feature (non-breaking change adding functionality)
- [ ] Breaking change (fix or feature causing existing functionality to change)
- [ ] Documentation update

## Related Issue
Fixes #123

## How Has This Been Tested?
- [ ] Unit tests
- [ ] Integration tests
- [ ] Manual testing

## Checklist
- [ ] My code follows the project's code standards
- [ ] I have added tests covering my changes
- [ ] All new and existing tests pass
- [ ] I have updated the documentation accordingly
- [ ] My changes generate no new warnings
```

### Review Process

1. **Automated checks** - CI runs tests and linting
2. **Code review** - At least 1 maintainer approval
3. **Documentation review** - For user-facing changes
4. **Testing** - Reviewer may test locally

### Addressing Feedback

```bash
# Make changes based on review
git add .
git commit -m "address review feedback"
git push origin feature/my-feature

# Or amend if single commit
git add .
git commit --amend
git push --force origin feature/my-feature
```

## Issue Guidelines

### Bug Reports

Include:
- SQL Server Operator version
- Kubernetes version
- SQL Server version
- Steps to reproduce
- Expected vs actual behavior
- Logs and error messages

### Feature Requests

Include:
- Use case description
- Proposed solution
- Alternative solutions considered
- Impact on existing functionality

### Issue Labels

| Label | Description |
|-------|-------------|
| `bug` | Something isn't working |
| `enhancement` | New feature request |
| `documentation` | Documentation improvement |
| `good first issue` | Good for newcomers |
| `help wanted` | Extra attention needed |
| `priority/high` | High priority |
| `area/controller` | Controller-related |
| `area/api` | API/CRD-related |

## Community

### Code of Conduct

We follow the [Contributor Covenant](https://www.contributor-covenant.org/) code of conduct.

### Communication

| Channel | Purpose |
|---------|---------|
| GitHub Issues | Bug reports, feature requests |
| GitHub Discussions | Questions, ideas, announcements |
| Slack | Real-time discussion |

### Recognition

Contributors are recognized in:
- CONTRIBUTORS.md file
- Release notes
- Project documentation

## Quick Reference

```bash
# Full development cycle
git checkout -b feature/my-feature
# ... make changes ...
make fmt
make lint
make test
git add .
git commit -m "feat(scope): description"
git push origin feature/my-feature
# Create PR on GitHub
```

## Next Steps

- [Local Development](local-development.md) - Environment setup
- [Building](building.md) - Build process
- [Testing](testing.md) - Testing guide
