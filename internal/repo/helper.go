package repo

import (
	"errors"

	"gorm.io/gorm"
)

// isNoRows 判断错误是否为"查询无结果"。
// 统一处理 gorm.ErrRecordNotFound，供各 repo 方法在查询单条记录时使用，
// 避免将"未找到记录"当作业务错误向上抛出。
func isNoRows(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
