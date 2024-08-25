# Chains Examples

## interchainsecurity & neutron

Current the neutron heighliner image does not work due to some /tmp issue in the docker image. For this reason, you must compile the image yourself to use the latest v50 instance.

```bash
git clone https://github.com/neutron-org/neutron.git
cd neutron
git checkout v4.2.1

# neutron-node:latest
make build-e2e-docker-image
```
