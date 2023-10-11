package main

import (
	"os"

	"github.com/saschagrunert/demo"
)

func main() {
	d := demo.New()
	d.Name = "CRI-O demo for KubeCon NA 2023"
	d.Usage = "How to use the new deb and rpm packages"

	d.Add(sigstore(), "sigstore", "Validating sigstore signatures")
	d.Add(rpm(), "rpm", "Using rpm packages")
	d.Add(deb(), "deb", "Using deb packages")
	d.Run()
}

func sigstore() *demo.Run {
	r := demo.NewRun("How to validate sigstore signatures with CRI-O")

	r.Step(demo.S(
		"Assuming that we have an up and running Kubernetes",
		"cluster using CRI-O v1.28",
	), demo.S(
		"crio --version | head",
	))

	r.Step(nil, demo.S(
		"kubectl get nodes",
	))

	r.Step(demo.S(
		"We need to ensure that CRI-O runs with",
		"enabled sigstore support",
	), demo.S(
		"bat /etc/containers/registries.d/default.yaml",
	))

	r.Step(demo.S(
		"The default policy does not specify anything of interest",
	), demo.S(
		"jq . /etc/containers/policy.json",
	))

	r.Step(demo.S(
		"Let's create a new namespaced policy for the default namespace",
	), demo.S(
		"cat default.json | sudo tee /etc/crio/policies/default.json | cut -c-80",
	))

	r.Step(demo.S(
		"And try to pull the image specifying the sandbox config,",
		"which contains the referenced namespace",
	), demo.S(
		"jq . sandbox_config.json",
	))

	r.Step(nil, demo.S(
		"sudo crictl pull --pod-config sandbox_config.json quay.io/crio/signed",
	))

	r.Step(demo.S(
		"Awesome, this works as expected. But now let's",
		"modify the subjectEmail to become wrong",
	), demo.S(
		"sudo sed -i 's;sgrunert@redhat.com;wrong;g' /etc/crio/policies/default.json",
	))

	r.Step(nil, demo.S(
		"sudo crictl rmi quay.io/crio/signed",
	))

	r.Step(demo.S(
		"If we now pull again, then the signature verification fails",
	), demo.S(
		"sudo crictl pull --pod-config sandbox_config.json quay.io/crio/signed || true",
	))

	r.Step(demo.S(
		"The same validation works with a Kubernetes pod",
	), demo.S(
		"bat pod.yaml",
	))

	r.Step(demo.S(
		"If we apply the pod",
	), demo.S(
		"kubectl apply -f pod.yaml",
	))

	r.Step(demo.S(
		"Then it fails to start for the same reason",
	), demo.S(
		"kubectl describe pod pod | grep Failed",
	))

	r.Step(nil, demo.S(
		"kubectl delete pod pod",
	))

	r.Step(demo.S(
		"Kubernetes also features a new status SignatureValidationFailed",
		"for example if we change the image to become unsigned",
	), demo.S(
		"sed -i 's;signed;unsigned;g' pod.yaml",
	))

	r.Step(demo.S(
		"And apply the pod manifest",
	), demo.S(
		"kubectl apply -f pod.yaml",
	))

	r.Step(demo.S(
		"Then the new status will get populated through the CLI",
	), demo.S(
		"kubectl get pod pod",
	))

	return r
}

const (
	streamKey = "PACKAGE_STREAM"
	streamVal = "prerelease:/main"
)

