# etcd 源码阅读


### 为什么有这个项目
2019其中一个目标是深入学习golang和分布式系统，个人觉得技术方面学习一个东西最快的方式就是阅读优秀源码（如最初学习PHP就是因为阅读了ecshop和腾讯微博的源码使水平提升了一大档次。其他技术也一样，学习iOS阅读了仿爱鲜蜂和仿微博的项目等）。那同时学习这2个就最好找个基于golang的分布式项目来阅读。综合项目star数、流行度、项目复杂度，那etcd就是最合适的了。

#### 我学到了什么
* golang的代码规范
* golang的面向接口编程
* golang的显示、隐式调用
* golang大型项目的结构和架构
* raft协议的基本原理
* 基于raft协议的分布式系统工程实现

---
![etcd go StartEtcd](https://user-images.githubusercontent.com/2595251/66731246-ff6ff100-ee88-11e9-86a6-ff6fc588ec09.png)