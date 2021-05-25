alb=$(kubectl get pod -n cpaas-system|grep alb| awk '{print $1}'| tr -d '\n')
kubectl logs -f  ${alb} -n cpaas-system log-sidecar