package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const timeLayout = "2006-01-02 15:04:05"

const (
	业务日志表名      = "operation_log"
	业务日志类型_操作   = "operation"
	业务日志类型_代理余额 = "agent_balance"
)

// 所有账号共用日志表，归属字段只用于权限和筛选，不随账号删除而删除历史记录。
type 业务日志记录 struct {
	ID        uint64    `gorm:"column:id;primaryKey;autoIncrement;index:idx_operation_admin_id,priority:2;index:idx_operation_agent_id,priority:3;index:idx_operation_type_id,priority:3" json:"id"`
	AdminID   int       `gorm:"column:admin_id;not null;index:idx_operation_admin_id,priority:1;index:idx_operation_agent_id,priority:1;index:idx_operation_type_id,priority:1" json:"admin_id"`
	AgentID   int       `gorm:"column:agent_id;not null;default:0;index:idx_operation_agent_id,priority:2" json:"agent_id"`
	Type      string    `gorm:"column:type;size:16;not null;index:idx_operation_type_id,priority:2" json:"type"`
	Log       string    `gorm:"column:log;type:longtext;not null" json:"log"`
	CreatedAt time.Time `gorm:"column:created_at;not null" json:"created_at"`
}

func (业务日志记录) TableName() string { return 业务日志表名 }

// 业务流水字段按 Tab 分隔；只清理控制字符，不截断卡密清单等回溯信息。
func 业务流水文本(fields ...string) string {
	cleaned := make([]string, 0, len(fields))
	for _, field := range fields {
		cleaned = append(cleaned, 清理拒绝日志字段(field, 0))
	}
	return strings.Join(cleaned, "\t")
}

// 保存业务日志不创建事务；余额调用方必须传入现有事务，普通操作传入 db。
func 保存业务日志(database *gorm.DB, admin string, agentID int, logType string, fields ...string) error {
	admin = strings.TrimSpace(admin)
	if database == nil || !验证管理员名称(admin) || agentID < 0 {
		return fmt.Errorf("日志归属参数不正确")
	}
	if logType != 业务日志类型_操作 && logType != 业务日志类型_代理余额 {
		return fmt.Errorf("日志类型不正确")
	}
	// 正常请求和后台任务复用已加载的管理员 ID，避免每条日志额外查询管理员表。
	info, exists := 全局_运行状态.读取用户设置(admin)
	adminID := info.ID
	if !exists || adminID <= 0 {
		var account user
		if err := database.Table("user").Select("id").Where("name = ?", admin).First(&account).Error; err != nil {
			return fmt.Errorf("读取日志管理员失败: %w", err)
		}
		adminID = account.ID
	}
	if adminID <= 0 {
		return fmt.Errorf("日志管理员不存在")
	}
	row := 业务日志记录{AdminID: adminID, AgentID: agentID, Type: logType, Log: 业务流水文本(fields...), CreatedAt: time.Now()}
	if err := database.Table(业务日志表名).Create(&row).Error; err != nil {
		return fmt.Errorf("保存业务日志失败: %w", err)
	}
	return nil
}

// 普通操作在业务成功后记录，日志失败不能让已提交的业务被误报为失败。
func 记录管理员代理业务流水(admin string, agentID int, fields ...string) {
	if err := 保存业务日志(db, admin, agentID, 业务日志类型_操作, fields...); err != nil {
		日志("log/启动记录.txt", fmt.Sprintf("写入业务日志失败;管理员:%s;代理ID:%d;%s", admin, agentID, err))
	}
}

// 余额日志和余额变化使用同一事务，写入失败由调用方返回并回滚整笔业务。
func 保存代理余额日志(tx *gorm.DB, admin string, agentID int, change, before, after int64, fields ...string) error {
	if change == 0 {
		return nil
	}
	if agentID <= 0 || (change > 0 && after <= before) || (change < 0 && after >= before) || after-before != change {
		return fmt.Errorf("代理余额日志金额不一致")
	}
	// 日志只展示变更后余额；变动前余额可由上一条余额日志推导，避免重复占用展示空间。
	allFields := append([]string{fmt.Sprintf("余额:%d", after), fmt.Sprintf("变更:%+d", change)}, fields...)
	return 保存业务日志(tx, admin, agentID, 业务日志类型_代理余额, allFields...)
}

// adminID、agentID 由认证和归属校验决定，不能使用客户端参数替换租户范围。
func 查询业务日志(ctx *gin.Context, adminID, agentID int) {
	if db == nil || adminID <= 0 || agentID < 0 {
		失败提示管理端(ctx, "日志归属不正确")
		return
	}
	logType := strings.TrimSpace(input(ctx, "type"))
	if logType != "" && logType != 业务日志类型_操作 && logType != 业务日志类型_代理余额 {
		失败提示管理端(ctx, "日志类型不正确")
		return
	}
	page, _ := strconv.Atoi(input(ctx, "page"))
	pageSize, _ := strconv.Atoi(input(ctx, "page_size"))
	page, pageSize = 规范化流水分页(page, pageSize)
	query := db.Table(业务日志表名).Where("admin_id = ?", adminID)
	if agentID > 0 {
		query = query.Where("agent_id = ?", agentID)
	}
	if logType != "" {
		query = query.Where("type = ?", logType)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		失败提示管理端(ctx, "查询日志失败")
		return
	}
	rows := make([]业务日志记录, 0)
	if err := query.Order("id DESC").Limit(pageSize).Offset((page - 1) * pageSize).Find(&rows).Error; err != nil {
		失败提示管理端(ctx, "查询日志失败")
		return
	}
	成功提示管理端(ctx, gin.H{"data": rows, "num": total, "page": page, "page_size": pageSize})
}

func 业务流水时间(value time.Time) string {
	if value.IsZero() {
		return "无"
	}
	return value.Format(timeLayout)
}
