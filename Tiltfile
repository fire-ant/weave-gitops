# Anyone seen Fast and Furious? :)

# Allow a K8s context named wego-dev, in addition to the local cluster
allow_k8s_contexts('wego-dev')

if os.getenv('MANUAL_MODE'):
   trigger_mode(TRIGGER_MODE_MANUAL)

# Support IMAGE_REPO env so that we can run Tilt with a remote cluster
image_repository = os.getenv('IMAGE_REPO', 'localhost:5001/weaveworks/wego-app')

load('ext://restart_process', 'docker_build_with_restart')

advanced_go_dev_mode = os.getenv('FAST_AND_FURIOUSER')
skip_ui_build = os.getenv("SKIP_UI_BUILD")

if advanced_go_dev_mode:

    local_resource(
        'gitops-server',
        'GOOS=linux make gitops-server',
        deps=[
            './cmd',
            './pkg',
            './core',
            './api',
        ]
    )

    if not skip_ui_build:
        local_resource(
            'ui-server',
            'make ui',
            deps=[
                './ui',
            ]
        )

    docker_build_with_restart(
        image_repository,
        '.',
        only=[
            './bin',
        ],
        dockerfile="dev.dockerfile",
        entrypoint="/app/build/gitops-server --log-level=debug --insecure",
        live_update=[
            sync('./bin', '/app/build'),
        ],
    )
else:
    docker_build(
        image_repository,
        '.',
        dockerfile="gitops-server.dockerfile",
    )

# Override image.repository of the dev Helm chart with image_repository
k8s_yaml(helm('./charts/gitops-server', name='dev', values='./tools/helm-values-dev.yaml', set=['image.repository=' + image_repository]))
k8s_yaml(helm('./tools/charts/dev', name='dev', values='./tools/charts/dev/values.yaml'))

deps = ['gitops-server'] if advanced_go_dev_mode else []

if not skip_ui_build:
    deps.append('ui-server')


k8s_resource('dev-weave-gitops', port_forwards='9001', resource_deps=deps)
