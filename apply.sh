#!/bin/bash
RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

apply_one() {
    case_idx=$1
    test_case=$2
    last_line=$3
    
    echo -e "-----${GREEN}start case [${backend}/${case_idx}]${NC} ${RED}${test_case}${NC}-----"
    DESIRED=-1
    READY=0
    
    kubectl apply -f k8s/${backend}/${test_case}.yaml
    sleep 10
    while [ "${DESIRED}" != "${READY}" ]; do
        DESIRED=$(kubectl get ds | grep ${test_case} | awk '{print $2}')
        READY=$(  kubectl get ds | grep ${test_case} | awk '{print $4}')
        echo "wait test_case ${test_case} until DESIRED(${DESIRED}) == READY(${READY})" && sleep 1
    done
    sleep 10
    ./analysis --kubeconfig ~/.kube/config --endline="${last_line}" --app=${test_case} --idx=${case_idx} --backend=${backend} --output ${filename}
    
    kubectl delete -f k8s/${backend}/${test_case}.yaml
    sleep 10
}

apply_backend() {
    backend=$1
    # if [ -z ${backend} ];then backend="nydus" ;fi
    echo -e "\n-----==== Benchmark via Backend: ${RED}${backend}${NC} ====-----\n"
    
    for (( i = 0; i < ${#test_cases[*]}; i++ )); do
        apply_one "${i}" "${test_cases[i]}" "${last_lines[i]}"
    done
}

test_cases=('scf-2g'  'phpmyadmin'  'mysql' 'rethinkdb' 'python'    'nginx' 'glassfish' 'gcc'       'files-1w'  'files-1g')
# last_lines=('empty'     'Server ready'  'finished'  '200 612'   'OSGi service registration' 'finished'  'finished'      'finished')
last_lines=(''        ''      ''      ''          ''  ''      ''          ''  ''  '')
filename="data/bench_$(date +%Y-%m-%d_%H-%M).xlsx"
backends=('apparate'  'nydus' 'docker')
# backends=('apparate')


for backend in ${backends[*]}; do
    apply_backend ${backend}
done
