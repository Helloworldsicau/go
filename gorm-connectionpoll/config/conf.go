/**
 * @Time    :2022/2/19 9:36
 * @Author  :MELF晓宇
 * @Email   :xyzh.melf@petalmail.com
 * @FileName:conf.go
 * @Project :gin-start
 * @Blog    :https://blog.csdn.net/qq_29537269
 * @Guide   :https://guide.melf.space
 * @Information:
 *
 */

package conf

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/bwmarrin/snowflake"
	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v8"
	"github.com/melf-xyzh/gin-start/global"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/soheilhy/cmux"
	"github.com/spf13/viper"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"log"
	"net"
	"strings"
	"time"
)

type Init struct{}

func (i Init) EnvActive() *global.Env {
	v := viper.New()
	// 配置文件路径
	v.AddConfigPath("resource")
	// 配置文件名
	v.SetConfigName("active")
	// 配置文件类型
	v.SetConfigType("yaml")
	// 读取配置文件信息
	err := v.ReadInConfig()
	if err != nil {
		panic("读取配置文件信息失败：" + err.Error())
	}
	active := v.GetString("active")
	log.Println("当前环境：" + active)
	switch active {
	case "fat":
		return i.Env(global.ENV_FAT)
	case "dev":
		return i.Env(global.ENV_DEV)
	case "pro":
		return i.Env(global.ENV_PRO)
	default:
		return i.Env(global.ENV_DEV)
	}
}
func (i Init) Env(e global.Env) *global.Env {
	return &e
}

// Env
/**
 * @Description: 初始化环境
 * @receiver i
 * @param e
 * @return env
 */

// Viper
/**
 * @Description:  初始化Viper(读取配置文件)
 * @receiver i
 * @return *viper.Viper
 */
func (i Init) Viper() *viper.Viper {
	v := viper.New()
	// 配置文件路径
	v.AddConfigPath("resource")
	// 配置文件名
	v.SetConfigName(*global.E + "_config")
	// 配置文件类型
	v.SetConfigType("yaml")
	// 读取配置文件信息
	err := v.ReadInConfig()
	if err != nil {
		panic("读取配置文件信息失败：" + err.Error())
	}
	return v
}

// Database
/**
 * @Description: 初始化数据库
 * @receiver i
 */
func (i Init) Mysql(env global.Env) (db *gorm.DB) {
	Prepare(env)
	var err error
	dbType := global.V.GetString("Database.Type")
	dbHost := global.V.GetString("Database.DbHost")
	dbPort := global.V.GetString("Database.DbPort")
	dbUser := global.V.GetString("Database.DbUser")
	dbPassword := global.V.GetString("Database.DbPassword")
	dbName := global.V.GetString("Database.DbName")

	switch strings.ToLower(dbType) {
	case "mysql":
		dsn := dbUser + ":" + dbPassword + "@tcp(" + dbHost + ":" + dbPort + ")/" + dbName + "?charset=utf8&parseTime=True&loc=Local"
		mysqlConfig := mysql.Config{
			DSN:                       dsn,   // DSN data source name
			DefaultStringSize:         256,   // string 类型字段的默认长度
			DisableDatetimePrecision:  true,  // 禁用 datetime 精度，MySQL 5.6 之前的数据库不支持
			DontSupportRenameIndex:    true,  // 重命名索引时采用删除并新建的方式，MySQL 5.7 之前的数据库和 MariaDB 不支持重命名索引
			DontSupportRenameColumn:   true,  // 用 `change` 重命名列，MySQL 8 之前的数据库和 MariaDB 不支持重命名列
			SkipInitializeWithVersion: false, // 根据当前 MySQL 版本自动配置
		}
		db, err = gorm.Open(mysql.New(mysqlConfig), &gorm.Config{})
		if err != nil {
			panic("初始化数据库连接池失败：" + err.Error())
		}
	default:
		panic("暂不支持")
	}

	maxIdleConns := global.V.GetInt("Database.MaxIdleConns")
	maxOpenConns := global.V.GetInt("Database.MaxOpenConns")
	connMaxLifetime := global.V.GetInt("Database.ConnMaxLifetime")
	connMaxIdleTime := global.V.GetInt("Database.ConnMaxIdleTime")
	var sqlDB *sql.DB
	sqlDB, err = db.DB()
	// 设置空闲连接池中连接的最大数量
	sqlDB.SetMaxIdleConns(maxIdleConns)
	// 设置打开数据库连接的最大数量。
	sqlDB.SetMaxOpenConns(maxOpenConns)
	// 设置了连接可复用的最大时间。
	sqlDB.SetConnMaxLifetime(time.Minute * time.Duration(connMaxLifetime))
	// 连接池里面的连接最大空闲时长
	sqlDB.SetConnMaxIdleTime(time.Minute * time.Duration(connMaxIdleTime))
	log.Print("mysql连接成功")
	return db
}

