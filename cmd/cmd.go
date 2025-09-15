package cmd

import (
	"Yearning-go/src/handler/manage/user"
	"Yearning-go/src/i18n"
	"Yearning-go/src/lib/factory"
	"Yearning-go/src/lib/vars"
	"Yearning-go/src/model"
	"Yearning-go/src/service"
	"fmt"
	"github.com/gookit/gcli/v3"
	"github.com/gookit/gcli/v3/builtin"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql" // 导入 MySQL 驱动
	"github.com/spf13/viper"
	"log"
)

var RunOpts = struct {
	port       string
	config     string
	push       string // ?
	repair     bool
	resetAdmin bool
}{}

type MysqlConfig struct {
	Db       string
	Host     string
	Port     string
	User     string
	Password string
}

var DBConfig MysqlConfig

func LoadConfig() {
	viper.SetConfigName("conf") // 文件名 (不带扩展名)
	viper.SetConfigType("toml") // 文件类型
	viper.AddConfigPath(".")    // 检索当前目录
	viper.AddConfigPath("../")  // 检索上一级目录

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file: %v", err)
	}

	// 映射到结构体
	if err := viper.UnmarshalKey("Mysql", &DBConfig); err != nil {
		log.Fatalf("Unable to decode into struct: %v", err)
	}
}

// ?
func RunDatabaseOperations() {
	LoadConfig()
	//fmt.Printf("Loaded config: %+v\n", DBConfig)

	//dsn := "root:123qqq...A@tcp(192.168.118.117:3306)/yearning_go?charset=utf8mb4&parseTime=True&loc=Local"
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		DBConfig.User,
		DBConfig.Password,
		DBConfig.Host,
		DBConfig.Port,
		DBConfig.Db,
	)

	// 注意这里使用了老版本的gorm就不要同时使用新版本的
	// 在这个函数内调用 model.DefaultLogger 就会导致编译失败
	db, err := gorm.Open("mysql", dsn) // 使用旧版的 Open 方法
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// 获取一个访问令牌, 用于调用企业微信API
	token, err := user.GetAccessToken()
	if err != nil {
		log.Fatalf("Failed to get access token: %v", err)
	}

	// 获取研发部门的用户信息
	users, err := user.GetUsers(user.Wechat.DepartmentID, token)

	if err != nil {
		log.Fatalf("Failed to get users: %v", err)
	}

	// 把拉取到的用户信息存入数据库
	user.SaveUsersToDB(users, db)
}

// Wechat 定义了一个名为 wechat 的子命令
var Wechat = &gcli.Command{
	Name:     "wechat",
	Desc:     "初始化企业微信用户", // 命令的描述，用于 --help 显示
	Examples: `{$binName} {$cmd}`,
	//Config: func(c *gcli.Command) {
	//	c.StrOpt(&RunOpts.config, "config", "c", "conf.toml", "配置文件路径,默认为conf.toml.如无移动配置文件则无需配置！")
	//},
	Func: func(c *gcli.Command, args []string) error { // 当命令被执行时调用该函数
		user.LoadConfig() // 通过viper加载部门信息
		RunDatabaseOperations()
		return nil
	},
}

var Migrate = &gcli.Command{
	Name:     "install",
	Desc:     "数据审计平台安装及数据初始化",
	Examples: `{$binName} {$cmd} --config conf.toml`,
	Config: func(c *gcli.Command) {
		c.StrOpt(&RunOpts.config, "config", "c", "conf.toml", "配置文件路径,默认为conf.toml.如无移动配置文件则无需配置！")
	},
	Func: func(c *gcli.Command, args []string) error {
		model.DBNew(RunOpts.config)
		service.Migrate()
		return nil
	},
}

var Fix = &gcli.Command{
	Name: "migrate",
	Desc: "破坏性版本升级修复",
	Config: func(c *gcli.Command) {
		c.StrOpt(&RunOpts.config, "config", "c", "conf.toml", "配置文件路径,默认为conf.toml.如无移动配置文件则无需配置！")
	},
	Func: func(c *gcli.Command, args []string) error {
		model.DBNew(RunOpts.config)
		service.DelCol()
		service.MargeRuleGroup()
		return nil
	},
}

var Super = &gcli.Command{
	Name: "reset_super",
	Desc: "重置admin密码",
	Config: func(c *gcli.Command) {
		c.StrOpt(&RunOpts.config, "config", "c", "conf.toml", "配置文件路径,默认为conf.toml.如无移动配置文件则无需配置！")
	},
	Func: func(c *gcli.Command, args []string) error {
		model.DBNew(RunOpts.config)
		model.DB().Model(model.CoreAccount{}).Where("username =?", "admin").Updates(&model.CoreAccount{Password: factory.DjangoEncrypt("123qqq...A", string(factory.GetRandom()))})
		fmt.Println(i18n.DefaultLang.Load(i18n.INFO_ADMIN_PASSWORD_RESET))
		return nil
	},
}

var RunServer = &gcli.Command{
	Name: "run",
	Desc: "启动 SQL Audit Platform",
	// 配置命令行选项
	Config: func(c *gcli.Command) {
		c.StrOpt(&RunOpts.port, "port", "p", "8000", "服务端口")
		c.StrOpt(&RunOpts.config, "config", "c", "conf.toml", "配置文件路径")
		c.StrOpt(&RunOpts.push, "push", "b", "yearning.io", "钉钉/邮件推送时显示的平台地址") // ?
	},
	Examples: `<cyan>{$binName} {$cmd} --port 80 --push "yearning.io" --config ../config.toml</>`,
	Func: func(c *gcli.Command, args []string) error {
		model.DBNew(RunOpts.config)
		service.UpdateData()
		service.StartYearning(RunOpts.port, RunOpts.push)
		return nil
	},
}

func Command() {
	app := gcli.NewApp()
	app.Version = fmt.Sprintf("%s %s", vars.Version, vars.Kind)
	app.Name = "SAP"
	app.Logo = &gcli.Logo{Text: LOGO, Style: "info"}
	app.Desc = "SQL Audit Platform"
	app.Add(Migrate)
	app.Add(RunServer)
	app.Add(Fix)
	app.Add(Super)
	app.Add(Wechat) // ?
	app.Add(builtin.GenAutoComplete())
	app.Run(nil)
}
