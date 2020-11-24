# 负载均衡规则优先级迁移

http://confluence.alauda.cn/pages/viewpage.action?pageId=78809412

开放rule的priority字段，范围为1-10，1最高，10最低。

历史数据迁移，历史rule用户手动创建的优先级默认为5，ingress翻译来的优先级默认为5。

需要兼容业务集群和global版本不同的情况，业务集群未升级时，规则优先级只按内部优先级生效。想使用界面配置优先级必须做升级。