// Redis
/**
 * @Description: 初始化Redis
 * @receiver i
 */
func (i Init) Redis(env global.Env) *redis.Client {
	Prepare(env)
	host := global.V.GetString("Redis.Host")
	port := global.V.GetString("Redis.Port")

	password := global.V.GetString("Redis.Password")
	db := global.V.GetInt("Redis.DB")
	rdb := redis.NewClient(&redis.Options{
		Addr:     host + ":" + port,
		Password: password, // no password set
		DB:       db,       // use default DB
	})
	log.Print(rdb)
	var ctx = context.Background()
	// 可连接性检测
	_, err := rdb.Ping(ctx).Result()
	if err != nil {
		panic("Redis连接池初始失败：" + err.Error())
	}

	log.Print("redis连接成功")
	return rdb
}

// Casbin
/**
 * @Description: 初始化Casbin
 * @receiver i
 */
func (i Init) Casbin(env global.Env) (Enforcer *casbin.SyncedEnforcer) {
	Prepare(env)
	// Gorm适配器
	adapter, err := gormadapter.NewAdapterByDB(global.DB)
	if err != nil {
		panic("Casbin Gorm适配器错误：" + err.Error())
	}
	log.Println("导入适配器")

	m, _ := model.NewModelFromString(`
		[request_definition]
		r = sub, obj, act
		
		[policy_definition]
		p = sub, obj, act
		
		[role_definition]
		g = _, _
		
		[policy_effect]
		e = some(where (p.eft == allow))
		
		[matchers]
		m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
	`)

	// 通过ORM新建一个执行者
	Enforcer, err = casbin.NewSyncedEnforcer(m, adapter)
	if err != nil {
		panic("新建Casbin执行者异常：" + err.Error())
	}
	// 导入访问策略
	err = Enforcer.LoadPolicy()
	if err != nil {
		panic("导入访问策略异常：" + err.Error())
	}
	return Enforcer
}

// Run
/**
 * @Description: 启动服务
 * @receiver i
 * @param r
 */
func (i Init) Run(r *gin.Engine) {
	port := global.V.GetString("Self.RouterPort")
	// 创建一个listener
	l, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatal(err)
	}
	// 创建一个cmux.
	m := cmux.New(l)
	go func() {
		httpL := m.Match(cmux.HTTP1Fast())
		err = r.RunListener(httpL)
		if err != nil {
			return
		}
	}()

	// 启动端口监听
	err = m.Serve()
	if err != nil {
		panic("服务启动失败：" + err.Error())
	}
}

// Validate
/**
 *  @Description: 初始化参数校验器
 *  @receiver i
 *  @return *validator.Validate
 */
func (i Init) Validate() *validator.Validate {
	v := validator.New()
	return v
}

func (i Init) Node() *snowflake.Node {
	nodeNum := global.V.GetInt64("Distributed.Node")
	node, err := snowflake.NewNode(nodeNum)
	if err != nil {
		fmt.Println(err)
	}
	return node
}

// MinIO
/**
 *  @Description: 初始化MinIO客户端
 *  @receiver i
 *  @return *minio.Client
 */
func (i Init) MinIO() *minio.Client {
	endpoint := global.V.GetString("MinIO.endpoint")
	accessKeyID := global.V.GetString("MinIO.accessKeyID")
	secretAccessKey := global.V.GetString("MinIO.secretAccessKey")
	useSSL := global.V.GetBool("MinIO.useSSL")
	// 初始化MioIO客户端
	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		panic(err)
	}
	return minioClient
}

func Prepare(env global.Env) {
	init := Init{}
	global.E = init.Env(env)
	// 初始化Viper（读取配置文件）
	global.V = init.Viper()
	// 初始化雪花ID节点
	global.Node = init.Node()
}
