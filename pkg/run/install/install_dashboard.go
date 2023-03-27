package install

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	helmv2 "github.com/fluxcd/helm-controller/api/v2beta1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1beta2"
	loglevels "github.com/weaveworks/weave-gitops/core/logger"
	coretypes "github.com/weaveworks/weave-gitops/core/server/types"
	"github.com/weaveworks/weave-gitops/pkg/config"
	"github.com/weaveworks/weave-gitops/pkg/logger"
	"github.com/weaveworks/weave-gitops/pkg/run"
	"github.com/weaveworks/weave-gitops/pkg/utils"
	"golang.org/x/crypto/bcrypt"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/resource"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"
	"sigs.k8s.io/yaml"
)

type DashboardType int32

const (
	DashboardTypeNone       DashboardType = 0
	DashboardTypeOSS        DashboardType = 1
	DashboardTypeEnterprise DashboardType = 2
)

const (
	ossDashboardHelmChartName             = "weave-gitops"
	enterpriseDashboardHelmChartName      = "mccp"
	enterpriseDashboardHelmRepositoryName = "weave-gitops-enterprise-charts"
	helmRepositoryURL                     = "oci://ghcr.io/weaveworks/charts"
)

func ReadPassword(log logger.Logger) (string, error) {
	password, err := utils.ReadPasswordFromStdin(log, "Please enter a password for logging into the dashboard: ")
	if err != nil {
		log.Failuref("Could not read password")
		return "", err
	}

	return strings.TrimSpace(password), nil
}

func GeneratePasswordHash(log logger.Logger, password string) (string, error) {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Failuref("Error generating hash from password")
		return "", err
	}

	return string(passwordHash), nil
}

// CreateDashboardObjects creates HelmRepository and HelmRelease objects for the GitOps Dashboard installation.
func CreateDashboardObjects(log logger.Logger, name, namespace, username, passwordHash, chartVersion, dashboardImage string) ([]byte, error) {
	log.Actionf("Creating GitOps Dashboard objects ...")

	helmRepository := makeHelmRepository(name, namespace)
	helmRelease, err := makeHelmRelease(log, name, namespace, username, passwordHash, chartVersion, dashboardImage)

	if err != nil {
		log.Failuref("Creating HelmRelease failed")
		return nil, err
	}

	log.Generatef("Generating GitOps Dashboard manifests ...")

	manifests, err := generateManifestsForDashboard(log, helmRepository, helmRelease)
	if err != nil {
		log.Failuref("Generating GitOps Dashboard manifests failed")
		return nil, err
	}

	return manifests, nil
}

// InstallDashboard installs the GitOps Dashboard.
func InstallDashboard(ctx context.Context, log logger.Logger, manager ResourceManagerForApply, manifests []byte) error {
	log.Actionf("Installing the GitOps Dashboard ...")

	applyOutput, err := apply(ctx, log, manager, manifests)
	if err != nil {
		log.Failuref("GitOps Dashboard install failed")
		return err
	}

	log.L().V(loglevels.LogLevelInfo).Info(applyOutput)

	return nil
}

// GetInstalledDashboard checks if the GitOps Dashboard is installed.
func GetInstalledDashboard(ctx context.Context, kubeClient client.Client, namespace string, dashboards map[DashboardType]bool) (DashboardType, string) {
	helmReleaseList := &helmv2.HelmReleaseList{}

	if err := kubeClient.List(ctx, helmReleaseList,
		client.InNamespace(namespace),
	); err != nil {
		return DashboardTypeNone, ""
	}

	shouldDetectOSSDashboard := dashboards[DashboardTypeOSS]
	shouldDetectEnterpriseDashboard := dashboards[DashboardTypeEnterprise]

	ossDashboardInstalled := false
	dashboardName := ""

	for _, helmRelease := range helmReleaseList.Items {
		chartSpec := helmRelease.Spec.Chart.Spec

		if shouldDetectEnterpriseDashboard && chartSpec.Chart == enterpriseDashboardHelmChartName &&
			chartSpec.SourceRef.Name == enterpriseDashboardHelmRepositoryName {
			return DashboardTypeEnterprise, helmRelease.Name
		}

		if shouldDetectOSSDashboard && chartSpec.Chart == ossDashboardHelmChartName {
			ossDashboardInstalled = true
			dashboardName = helmRelease.Name
		}
	}

	if ossDashboardInstalled {
		return DashboardTypeOSS, dashboardName
	}

	return DashboardTypeNone, ""
}

