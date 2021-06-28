#!/bin/bash
END=500
pattern=$1
for ((i=0;i<END;i++)); do
    echo $i
	kubectl delete rule -n cpaas-system $pattern-${i}
done