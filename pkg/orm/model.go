package orm

import (
	"database/sql"
	"database/sql/driver"
	"encoding/json"

	"gorm.io/gorm"
)

// Deprecated: 请使用 GetEnabledAutoMigrate / SetEnabledAutoMigrate
var EnabledAutoMigrate bool

func SetEnabledAutoMigrate(v bool) {
	EnabledAutoMigrate = v
}

func GetEnabledAutoMigrate() bool {
	return EnabledAutoMigrate
}

// JSONValueScanner 数据库类型定义为 json 的结构体应当实现此接口
type JSONValueScanner interface {
	sql.Scanner
	driver.Valuer
}

// Model int id 模型
// sqlite 不支持 default:now()，支持 CURRENT_TIMESTAMP
type Model struct {
	ID        int  `gorm:"primaryKey;" json:"id"`
	CreatedAt Time `gorm:"notNull;default:CURRENT_TIMESTAMP;index;comment:创建时间" json:"created_at"`
	UpdatedAt Time `gorm:"notNull;default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"`
}

// ModelWithStrID string id 模型
type ModelWithStrID struct {
	ID        string `gorm:"primaryKey;" json:"id"`
	CreatedAt Time   `gorm:"notNull;default:CURRENT_TIMESTAMP;index;comment:创建时间" json:"created_at"`
	UpdatedAt Time   `gorm:"notNull;default:CURRENT_TIMESTAMP;comment:更新时间" json:"updated_at"`
}

func (d *ModelWithStrID) BeforeCreate(*gorm.DB) error {
	d.CreatedAt = Now()
	d.UpdatedAt = Now()
	return nil
}

func (d *ModelWithStrID) BeforeUpdate(*gorm.DB) error {
	d.UpdatedAt = Now()
	return nil
}

func (d *Model) BeforeCreate(*gorm.DB) error {
	d.CreatedAt = Now()
	d.UpdatedAt = Now()
	return nil
}

func (d *Model) BeforeUpdate(*gorm.DB) error {
	d.UpdatedAt = Now()
	return nil
}

// NewModelWithStrID 新建模型
func NewModelWithStrID(id string) ModelWithStrID {
	return ModelWithStrID{ID: id, CreatedAt: Now(), UpdatedAt: Now()}
}

// DeletedModel 删除模型
type DeletedModel struct {
	Model
	DeletedAt gorm.DeletedAt `gorm:"index;comment:删除时间" json:"-"`
}

func (d *DeletedModel) BeforeCreate(*gorm.DB) error {
	d.CreatedAt = Now()
	d.UpdatedAt = Now()
	return nil
}

func (d *DeletedModel) BeforeUpdate(*gorm.DB) error {
	d.UpdatedAt = Now()
	return nil
}

type DeletedAt = gorm.DeletedAt

// Tabler 模型需要用指针接收器实现接口
type Tabler interface {
	TableName() string
}

// JSONUnmarshal 将 input 反序列化到 obj 上
func JSONUnmarshal(input, obj any) error {
	if v, ok := input.([]byte); ok {
		return json.Unmarshal(v, obj)
	}
	if v, ok := input.(string); ok {
		return json.Unmarshal([]byte(v), obj)
	}
	return nil
}
