set -ex

make docker-build IMG=harbor.service.mohansinglenode.uhana.io/uhana/local-pv-operator:v0.0.1
make docker-push IMG=harbor.service.mohansinglenode.uhana.io/uhana/local-pv-operator:v0.0.1
make deploy IMG=harbor.service.mohansinglenode.uhana.io/uhana/local-pv-operator:v0.0.1
kubectl rollout restart deployment operators-controller-manager
