package server

import (
	"context"
	"fmt"

	"github.com/fluxcd/kustomize-controller/api/v1beta2"
	"github.com/fluxcd/source-controller/api/v1beta1"
	"github.com/weaveworks/weave-gitops/core/gitops/app"
	"github.com/weaveworks/weave-gitops/core/gitops/kustomize"
	"github.com/weaveworks/weave-gitops/core/gitops/types"
	"github.com/weaveworks/weave-gitops/core/repository"
	"github.com/weaveworks/weave-gitops/core/source"
	pb "github.com/weaveworks/weave-gitops/pkg/api/app"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func protoToKustomization(kustomization *pb.AddKustomizationRequest) v1beta2.Kustomization {
	return v1beta2.Kustomization{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1beta2.KustomizationKind,
			APIVersion: v1beta2.GroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      kustomization.Name,
			Namespace: kustomization.Namespace,
		},
		Spec: v1beta2.KustomizationSpec{
			Path:     kustomization.Path,
			Interval: intervalDuration(kustomization.Interval),
			SourceRef: v1beta2.CrossNamespaceSourceReference{
				Kind: kustomization.SourceRef.Kind.String(),
				Name: kustomization.Name,
			},
		},
		Status: v1beta2.KustomizationStatus{},
	}
}

func kustomizationToProto(kustomization v1beta2.Kustomization) *pb.Kustomization {

	var kind pb.SourceRef_Kind
	switch kustomization.Spec.SourceRef.Kind {
	case v1beta1.GitRepositoryKind:
		kind = pb.SourceRef_GitRepository
	case v1beta1.HelmRepositoryKind:
		kind = pb.SourceRef_HelmRepository
	case v1beta1.BucketKind:
		kind = pb.SourceRef_Bucket
	}

	return &pb.Kustomization{
		Name:      kustomization.Name,
		Namespace: kustomization.Namespace,
		Path:      kustomization.Spec.Path,
		SourceRef: &pb.SourceRef{
			Kind: kind,
			Name: kustomization.Spec.SourceRef.Name,
		},
		Interval: nil,
	}
}

type kustServer struct {
	pb.UnimplementedAppKustomizationServer

	creator     kustomize.Creator
	fetcher     app.Fetcher
	repoManager repository.Manager
	sourceSvc   source.Service
}

func NewKustomizationServer(creator kustomize.Creator, fetcher app.Fetcher, sourceSvc source.Service, repoManager repository.Manager) pb.AppKustomizationServer {
	return &kustServer{
		creator:     creator,
		fetcher:     fetcher,
		repoManager: repoManager,
		sourceSvc:   sourceSvc,
	}
}

func (ks *kustServer) Add(ctx context.Context, msg *pb.AddKustomizationRequest) (*pb.AddKustomizationResponse, error) {
	repo, key, err := getRepo(ks.sourceSvc, ks.repoManager, msg.RepoName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "kustServer.Add: %w", err)
	}

	k, err := ks.creator.Create(ctx, repo, key, kustomize.CreateInput{
		AppName:       msg.AppName,
		RepoName:      msg.RepoName,
		Kustomization: protoToKustomization(msg),
	})
	if err == types.ErrNotFound {
		fmt.Println("hitting not found")
		return nil, status.Error(codes.NotFound, "resource does not exist")
	} else if err != nil {
		return nil, status.Errorf(codes.Internal, "unable to create kustomization: %w", err)
	}

	return &pb.AddKustomizationResponse{
		Success:       true,
		Kustomization: kustomizationToProto(k),
	}, nil
}

func (ks *kustServer) Remove(ctx context.Context, msg *pb.RemoveKustomizationRequest) (*pb.RemoveKustomizationResponse, error) {
	//repo, key, err := getRepo(ks.sourceSvc, ks.repoManager, msg.RepoName)
	//if err != nil {
	//	return nil, status.Errorf(codes.Internal, "kustServer.Remove: %w", err)
	//}
	//
	//app, err := ks.fetcher.Get(ctx, msg.AppName, msg.RepoName, types.FluxNamespace)
	//if err == types.ErrNotFound {
	//	return nil, status.Error(codes.NotFound, "resource does not exist")
	//} else if err != nil {
	//	return nil, fmt.Errorf("kustServer.Add: %w")
	//}
	//
	//return &pb.RemoveKustomizationResponse{
	//	Success: true,
	//}, nil

	return nil, nil
}
