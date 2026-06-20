// Package boss - Boss掉落辅助函数
package boss

import (
	"math/rand"

	"hero-quest/internal/model"
)

// pickEquipByQuality 按品质从真实装备模板中随机选取一件装备。
// 返回装备模板ID和名称。若对应品质无装备则回退到白色品质。
func pickEquipByQuality(quality int32) (int32, string) {
	// 按品质分组收集真实装备模板
	var candidates []*model.EquipTemplate
	for _, tpl := range model.EquipTemplates {
		if tpl.Quality == quality {
			candidates = append(candidates, tpl)
		}
	}
	// 回退：无对应品质装备时使用白色品质
	if len(candidates) == 0 {
		for _, tpl := range model.EquipTemplates {
			if tpl.Quality == model.QualityWhite {
				candidates = append(candidates, tpl)
			}
		}
	}
	// 极端兜底
	if len(candidates) == 0 {
		return 1, "木剑"
	}
	picked := candidates[rand.Intn(len(candidates))]
	return picked.ID, picked.Name
}