// ReconcileDashboard reconciles the dashboard. If podName is an empty string, it will get the dashboard pod by labels instead of pod name.
func ReconcileDashboard(ctx context.Context, kubeClient client.Client, dashboardName, namespace, podName string, timeout time.Duration) error {
	const interval = 3 * time.Second / 2

	helmChartName := namespace + "-" + dashboardName

	// reconcile dashboard
	namespacedName := types.NamespacedName{
		Namespace: namespace,
		Name:      helmChartName,
	}
	gvk := schema.GroupVersionKind{
		Group:   "source.toolkit.fluxcd.io",
		Version: "v1beta2",
		Kind:    sourcev1.HelmChartKind,
	}

	var sourceRequestedAt string

	if err := wait.Poll(interval, timeout, func() (bool, error) {
		var err error
		sourceRequestedAt, err = run.RequestReconciliation(ctx, kubeClient,
			namespacedName, gvk)

		return err == nil, nil
	}); err != nil {
		return err
	}

	// wait for the reconciliation of dashboard to be done
	if err := wait.Poll(interval, timeout, func() (bool, error) {
		dashboard := &sourcev1.HelmChart{}
		if err := kubeClient.Get(ctx, types.NamespacedName{
			Namespace: namespace,
			Name:      helmChartName,
		}, dashboard); err != nil {
			return false, err
		}

		return dashboard.Status.GetLastHandledReconcileRequest() == sourceRequestedAt, nil
	}); err != nil {
		return err
	}

	// wait for dashboard to be ready
	if err := wait.Poll(interval, timeout, func() (bool, error) {
		namespacedName := types.NamespacedName{Namespace: namespace, Name: podName}

		var labels map[string]string = nil

		if podName == "" {
			labels = map[string]string{
				coretypes.InstanceLabel: dashboardName,
				coretypes.NameLabel:     ossDashboardHelmChartName,
			}
		}

		dashboard, _ := run.GetPodFromResourceDescription(ctx, kubeClient, namespacedName, "deployment", labels)
		if dashboard == nil {
			return false, nil
		}

		return isPodStatusConditionPresentAndEqual(dashboard.Status.Conditions, corev1.PodReady, corev1.ConditionTrue), nil
	}); err != nil {
		return err
	}

	return nil
}

// generateManifestsForDashboard generates dashboard manifests from objects.
func generateManifestsForDashboard(log logger.Logger, helmRepository *sourcev1.HelmRepository, helmRelease *helmv2.HelmRelease) ([]byte, error) {
	helmRepositoryData, err := yaml.Marshal(helmRepository)
	if err != nil {
		log.Failuref("Error generating HelmRepository manifest from object")
		return nil, err
	}

	helmReleaseData, err := yaml.Marshal(helmRelease)
	if err != nil {
		log.Failuref("Error generating HelmRelease manifest from object")
		return nil, err
	}

	sanitizedHelmRepositoryData, err := SanitizeResourceData(log, helmRepositoryData)
	if err != nil {
		log.Failuref("Error sanitizing HelmRepository data")
		return nil, err
	}

	sanitizedHelmReleaseData, err := SanitizeResourceData(log, helmReleaseData)
	if err != nil {
		log.Failuref("Error sanitizing HelmRelease data")
		return nil, err
	}

	divider := []byte("---\n")

	content := sanitizedHelmRepositoryData
	content = append(content, divider...)
	content = append(content, sanitizedHelmReleaseData...)

	return content, nil
}

// makeHelmRepository creates a HelmRepository object for installing the GitOps Dashboard.
func makeHelmRepository(name, namespace string) *sourcev1.HelmRepository {
	helmRepository := &sourcev1.HelmRepository{
		TypeMeta: metav1.TypeMeta{
			Kind:       sourcev1.HelmRepositoryKind,
			APIVersion: sourcev1.GroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "weave-gitops-dashboard",
				"app.kubernetes.io/component":  "ui",
				"app.kubernetes.io/part-of":    "weave-gitops",
				"app.kubernetes.io/created-by": "weave-gitops-cli",
			},
			Annotations: map[string]string{
				"metadata.weave.works/description": "This is the source location for the Weave GitOps Dashboard's helm chart.",
			},
		},
		Spec: sourcev1.HelmRepositorySpec{
			URL:  helmRepositoryURL,
			Type: "oci",
			Interval: metav1.Duration{
				Duration: time.Minute * 60,
			},
		},
	}

	return helmRepository
}

