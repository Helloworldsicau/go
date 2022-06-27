package main

import (
	"github.com/gin-gonic/gin"
	conf "github.com/melf-xyzh/gin-start/config"
	"github.com/melf-xyzh/gin-start/global"
	"github.com/melf-xyzh/gin-start/global/check"
	"github.com/melf-xyzh/gin-start/middleware"
	usermod "github.com/melf-xyzh/gin-start/user/model"
	"github.com/melf-xyzh/gin-start/utils/result"
)

func main() {

	init := conf.Init{}
	// 初始化Viper（读取配置文件）
	// 初始化数据库连接池
	global.DB = init.Mysql(global.ENV_DEV)
	// 初始化雪花ID节点
	global.Node = init.Node()
	// 初始化Redis连接池
	global.RDB = init.Redis(global.ENV_DEV)
	// 初始化参数校验器
	global.Validate = init.Validate()
	// 初始化Casbin
	global.Enforcer = init.Casbin(global.ENV_DEV)

	r := gin.New()
	r.Use(middleware.Cors())

	err := global.DB.AutoMigrate(
		usermod.User{},
	)
	if err != nil {
		panic("数据迁移失败")
	}

	r.GET("/", func(c *gin.Context) {

		user := usermod.User{
			Model: global.Model{
				ID:         global.CreateId(),
				CreateTime: global.CreateTime(),
			},
			Name:        "MELF",
			Email:       "123456789@99.com",
			LastLoginIp: "8.8.8.8",
		}
		err = check.Check(user, usermod.CreateUserCheck)
		if err != nil {
			return
		}
		//err = global.DB.Create(&user).Error

		var userFind usermod.User
		global.DB.First(&userFind)
		userFind.LastLoginIp = "192.168.1.11"
		global.DB.Updates(&userFind)
		result.OkDataMsg(c, userFind, "创建成功")
	})

	r.GET("/aaa/", middleware.Rate0("4-H"), func(c *gin.Context) {
		var userFind usermod.User
		global.DB.First(&userFind)
		result.OkDataMsg(c, userFind, "创建成功")
	})

	// 启动服务
	init.Run(r)

}
