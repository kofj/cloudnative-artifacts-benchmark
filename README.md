# cloudnative-artifacts-benchmark
Cloud Native Artifacts Benchmark

## Usage

### Apparate


### Nydus
At first, adds your hosts to the `[nydus]` block of `ansible/hosts`

#### Install
```bash
ansible-playbook -i ansible/hosts ansible/nydus.yaml -u root
```

#### Benchmark


#### Remove images
```bash
ansible-playbook -i ansible/hosts ansible/nydus.yaml -u root -t clean
```
### Stargz
