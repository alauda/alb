
alb=$(kubectl get pod -n cpaas-system|grep alb| awk '{print $1}'| tr -d '\n')
kubectl exec -it ${alb} -n cpaas-system /bin/sh