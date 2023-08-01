这里的config是所有的配置相关的逻辑
1. operator所使用,如何将cr上的配置转成具体的全部的配置
2. alb容器所使用的配置,alb会根据这些配置做事情  
     目前alb容器是读取环境变量的

从cr上能够获取到ALbRunConfig ToEnv  
从环境变量上也能获取到AlbRunConfig FromEnv  
整个的逻辑块为  
operator   
  补全cr => external
  整理config  external => internal (merge)

整理env    interval => env   (必须在operator中)
从env中恢复   env=>interval  ()

alb  
  更多的配置 alb ext  
  读写锁 config现在是不可变的,单例  

alb import and extend operator

common中有AlbRunConfig 有toEnv和FromEnv
operator中有 fillup merge
alb中有