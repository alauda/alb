现在alb的代码中有三重测试方式
##  unit test
测试代码放在源代码相同的package
一般用于测具体的函数.
## env test
测试代码放在test/e2e下,和unit test的区别是,env test使用会启动envtest并直接启动一个alb或者operator.
一般用于测alb和operator的行为
## kind test
测试代码放在test/kind下,和env test的区别是,kind test会使用kind部署一个k8s集群,并部署operator.
一般用于测试nginx相关的逻辑,和operator部署相关的逻辑.
