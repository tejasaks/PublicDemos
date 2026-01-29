# Testing

[← Back to Development](../README.md) | [Documentation Home](../README.md)

Guide to testing the SQL Server Kubernetes Operator.

## Table of Contents

- [Testing Overview](#testing-overview)
- [Unit Tests](#unit-tests)
- [Integration Tests](#integration-tests)
- [End-to-End Tests](#end-to-end-tests)
- [Test Coverage](#test-coverage)
- [Test Fixtures](#test-fixtures)
- [Mocking](#mocking)

## Testing Overview

### Test Pyramid

```
        ┌───────────┐
        │   E2E     │  ← Slow, high confidence
        │  Tests    │
        ├───────────┤
        │Integration│  ← Medium speed
        │  Tests    │
        ├───────────┤
        │   Unit    │  ← Fast, focused
        │  Tests    │
        └───────────┘
```

### Test Locations

| Type | Location | Framework |
|------|----------|-----------|
| Unit | `*_test.go` alongside code | Go testing |
| Integration | `internal/controller/*_test.go` | envtest |
| E2E | `test/e2e/` | Ginkgo + Kind |

### Running Tests

```bash
# All tests
make test

# Unit tests only
go test ./... -short

# Integration tests
make test-integration

# E2E tests
make test-e2e
```

## Unit Tests

### Controller Unit Tests

```go
// internal/controller/sqlserver_controller_test.go
package controller

import (
    "context"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    
    mssqlv1alpha1 "github.com/yourorg/mssql-operator/api/v1alpha1"
)

func TestSQLServerReconciler_validateSpec(t *testing.T) {
    tests := []struct {
        name    string
        spec    mssqlv1alpha1.SQLServerSpec
        wantErr bool
    }{
        {
            name: "valid spec",
            spec: mssqlv1alpha1.SQLServerSpec{
                Version:  "2022",
                Edition:  "Developer",
                Instance: mssqlv1alpha1.InstanceSpec{
                    Replicas: 1,
                },
            },
            wantErr: false,
        },
        {
            name: "invalid version",
            spec: mssqlv1alpha1.SQLServerSpec{
                Version: "2000",
                Edition: "Developer",
            },
            wantErr: true,
        },
        {
            name: "invalid edition",
            spec: mssqlv1alpha1.SQLServerSpec{
                Version: "2022",
                Edition: "Invalid",
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            r := &SQLServerReconciler{}
            err := r.validateSpec(&tt.spec)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func TestSQLServerReconciler_buildStatefulSet(t *testing.T) {
    sqlServer := &mssqlv1alpha1.SQLServer{
        ObjectMeta: metav1.ObjectMeta{
            Name:      "test-sql",
            Namespace: "default",
        },
        Spec: mssqlv1alpha1.SQLServerSpec{
            Version:  "2022",
            Edition:  "Developer",
            Instance: mssqlv1alpha1.InstanceSpec{
                Replicas: 3,
            },
        },
    }
    
    r := &SQLServerReconciler{}
    sts := r.buildStatefulSet(sqlServer)
    
    require.NotNil(t, sts)
    assert.Equal(t, "test-sql", sts.Name)
    assert.Equal(t, int32(3), *sts.Spec.Replicas)
    assert.Equal(t, "test-sql", sts.Spec.ServiceName)
}
```

### Validation Tests

```go
// api/v1alpha1/sqlserver_validation_test.go
package v1alpha1

import (
    "testing"
    
    "github.com/stretchr/testify/assert"
)

func TestValidateResourceName(t *testing.T) {
    tests := []struct {
        name     string
        input    string
        wantErr  bool
        errField string
    }{
        {"valid short name", "sql-prod", false, ""},
        {"valid 13 chars", "sqlprod123456", false, ""},
        {"too long", "sql-prod-server-long-name", true, "name"},
        {"uppercase", "SQL-Prod", true, "name"},
        {"underscore", "sql_prod", true, "name"},
        {"starts with dash", "-sql-prod", true, "name"},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateResourceName(tt.input)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func TestValidateMemory(t *testing.T) {
    tests := []struct {
        name    string
        memory  string
        wantErr bool
    }{
        {"2Gi valid", "2Gi", false},
        {"4Gi valid", "4Gi", false},
        {"1Gi invalid", "1Gi", true},
        {"500Mi invalid", "500Mi", true},
        {"2048Mi valid", "2048Mi", false},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateMemory(tt.memory)
            if tt.wantErr {
                assert.Error(t, err)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}
```

### Running Unit Tests

```bash
# All unit tests
go test ./... -short -v

# Specific package
go test ./internal/controller -v

# Specific test
go test ./internal/controller -run TestSQLServerReconciler_validateSpec -v

# With race detection
go test ./... -race
```

## Integration Tests

Integration tests use envtest to run a real API server.

### Setup envtest

```go
// internal/controller/suite_test.go
package controller

import (
    "context"
    "path/filepath"
    "testing"
    
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    
    "k8s.io/client-go/kubernetes/scheme"
    "k8s.io/client-go/rest"
    ctrl "sigs.k8s.io/controller-runtime"
    "sigs.k8s.io/controller-runtime/pkg/client"
    "sigs.k8s.io/controller-runtime/pkg/envtest"
    logf "sigs.k8s.io/controller-runtime/pkg/log"
    "sigs.k8s.io/controller-runtime/pkg/log/zap"
    
    mssqlv1alpha1 "github.com/yourorg/mssql-operator/api/v1alpha1"
)

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var ctx context.Context
var cancel context.CancelFunc

func TestControllers(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
    logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))
    
    ctx, cancel = context.WithCancel(context.TODO())
    
    By("bootstrapping test environment")
    testEnv = &envtest.Environment{
        CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
        ErrorIfCRDPathMissing: true,
    }
    
    var err error
    cfg, err = testEnv.Start()
    Expect(err).NotTo(HaveOccurred())
    Expect(cfg).NotTo(BeNil())
    
    err = mssqlv1alpha1.AddToScheme(scheme.Scheme)
    Expect(err).NotTo(HaveOccurred())
    
    k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
    Expect(err).NotTo(HaveOccurred())
    Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
    cancel()
    By("tearing down the test environment")
    err := testEnv.Stop()
    Expect(err).NotTo(HaveOccurred())
})
```

### Integration Test Example

```go
// internal/controller/sqlserver_controller_integration_test.go
package controller

import (
    "time"
    
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/types"
    
    mssqlv1alpha1 "github.com/yourorg/mssql-operator/api/v1alpha1"
)

var _ = Describe("SQLServer Controller", func() {
    const (
        timeout  = time.Second * 30
        interval = time.Millisecond * 250
    )
    
    Context("When creating a SQLServer", func() {
        It("Should create the StatefulSet and Service", func() {
            By("Creating a new SQLServer")
            sqlServer := &mssqlv1alpha1.SQLServer{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      "test-sql",
                    Namespace: "default",
                },
                Spec: mssqlv1alpha1.SQLServerSpec{
                    Version:  "2022",
                    Edition:  "Developer",
                    Instance: mssqlv1alpha1.InstanceSpec{
                        Replicas: 1,
                    },
                    Credentials: mssqlv1alpha1.CredentialsSpec{
                        SAPasswordSecretRef: mssqlv1alpha1.SecretRef{
                            Name: "test-sql-sa",
                            Key:  "password",
                        },
                    },
                },
            }
            
            // Create secret first
            secret := &corev1.Secret{
                ObjectMeta: metav1.ObjectMeta{
                    Name:      "test-sql-sa",
                    Namespace: "default",
                },
                StringData: map[string]string{
                    "password": "TestP@ssw0rd!",
                },
            }
            Expect(k8sClient.Create(ctx, secret)).To(Succeed())
            
            Expect(k8sClient.Create(ctx, sqlServer)).To(Succeed())
            
            By("Checking the StatefulSet is created")
            Eventually(func() bool {
                var sts appsv1.StatefulSet
                err := k8sClient.Get(ctx, types.NamespacedName{
                    Name:      "test-sql",
                    Namespace: "default",
                }, &sts)
                return err == nil
            }, timeout, interval).Should(BeTrue())
            
            By("Checking the Service is created")
            Eventually(func() bool {
                var svc corev1.Service
                err := k8sClient.Get(ctx, types.NamespacedName{
                    Name:      "test-sql",
                    Namespace: "default",
                }, &svc)
                return err == nil
            }, timeout, interval).Should(BeTrue())
        })
    })
})
```

### Running Integration Tests

```bash
# Run integration tests
make test-integration

# Or directly
KUBEBUILDER_ASSETS="$(go env GOPATH)/src/sigs.k8s.io/controller-runtime/tools/bin/envtest" \
  go test ./internal/controller -v -ginkgo.v
```

## End-to-End Tests

E2E tests run against a real Kubernetes cluster.

### E2E Test Setup

```go
// test/e2e/e2e_suite_test.go
package e2e

import (
    "os"
    "testing"
    
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    
    "k8s.io/client-go/kubernetes"
    "k8s.io/client-go/tools/clientcmd"
)

var clientset *kubernetes.Clientset

func TestE2E(t *testing.T) {
    RegisterFailHandler(Fail)
    RunSpecs(t, "E2E Suite")
}

var _ = BeforeSuite(func() {
    kubeconfig := os.Getenv("KUBECONFIG")
    if kubeconfig == "" {
        kubeconfig = os.Getenv("HOME") + "/.kube/config"
    }
    
    config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
    Expect(err).NotTo(HaveOccurred())
    
    clientset, err = kubernetes.NewForConfig(config)
    Expect(err).NotTo(HaveOccurred())
})
```

### E2E Test Example

```go
// test/e2e/sqlserver_test.go
package e2e

import (
    "time"
    
    . "github.com/onsi/ginkgo/v2"
    . "github.com/onsi/gomega"
    
    corev1 "k8s.io/api/core/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("SQLServer E2E", func() {
    const namespace = "e2e-test"
    
    BeforeEach(func() {
        // Create test namespace
        ns := &corev1.Namespace{
            ObjectMeta: metav1.ObjectMeta{
                Name: namespace,
            },
        }
        _, _ = clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
    })
    
    AfterEach(func() {
        // Cleanup
        _ = clientset.CoreV1().Namespaces().Delete(ctx, namespace, metav1.DeleteOptions{})
    })
    
    It("Should deploy a standalone SQL Server", func() {
        By("Applying the SQLServer manifest")
        // Apply manifest using kubectl or client
        
        By("Waiting for pod to be ready")
        Eventually(func() bool {
            pods, err := clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
                LabelSelector: "app=mssql",
            })
            if err != nil || len(pods.Items) == 0 {
                return false
            }
            for _, pod := range pods.Items {
                if pod.Status.Phase != corev1.PodRunning {
                    return false
                }
            }
            return true
        }, 5*time.Minute, 10*time.Second).Should(BeTrue())
        
        By("Connecting to SQL Server")
        // Test database connectivity
    })
})
```

### Running E2E Tests

```bash
# Create Kind cluster and run E2E
make e2e

# Or step by step
kind create cluster --name e2e
make deploy IMG=mssql-operator:e2e
go test ./test/e2e -v -ginkgo.v
kind delete cluster --name e2e
```

## Test Coverage

### Generate Coverage Report

```bash
# Run tests with coverage
go test ./... -coverprofile=coverage.out

# View coverage report
go tool cover -html=coverage.out -o coverage.html
open coverage.html

# Coverage summary
go tool cover -func=coverage.out
```

### Coverage Targets

| Package | Target |
|---------|--------|
| api/v1alpha1 | 80%+ |
| internal/controller | 70%+ |
| internal/validation | 90%+ |
| Overall | 75%+ |

## Test Fixtures

### Fixtures Directory

```
test/
├── fixtures/
│   ├── sqlserver-basic.yaml
│   ├── sqlserver-ha.yaml
│   ├── sqlserverag.yaml
│   └── secrets/
│       └── sa-password.yaml
└── e2e/
```

### Loading Fixtures

```go
func loadFixture(name string) ([]byte, error) {
    return os.ReadFile(filepath.Join("test", "fixtures", name))
}

func TestWithFixture(t *testing.T) {
    data, err := loadFixture("sqlserver-basic.yaml")
    require.NoError(t, err)
    
    var sqlServer mssqlv1alpha1.SQLServer
    err = yaml.Unmarshal(data, &sqlServer)
    require.NoError(t, err)
    
    // Use fixture in test
}
```

## Mocking

### Mock Client

```go
// internal/controller/mock_client_test.go
package controller

import (
    "context"
    
    "sigs.k8s.io/controller-runtime/pkg/client"
)

type MockClient struct {
    client.Client
    GetFunc    func(ctx context.Context, key client.ObjectKey, obj client.Object) error
    CreateFunc func(ctx context.Context, obj client.Object, opts ...client.CreateOption) error
    UpdateFunc func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error
}

func (m *MockClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
    if m.GetFunc != nil {
        return m.GetFunc(ctx, key, obj)
    }
    return nil
}
```

### Using testify/mock

```go
import "github.com/stretchr/testify/mock"

type MockSQLClient struct {
    mock.Mock
}

func (m *MockSQLClient) Execute(query string) error {
    args := m.Called(query)
    return args.Error(0)
}

func TestReconcilerWithMock(t *testing.T) {
    mockClient := new(MockSQLClient)
    mockClient.On("Execute", mock.Anything).Return(nil)
    
    // Use mock in test
    err := mockClient.Execute("SELECT 1")
    assert.NoError(t, err)
    
    mockClient.AssertExpectations(t)
}
```

## Next Steps

- [Contributing](contributing.md) - Contribution guidelines
- [Local Development](local-development.md) - Development setup
- [Building](building.md) - Build process
