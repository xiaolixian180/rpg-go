package model

// RankingItem 排行榜条目
type RankingItem struct {
	Rank     int32  // 排名
	PlayerID uint64 // 玩家ID
	Name     string // 玩家名称
	Value    int64  // 分数值（等级/战力/荣誉）
}
