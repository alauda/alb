kubectl delete configmap -n alb-wc alb-wc-nginx-config
kubectl delete Deployment -n alb-wc echo-resty
kubectl delete Service -n alb-wc echo-resty