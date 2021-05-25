
alb=$(kubectl get pod -n cpaas-system|grep alb| awk '{print $1}'| tr -d '\n')
kubectl port-forward -n cpaas-system ${alb}  8080:80 