# jaeger mysql 插件说明
一个jaeger后端存储扩展，使用mysql作为存储后端。

# 适用场景
- 计算资源有限
- 数据量不是特别大。（千万级别）
- 高可用

# require
- mysql >= 5.7

# feature
- 批量异步写入，单实例 3000qps+
- 可配置定时删除过期数据

# 源码
插件源代码在src目录下的jaeger目录里。

目录结构如下：
```
jaeger
- plugin
  - stoarge
    - mysql
- pkg
  - mysql
```
具体代码是在上面两个mysql文件夹里。

# 接入方式
- 下载jaeger原仓库代码
   git clone https://github.com/jaegertracing/jaeger.git
- 将上述两个msyql文件夹，按照上面目录的顺序，放入jaeger源码相应的目录
- 更改jaeger/plugin/storage/factory.go文件，注册这个mysql插件
```go
    const mysqlStorageType         = "mysql"  // 声明存储类型
    
	// 在getFactoryOfType函数里，增加
	case mysqlStorageType
		return mysql.NewFactory(), nil
```	

# 启动
- 编译获取二进制可执行文件。
  按照上面说明增加代码后，自行编译。 
          or
  在bin目录里有已经本地打好的二进制文件bin/jaeger/all-in-one-linux   
- 执行sql/full.sql 初始化相应的表
- 设置参数env参数 SPAN_STORAGE_TYPE: "mysql"。


```
bin/jaeger/all-in-one-linux --config-file=config/config.yaml
```