// makeHelmRelease creates a HelmRelease object for installing the GitOps Dashboard.
func makeHelmRelease(log logger.Logger, name, namespace, username, passwordHash, chartVersion, dashboardImage string) (*helmv2.HelmRelease, error) {
	helmRelease := &helmv2.HelmRelease{
		TypeMeta: metav1.TypeMeta{
			Kind:       helmv2.HelmReleaseKind,
			APIVersion: helmv2.GroupVersion.Identifier(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Annotations: map[string]string{
				"metadata.weave.works/description": "This is the Weave GitOps Dashboard.  It provides a simple way to get insights into your GitOps workloads.",
			},
		},
		Spec: helmv2.HelmReleaseSpec{
			Interval: metav1.Duration{
				Duration: 60 * time.Minute,
			},
			Chart: helmv2.HelmChartTemplate{
				Spec: helmv2.HelmChartTemplateSpec{
					Chart: ossDashboardHelmChartName,
					SourceRef: helmv2.CrossNamespaceObjectReference{
						Kind: sourcev1.HelmRepositoryKind,
						Name: name,
					},
				},
			},
		},
	}

	if chartVersion != "" {
		helmRelease.Spec.Chart.Spec.Version = chartVersion
	}

	values, err := makeValues(username, passwordHash, dashboardImage)
	if err != nil {
		log.Failuref("Error generating chart values")
		return nil, err
	}

	if values != nil {
		helmRelease.Spec.Values = &apiextensionsv1.JSON{Raw: values}
	}

	return helmRelease, nil
}

func parseImageRepository(input string) (repository, image, tag string, err error) {
	lastSlashIndex := strings.LastIndex(input, "/")
	if lastSlashIndex == -1 {
		subComponents := strings.Split(input, ":")
		switch len(subComponents) {
		case 1:
			image = subComponents[0]
			repository = ""
			tag = ""
		case 2:
			image = subComponents[0]
			tag = subComponents[1]
			repository = ""
			if tag == "" || image == "" {
				err = fmt.Errorf("invalid input format, repo = %s, image = %s, tag = %s", repository, image, tag)
				return
			}
		default:
			err = fmt.Errorf("invalid input format, input = %s", input)
			return
		}
	} else {
		repository = input[:lastSlashIndex]
		imageAndTag := input[lastSlashIndex+1:]
		subComponents := strings.Split(imageAndTag, ":")
		if len(subComponents) > 1 {
			image = subComponents[0]
			tag = subComponents[1]
		} else {
			image = subComponents[0]
			tag = "latest"
		}
	}

	if image == "" {
		err = fmt.Errorf("invalid input format, repo = %s, image = %s, tag = %s", repository, image, tag)
		return
	}

	return
}

// makeValues creates a values object for installing the GitOps Dashboard.
func makeValues(username, passwordHash, dashboardImage string) ([]byte, error) {
	valuesMap := make(map[string]interface{})
	if username != "" && passwordHash != "" {
		valuesMap["adminUser"] =
			map[string]interface{}{
				"create":       true,
				"username":     username,
				"passwordHash": passwordHash,
			}
	}

	gitopsConfig, err := config.GetConfig(false)
	if err == nil && gitopsConfig.Analytics {
		valuesMap["WEAVE_GITOPS_FEATURE_TELEMETRY"] = "true"
	}

	if dashboardImage != "" {
		// check : and spit on it
		// detect the right most colon

		repository, image, tag, err := parseImageRepository(dashboardImage)
		if err != nil {
			return nil, err
		}

		valuesMap["image"] = map[string]interface{}{
			"repository": strings.TrimPrefix(repository+"/"+image, "/"),
			"tag":        tag,
		}
	}

	if len(valuesMap) > 0 {
		jsonRaw, err := json.Marshal(valuesMap)
		if err != nil {
			return nil, fmt.Errorf("encoding values failed: %w", err)
		}

		return jsonRaw, nil
	}

	return nil, nil
}

func SanitizeResourceData(log logger.Logger, resourceData []byte) ([]byte, error) {
	// remove status
	resNode, err := kyaml.Parse(string(resourceData))
	if err != nil {
		return nil, fmt.Errorf("failed to parse resource data: %v", err.Error())
	}

	res := &resource.Resource{RNode: *resNode}

	err = res.PipeE(kyaml.FieldClearer{Name: "status"})
	if err != nil {
		return nil, fmt.Errorf("failed to remove status: %v", err.Error())
	}

	// remove creationTimestamp
	metadataNode, err := res.Pipe(kyaml.Get("metadata"))
	if err != nil {
		return nil, fmt.Errorf("failed to get metadata: %v", err.Error())
	}

	metadataRes := &resource.Resource{RNode: *metadataNode}

	err = metadataRes.PipeE(kyaml.FieldClearer{Name: "creationTimestamp"})
	if err != nil {
		return nil, fmt.Errorf("failed to remove creationTimestamp: %v", err.Error())
	}

	err = res.PipeE(kyaml.FieldSetter{Name: "metadata", Value: &metadataRes.RNode})
	if err != nil {
		return nil, fmt.Errorf("failed to set metadata: %v", err.Error())
	}

	resourceData, err = res.AsYAML()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource to YAML: %v", err.Error())
	}

	return resourceData, nil
}
