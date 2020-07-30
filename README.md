# Service Discovery for VM Telemetry
## Overview
This repo contains an experimental feature to support VM
telemetry with [file-based service discovery](https://prometheus.io/docs/prometheus/latest/configuration/configuration/#file_sd_config) of Prometheus.
For more information, one could access the RFC [here](https://docs.google.com/document/d/1gP12rR2fUV0iHpABiwFiQSy-M1AfyR2XDbQvQil64-M/edit?usp=sharing).
This repo provides a binary to be run with Istio, which would watch
the updates to the workload entries registered with VMs, and
write the endpoint IP to a config map. The config map will then
be mounted by the Prometheus as file, thus the service discovery.
A sample of Prometheus deployment could be found in `kubernetes/prometheus.yaml`.

## Usage
To build the binary, simply run:
```
make build
```
The binary will be written to `out` directory. 

Then, run the binary:
```
cd out/ && ./vm-discovery
```
It will watch the events and handle the update accordingly.

**Note: the binary must be run after the installation of Istio,
or it will fail.**
