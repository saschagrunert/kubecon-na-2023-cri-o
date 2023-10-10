all: build

.PHONY: build
build:
	CGO_ENABLED=0 go build -o demo

.PHONY: vagrant-rpm
vagrant-rpm: build
	ln -sf Vagrantfile-rpm Vagrantfile
	vagrant up

.PHONY: rpm
rpm:
	vagrant ssh -- sudo demo --rpm

.PHONY: vagrant-deb
vagrant-deb: build
	ln -sf Vagrantfile-deb Vagrantfile
	vagrant up

.PHONY: deb
deb:
	vagrant ssh -- sudo demo --deb
