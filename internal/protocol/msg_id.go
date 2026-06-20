package protocol

// ==================== 消息ID定义 ====================
// 按功能模块分段，每段预留100个ID，便于后续扩展

const (
	// 登录模块 1001-1099
	MsgIDLogin            uint16 = 1001 // 客户端→服务端：登录请求
	MsgIDLoginResp        uint16 = 1002 // 服务端→客户端：登录响应
	MsgIDCreatePlayer     uint16 = 1003 // 客户端→服务端：创建角色
	MsgIDCreatePlayerResp uint16 = 1004 // 服务端→客户端：创建角色响应

	// 地下城模块 1101-1199
	MsgIDEnterDungeon      uint16 = 1101 // 客户端→服务端：进入地下城
	MsgIDEnterDungeonResp  uint16 = 1102 // 服务端→客户端：进入地下城响应
	MsgIDLeaveDungeon      uint16 = 1103 // 客户端→服务端：离开地下城
	MsgIDLeaveDungeonResp  uint16 = 1104 // 服务端→客户端：离开地下城响应
	MsgIDDungeonInfo       uint16 = 1105 // 服务端→客户端：地下城信息推送
	MsgIDMonsterRefresh    uint16 = 1106 // 服务端→客户端：怪物刷新通知
	MsgIDLayerTeleport     uint16 = 1107 // 客户端→服务端：层间传送
	MsgIDLayerTeleportResp uint16 = 1108 // 服务端→客户端：层间传送响应

	// 战斗模块 1201-1299
	MsgIDAttack          uint16 = 1201 // 客户端→服务端：普通攻击
	MsgIDDamage          uint16 = 1202 // 服务端→客户端：伤害结果
	MsgIDBossSpawn       uint16 = 1203 // 服务端→客户端：Boss刷新通知
	MsgIDBossDie         uint16 = 1204 // 服务端→客户端：Boss死亡通知
	MsgIDSkillCast       uint16 = 1205 // 客户端→服务端：释放技能
	MsgIDSkillEffect     uint16 = 1206 // 服务端→客户端：技能效果
	MsgIDPlayerDie       uint16 = 1207 // 服务端→客户端：玩家死亡
	MsgIDPlayerRevive    uint16 = 1208 // 服务端→客户端：玩家复活
	MsgIDCollectResource uint16 = 1209 // 客户端→服务端：采集资源
	MsgIDCollectResult   uint16 = 1210 // 服务端→客户端：采集结果
	MsgIDAutoBattle      uint16 = 1211 // 客户端→服务端：开启/关闭自动战斗
	MsgIDAutoBattleResp  uint16 = 1212 // 服务端→客户端：自动战斗状态响应
	MsgIDUseItem         uint16 = 1213 // 客户端→服务端：使用消耗品
	MsgIDUseItemResp     uint16 = 1214 // 服务端→客户端：使用消耗品结果
	MsgIDInventorySync   uint16 = 1215 // 服务端→客户端：库存同步推送

	// 装备模块 1301-1399
	MsgIDEquipStrengthen     uint16 = 1301 // 客户端→服务端：装备强化
	MsgIDEquipStrengthenResp uint16 = 1302 // 服务端→客户端：装备强化结果
	MsgIDEquipEnchant        uint16 = 1303 // 客户端→服务端：装备附魔
	MsgIDEquipEnchantResp    uint16 = 1304 // 服务端→客户端：装备附魔结果
	MsgIDEquipWear           uint16 = 1305 // 客户端→服务端：穿戴装备
	MsgIDEquipWearResp       uint16 = 1306 // 服务端→客户端：穿戴装备结果
	MsgIDEquipUnload         uint16 = 1307 // 客户端→服务端：卸下装备
	MsgIDEquipUnloadResp     uint16 = 1308 // 服务端→客户端：卸下装备结果
	MsgIDForge               uint16 = 1309 // 客户端→服务端：锻造合成
	MsgIDForgeResp           uint16 = 1310 // 服务端→客户端：锻造合成结果

	// PvP模块 1401-1499
	MsgIDPvpAttack    uint16 = 1401 // 客户端→服务端：PvP攻击
	MsgIDPvpResult    uint16 = 1402 // 服务端→客户端：PvP结果
	MsgIDRedNameList  uint16 = 1403 // 服务端→客户端：红名列表
	MsgIDBountyHunt   uint16 = 1404 // 客户端→服务端：悬赏追杀
	MsgIDBountyReward uint16 = 1405 // 服务端→客户端：悬赏奖励
	MsgIDRevenge      uint16 = 1406 // 客户端→服务端：复仇请求
	MsgIDRevengeResp  uint16 = 1407 // 服务端→客户端：复仇响应

	// 移动模块 1501-1599
	MsgIDMove       uint16 = 1501 // 客户端→服务端：玩家移动
	MsgIDPlayerMove uint16 = 1502 // 服务端→客户端：其他玩家移动广播

	// 宠物模块 1601-1699
	MsgIDPetSummon      uint16 = 1601 // 客户端→服务端：召唤宠物
	MsgIDPetSummonResp  uint16 = 1602 // 服务端→客户端：召唤宠物结果
	MsgIDPetRecall      uint16 = 1603 // 客户端→服务端：收回宠物
	MsgIDPetRecallResp  uint16 = 1611 // 服务端→客户端：收回宠物结果
	MsgIDPetLevelUp     uint16 = 1604 // 客户端→服务端：宠物升级
	MsgIDPetEvolve      uint16 = 1605 // 客户端→服务端：宠物进阶
	MsgIDPetEvolveResp  uint16 = 1606 // 服务端→客户端：宠物进阶结果
	MsgIDPetExplore     uint16 = 1607 // 客户端→服务端：宠物探险
	MsgIDPetExploreResp uint16 = 1608 // 服务端→客户端：宠物探险结果
	MsgIDPetCompose     uint16 = 1609 // 客户端→服务端：宠物合成
	MsgIDPetComposeResp uint16 = 1610 // 服务端→客户端：宠物合成结果
	MsgIDPetEquip       uint16 = 1612 // 客户端→服务端：宠物穿戴装备
	MsgIDPetEquipResp   uint16 = 1613 // 服务端→客户端：宠物穿戴装备结果
	MsgIDPetUnequip     uint16 = 1614 // 客户端→服务端：宠物卸下装备
	MsgIDPetUnequipResp uint16 = 1615 // 服务端→客户端：宠物卸下装备结果

	// 交易行模块 1701-1799
	MsgIDTradeList        uint16 = 1701 // 客户端→服务端：查询交易行列表
	MsgIDTradeListResp    uint16 = 1702 // 服务端→客户端：交易行列表
	MsgIDTradePublish     uint16 = 1703 // 客户端→服务端：上架商品
	MsgIDTradePublishResp uint16 = 1704 // 服务端→客户端：上架结果
	MsgIDTradeBuy         uint16 = 1705 // 客户端→服务端：购买商品
	MsgIDTradeBuyResp     uint16 = 1706 // 服务端→客户端：购买结果
	MsgIDTradeCancel      uint16 = 1707 // 客户端→服务端：取消上架
	MsgIDTradeCancelResp  uint16 = 1708 // 服务端→客户端：取消上架结果

	// 商店模块 1801-1899
	MsgIDShopList     uint16 = 1801 // 客户端→服务端：查询商店列表
	MsgIDShopListResp uint16 = 1802 // 服务端→客户端：商店列表
	MsgIDShopBuy      uint16 = 1803 // 客户端→服务端：购买商品
	MsgIDShopBuyResp  uint16 = 1804 // 服务端→客户端：购买结果

	// 技能模块 1901-1999
	MsgIDSkillLevelUp     uint16 = 1901 // 客户端→服务端：技能升级
	MsgIDSkillLevelUpResp uint16 = 1902 // 服务端→客户端：技能升级结果
	MsgIDSkillReset       uint16 = 1903 // 客户端→服务端：技能重置
	MsgIDSkillResetResp   uint16 = 1904 // 服务端→客户端：技能重置结果

	// 属性模块 2001-2099
	MsgIDAttrAssign     uint16 = 2001 // 客户端→服务端：分配属性点
	MsgIDAttrAssignResp uint16 = 2002 // 服务端→客户端：属性分配结果

	// 排行榜模块 2101-2199
	MsgIDRankingList     uint16 = 2101 // 客户端→服务端：查询排行榜
	MsgIDRankingListResp uint16 = 2102 // 服务端→客户端：排行榜数据

	// 组队模块 2201-2299
	MsgIDTeamCreate       uint16 = 2201 // 客户端→服务端：创建队伍
	MsgIDTeamInfoResp     uint16 = 2202 // 服务端→客户端：队伍信息响应（创建/查询/层级传送通用响应）
	MsgIDTeamInvite       uint16 = 2203 // 客户端→服务端：邀请玩家入队
	MsgIDTeamInvitePush   uint16 = 2204 // 服务端→客户端：被邀请通知（推送给目标）
	MsgIDTeamInviteReply  uint16 = 2205 // 客户端→服务端：邀请回复（接受/拒绝）
	MsgIDTeamInviteResult uint16 = 2206 // 服务端→客户端：邀请回复结果（推给邀请者）
	MsgIDTeamLeave        uint16 = 2207 // 客户端→服务端：离开队伍
	MsgIDTeamLeaveResp    uint16 = 2208 // 服务端→客户端：离开队伍结果
	MsgIDTeamDismiss      uint16 = 2209 // 客户端→服务端：解散队伍（队长）
	MsgIDTeamDismissResp  uint16 = 2210 // 服务端→客户端：解散队伍结果
	MsgIDTeamKick         uint16 = 2211 // 客户端→服务端：踢出队员（队长）
	MsgIDTeamKickResp     uint16 = 2212 // 服务端→客户端：踢出队员结果
	MsgIDTeamQuery        uint16 = 2213 // 客户端→服务端：查询我的队伍
	MsgIDTeamUpdate       uint16 = 2214 // 服务端→客户端：队伍状态变更推送（全员）

	// 聊天模块 2301-2399
	MsgIDChatSend        uint16 = 2301 // 客户端→服务端：发送聊天消息
	MsgIDChatSendResp    uint16 = 2302 // 服务端→客户端：发送结果
	MsgIDChatMessage     uint16 = 2303 // 服务端→客户端：聊天消息推送（世界/私聊/队伍）
	MsgIDChatHistory     uint16 = 2304 // 客户端→服务端：查询历史消息
	MsgIDChatHistoryResp uint16 = 2305 // 服务端→客户端：历史消息列表

	// 系统模块 9001-9099
	MsgIDBroadcast uint16 = 9001 // 服务端→客户端：全服广播
	MsgIDHeartbeat uint16 = 9002 // 心跳（双向）
	MsgIDKick      uint16 = 9003 // 服务端→客户端：踢下线
)

// 消息头格式：[2字节消息体长度][2字节消息ID][JSON Body]
// 长度字段只计算消息ID+Body的部分，不包含自身2字节
type Header struct {
	Length uint16 // 消息体长度（消息ID + Body）
	MsgID  uint16 // 消息ID
}