func rpm() *demo.Run {
	r := demo.NewRun("How to use the CRI-O rpm packages")

	r.Step(demo.S(
		"There are multiple package streams available",
		"- stable/v1.29",
		"- stable/v1.28",
		"- prerelease/main",
		"- prerelease/release-1.29",
		"- prerelease/release-1.28",
		"",
		"We have to choose one of them,",
		"for example the latest one from `main`",
	), demo.S(
		streamKey+"="+streamVal,
	))
	os.Setenv(streamKey, streamVal)

	r.Step(demo.S(
		"Kubernetes itself lives in a dedicated repository,",
		"which we have to add first",
	), demo.S(
		"KUBERNETES_VERSION=v1.28 &&",
		`cat <<EOF | tee /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://pkgs.k8s.io/core:/stable:/$KUBERNETES_VERSION/rpm/
enabled=1
gpgcheck=1
gpgkey=https://pkgs.k8s.io/core:/stable:/$KUBERNETES_VERSION/rpm/repodata/repomd.xml.key
EOF`,
	))

	r.Step(demo.S(
		"Then we can add the CRI-O repository",
	), demo.S(
		`cat <<EOF | tee /etc/yum.repos.d/cri-o.repo
[cri-o]
name=CRI-O
baseurl=https://pkgs.k8s.io/addons:/cri-o:/$`+streamKey+`/rpm/
enabled=1
gpgcheck=1
gpgkey=https://pkgs.k8s.io/addons:/cri-o:/$`+streamKey+`/rpm/repodata/repomd.xml.key
EOF`,
	))

	r.Step(demo.S(
		"Now we can install the required packages",
	), demo.S(
		"dnf install -y", "cri-o", "kubeadm", "kubectl", "kubelet", "kubernetes-cni",
	))

	r.Step(demo.S(
		"Starting CRI-O",
	), demo.S(
		"systemctl start crio.service",
	))

	r.Step(demo.S("Preparing the node"), demo.S("dnf remove -y zram-generator-defaults"))
	r.Step(nil, demo.S("systemctl stop dev-zram0.swap"))
	r.Step(nil, demo.S("swapoff -a"))
	r.Step(nil, demo.S("modprobe br_netfilter"))
	r.Step(nil, demo.S("sysctl -w net.ipv4.ip_forward=1"))

	r.Step(demo.S("Bootstrapping the cluster"), demo.S("kubeadm init"))

	r.Step(demo.S(
		"Verifying that the cluster is up and running",
	), demo.S(
		"export KUBECONFIG=/etc/kubernetes/admin.conf &&",
		"kubectl taint nodes --all node-role.kubernetes.io/control-plane- &&",
		"kubectl wait -n kube-system --timeout=180s --for=condition=available deploy coredns &&",
		"kubectl wait --timeout=180s --for=condition=ready pods --all -A &&",
		"kubectl get pods -A &&",
		"kubectl run -i --restart=Never --image debian --rm debian -- echo test | grep test",
	))

	return r
}

func deb() *demo.Run {
	r := demo.NewRun("How to use the CRI-O deb packages")

	const (
		streamKey = "PACKAGE_STREAM"
		streamVal = "prerelease:/main"
	)

	r.Step(demo.S(
		"There are multiple package streams available",
		"- stable/v1.29",
		"- stable/v1.28",
		"- prerelease/main",
		"- prerelease/release-1.29",
		"- prerelease/release-1.28",
		"",
		"We have to choose one of them,",
		"for example the latest one from `main`",
	), demo.S(
		streamKey+"="+streamVal,
	))
	os.Setenv(streamKey, streamVal)

	r.Step(demo.S(
		"Kubernetes itself lives in a dedicated repository,",
		"which we have to add first",
	), demo.S(
		"KUBERNETES_VERSION=v1.28 &&",
		"curl -fsSL https://pkgs.k8s.io/core:/stable:/$KUBERNETES_VERSION/deb/Release.key |",
		"gpg --dearmor -o /etc/apt/keyrings/kubernetes-apt-keyring.gpg &&",
		`echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] https://pkgs.k8s.io/core:/stable:/$KUBERNETES_VERSION/deb/ /" |`,
		"tee /etc/apt/sources.list.d/kubernetes.list",
	))

	r.Step(demo.S(
		"Then we can add the CRI-O repository",
	), demo.S(
		"curl -fsSL https://pkgs.k8s.io/addons:/cri-o:/$"+streamKey+"/deb/Release.key |",
		"gpg --dearmor -o /etc/apt/keyrings/cri-o-apt-keyring.gpg &&",
		`echo "deb [signed-by=/etc/apt/keyrings/cri-o-apt-keyring.gpg] https://pkgs.k8s.io/addons:/cri-o:/$`+streamKey+`/deb/ /" |`,
		"tee /etc/apt/sources.list.d/cri-o.list",
	))

	r.Step(demo.S(
		"Now we can install the required packages",
	), demo.S(
		"apt-get update &&",
		"apt-get install -y", "cri-o", "kubeadm", "kubectl", "kubelet", "kubernetes-cni",
	))

	r.Step(demo.S(
		"Starting CRI-O",
	), demo.S(
		"systemctl start crio.service",
	))

	r.Step(demo.S("Preparing the node"), demo.S("swapoff -a"))
	r.Step(nil, demo.S("modprobe br_netfilter"))
	r.Step(nil, demo.S("sysctl -w net.ipv4.ip_forward=1"))

	r.Step(demo.S("Bootstrapping the cluster"), demo.S("kubeadm init"))

	r.Step(demo.S(
		"Verifying that the cluster is up and running",
	), demo.S(
		"export KUBECONFIG=/etc/kubernetes/admin.conf &&",
		"kubectl taint nodes --all node-role.kubernetes.io/control-plane- &&",
		"kubectl wait -n kube-system --timeout=180s --for=condition=available deploy coredns &&",
		"kubectl wait --timeout=180s --for=condition=ready pods --all -A &&",
		"kubectl get pods -A &&",
		"kubectl run -i --restart=Never --image debian --rm debian -- echo test | grep test",
	))

	return r
}
