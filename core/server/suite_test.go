package server_test

import (
	"context"
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/go-logr/logr"
	"github.com/weaveworks/weave-gitops/core/clustersmngr"
	"github.com/weaveworks/weave-gitops/core/clustersmngr/cluster"
	"github.com/weaveworks/weave-gitops/core/clustersmngr/cluster/clusterfakes"
	"github.com/weaveworks/weave-gitops/core/clustersmngr/fetcher"
	"github.com/weaveworks/weave-gitops/core/nsaccess/nsaccessfakes"
	"github.com/weaveworks/weave-gitops/core/server"
	pb "github.com/weaveworks/weave-gitops/pkg/api/core"
	"github.com/weaveworks/weave-gitops/pkg/kube"
	"github.com/weaveworks/weave-gitops/pkg/server/auth"
	"github.com/weaveworks/weave-gitops/pkg/services/crd"
	"github.com/weaveworks/weave-gitops/pkg/testutils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/test/bufconn"
	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
	typedauth "k8s.io/client-go/kubernetes/typed/authorization/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var k8sEnv *testutils.K8sTestEnv
var nsChecker nsaccessfakes.FakeChecker

func TestMain(m *testing.M) {
	var err error
	k8sEnv, err = testutils.StartK8sTestEnvironment([]string{
		"../../manifests/crds",
		"../../tools/testcrds",
	})

	if err != nil {
		panic(err)
	}

	code := m.Run()

	k8sEnv.Stop()

	os.Exit(code)
}

func makeGRPCServer(cfg *rest.Config, t *testing.T) pb.CoreClient {
	log := logr.Discard()
	nsChecker = nsaccessfakes.FakeChecker{}
	nsChecker.FilterAccessibleNamespacesStub = func(ctx context.Context, t typedauth.AuthorizationV1Interface, n []v1.Namespace) ([]v1.Namespace, error) {
		// Pretend the user has access to everything
		return n, nil
	}

	scheme, err := kube.CreateScheme()
	if err != nil {
		t.Fatal(err)
	}

	cluster, err := cluster.NewSingleCluster("Default", k8sEnv.Rest, scheme)
	if err != nil {
		t.Fatal(err)
	}

	fetch := fetcher.NewSingleClusterFetcher(cluster)

	clustersManager := clustersmngr.NewClustersManager([]clustersmngr.ClusterFetcher{fetch}, &nsChecker, log)
	coreCfg, err := server.NewCoreConfig(log, cfg, "foobar", clustersManager)
	if err != nil {
		t.Fatal(err)
	}

	coreCfg.NSAccess = &nsChecker
	coreCfg.CRDService = crd.NewNoCacheFetcher(clustersManager)

	core, err := server.NewCoreServer(coreCfg)
	if err != nil {
		t.Fatal(err)
	}

	lis := bufconn.Listen(1024 * 1024)

	s := grpc.NewServer(
		withClientsPoolInterceptor(clustersManager),
	)

	pb.RegisterCoreServer(s, core)

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	go func(tt *testing.T) {
		if err := s.Serve(lis); err != nil {
			tt.Error(err)
		}
	}(t)

	conn, err := grpc.DialContext(
		context.Background(),
		"bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		s.GracefulStop()
		conn.Close()
	})

	return pb.NewCoreClient(conn)
}

type userKey struct{}

var UserKey = userKey{}

const (
	MetadataUserKey   string = "test_principal_user"
	MetadataGroupsKey string = "test_principal_groups"
)

func withClientsPoolInterceptor(clustersManager clustersmngr.ClustersManager) grpc.ServerOption {
	return grpc.UnaryInterceptor(func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if err := clustersManager.UpdateClusters(ctx); err != nil {
			return nil, err
		}
		if err := clustersManager.UpdateNamespaces(ctx); err != nil {
			return nil, err
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, fmt.Errorf("getting metadata from context failed")
		}

		var user string
		if len(md[MetadataUserKey]) > 0 {
			user = md[MetadataUserKey][0]
		}
		groups := md[MetadataGroupsKey]
		principal := auth.UserPrincipal{ID: user, Groups: groups}
		clustersManager.UpdateUserNamespaces(ctx, &principal)

		ctx = auth.WithPrincipal(ctx, &principal)

		return handler(ctx, req)
	})
}

func makeServerConfig(fakeClient client.Client, t *testing.T, clusterName string) server.CoreServerConfig {
	log := logr.Discard()
	nsChecker = nsaccessfakes.FakeChecker{}
	nsChecker.FilterAccessibleNamespacesStub = func(ctx context.Context, t typedauth.AuthorizationV1Interface, n []v1.Namespace) ([]v1.Namespace, error) {
		// Pretend the user has access to everything
		return n, nil
	}
	clientset := fake.NewSimpleClientset()

	cluster := clusterfakes.FakeCluster{}

	if clusterName == "" {
		clusterName = "Default"
	}

	cluster.GetNameReturns(clusterName)
	cluster.GetUserClientReturns(fakeClient, nil)
	cluster.GetUserClientsetReturns(clientset, nil)
	cluster.GetServerClientReturns(fakeClient, nil)

	fetcher := fetcher.NewSingleClusterFetcher(&cluster)

	// Don't include the clustersmngr.DefaultKubeConfigOptions here as we're using a fake kubeclient
	// and the default options include the Flowcontrol setup which is not mocked out
	clustersManager := clustersmngr.NewClustersManager([]clustersmngr.ClusterFetcher{fetcher}, &nsChecker, log)

	coreCfg, err := server.NewCoreConfig(log, &rest.Config{}, "foobar", clustersManager)
	if err != nil {
		t.Fatal(err)
	}

	coreCfg.NSAccess = &nsChecker

	return coreCfg
}

func makeServer(cfg server.CoreServerConfig, t *testing.T) pb.CoreClient {
	core, err := server.NewCoreServer(cfg)
	if err != nil {
		t.Fatal(err)
	}

	lis := bufconn.Listen(1024 * 1024)

	s := grpc.NewServer(
		withClientsPoolInterceptor(cfg.ClustersManager),
	)

	pb.RegisterCoreServer(s, core)

	dialer := func(context.Context, string) (net.Conn, error) {
		return lis.Dial()
	}

	go func(tt *testing.T) {
		if err := s.Serve(lis); err != nil {
			tt.Error(err)
		}
	}(t)

	conn, err := grpc.DialContext(
		context.Background(),
		"bufnet",
		grpc.WithContextDialer(dialer),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		s.GracefulStop()
		conn.Close()
	})

	return pb.NewCoreClient(conn)
}
