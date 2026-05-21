package repo

import (
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// isNoRows 判断错误是否为"查询无结果"
func isNoRows(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}

// gormClauseOnConflict 构造 GORM ON CONFLICT 子句
func gormClauseOnConflict(columns []string, updateColumns []string) clause.OnConflict {
	cols := make([]clause.Column, len(columns))
	for i, c := range columns {
		cols[i] = clause.Column{Name: c}
	}
	return clause.OnConflict{
		Columns:   cols,
		DoUpdates: clause.AssignmentColumns(updateColumns),
	}
}
