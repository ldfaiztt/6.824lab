#MIT 6.824lab

基于Go的分布式系统课的实验,跟着做了一下.

当然模板代码里面也有很多不足的地方,抛开源代码的因素,作为分布式的学习资源还是不错的,故跟着做了一边,
一些代码的错误我列在了下面,这些修改,是基于commit(f9ba545e)这个提交的修改,请在动手前查看一下,省得自己再踩坑.

源课程链接:http://nil.csail.mit.edu/6.824/2015/.

注意:
    进行实验之前,因为源代码的模板目录结构原因,请执行`export GOPATH=`pwd``,实验完之后.
    恢复成原来的工程目录的GOPATH,或者直接退出当前terminal的session.
    我的版本是写了答案的,如果想自己尝试可以checkout最初的commit(f9ba545e)即可.

修改:
    在lab1的第二部分:MapReduce的alive变量存在race condition,要加锁.
