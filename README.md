# jaeger mysql 插件说明

代码在src目录下的jaeger目录里。

目录结构如下：
```
jaeger
- plugin
  - stoarge
    - mysql
- pkg
  - mysql
```

具体代码是在上面两个mysql文件夹里，具体接入方式为：
- 将上述两个msyql文件夹，按照上面目录的顺序，放入jaeger源码相应的目录
- 更改plugin/storage/factory.go文件，注册这个mysql插件
```
    const mysqlStorageType         = "mysql"  // 声明存储类型
	在getFactoryOfType函数里，增加
	case mysqlStorageType
		return mysql.NewFactory(), nil
```	

使用方式：
- 按照上面说明增加代码后。
- 在启动项目时，设置参数env参数 SPAN_STORAGE_TYPE: "mysql" 

补充
- 在启动之前需要先初始化sql文件


# start 
./all-in-one-linux --config-file=config.yaml
