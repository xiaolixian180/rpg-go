package service

import "hero-quest/internal/service/iface"

// World 和 GameConfig 从 iface 子包重新导出，
// 保持 service.World 的引用路径不变，减少外部包的改动。
type World = iface.World
type GameConfig = iface.GameConfig
