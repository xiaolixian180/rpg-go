#!/usr/bin/env python3
"""
模拟Unity客户端，测试Go游戏服务器所有功能
使用与Unity客户端完全相同的二进制WebSocket协议：
  [2B length big-endian][2B msgID big-endian][JSON body]
"""

import asyncio
import json
import struct
import hmac
import hashlib
import base64
import time
import websockets

SERVER = "ws://127.0.0.1:8081/ws"
JWT_SECRET = "HQ_JWT_SECRET"

# ==================== 协议编解码 ====================

def encode_msg(msg_id: int, body: dict) -> bytes:
    """编码二进制帧: [2B length][2B msgID][JSON]"""
    json_bytes = json.dumps(body, separators=(',', ':')).encode('utf-8')
    length = 2 + len(json_bytes)  # length = msgID(2) + body
    return struct.pack('>HH', length, msg_id) + json_bytes

def decode_msg(data: bytes):
    """解码二进制帧"""
    if len(data) < 4:
        return None, None
    length, msg_id = struct.unpack('>HH', data[:4])
    body = data[4:4+length-2]
    return msg_id, json.loads(body.decode('utf-8')) if body else {}

# ==================== JWT生成 ====================

def base64url_encode(data: bytes) -> str:
    return base64.urlsafe_b64encode(data).rstrip(b'=').decode('ascii')

def generate_jwt(player_id: int) -> str:
    header = base64url_encode(b'{"alg":"HS256","typ":"JWT"}')
    now = int(time.time())
    exp = now + 72 * 3600
    payload = base64url_encode(f'{{"player_id":{player_id},"exp":{exp},"iat":{now}}}'.encode())
    sig_input = f'{header}.{payload}'.encode()
    signature = base64url_encode(hmac.new(JWT_SECRET.encode(), sig_input, hashlib.sha256).digest())
    return f'{header}.{payload}.{signature}'

# ==================== 测试框架 ====================

passed = 0
failed = 0
total = 0

def check(name, condition, detail=""):
    global passed, failed, total
    total += 1
    if condition:
        passed += 1
        print(f"  ✅ {name}")
    else:
        failed += 1
        print(f"  ❌ {name} {detail}")

async def send_and_wait(ws, msg_id, body, expected_resp_id, timeout=5):
    """发送消息并等待指定响应"""
    await ws.send(encode_msg(msg_id, body))
    deadline = time.time() + timeout
    while time.time() < deadline:
        try:
            data = await asyncio.wait_for(ws.recv(), timeout=timeout)
            resp_id, resp_body = decode_msg(data)
            if resp_id == expected_resp_id:
                return resp_body
            # 忽略心跳等其他消息
        except asyncio.TimeoutError:
            break
    return None

async def drain_messages(ws, duration=1):
    """接收一段时间内的所有消息"""
    messages = []
    deadline = time.time() + duration
    while time.time() < deadline:
        try:
            data = await asyncio.wait_for(ws.recv(), timeout=0.5)
            msg_id, body = decode_msg(data)
            messages.append((msg_id, body))
        except asyncio.TimeoutError:
            break
    return messages

# ==================== 测试用例 ====================

async def test_all():
    global passed, failed, total

    print("\n🎮 勇者远征 - 服务器集成测试")
    print("=" * 50)

    async with websockets.connect(SERVER) as ws:
        player_id = 100001
        token = generate_jwt(player_id)

        # ---------- 1. 登录 ----------
        print("\n📡 [1/12] 登录模块")
        resp = await send_and_wait(ws, 1001, {"token": token}, 1002)
        check("登录请求 (1001→1002)", resp is not None)
        if resp:
            check("返回码=0", resp.get("code") == 0, f"code={resp.get('code')}")
            player = resp.get("player", {})
            check("玩家ID存在", player.get("id", 0) > 0)
            check("玩家名称存在", len(player.get("name", "")) > 0)
            player_id = player.get("id", player_id)
            check("等级≥1", player.get("level", 0) >= 1)
            print(f"     玩家: {player.get('name')} Lv.{player.get('level')} 职业:{player.get('class')} 金币:{player.get('gold')}")

        # 如果登录失败，尝试创建角色
        if not resp or resp.get("code") != 0:
            print("     角色不存在，创建新角色...")
            resp = await send_and_wait(ws, 1003, {"token": token, "name": "TestHero", "class": 0}, 1004)
            check("创建角色 (1003→1004)", resp is not None)
            if resp:
                check("创建成功 code=0", resp.get("code") == 0, f"code={resp.get('code')}")
                player_id = resp.get("player", {}).get("id", player_id)

        # ---------- 2. 进入地下城 ----------
        print("\n🏰 [2/12] 地下城模块")
        resp = await send_and_wait(ws, 1101, {"layer": 1}, 1102)
        check("进入地下城 (1101→1102)", resp is not None)
        if resp:
            check("返回码=0", resp.get("code") == 0, f"code={resp.get('code')}")
            check("层数=1", resp.get("layer") == 1)
            monsters = resp.get("monsters", [])
            check("怪物列表非空", len(monsters) > 0, f"count={len(monsters)}")
            check("资源列表存在", "resources" in resp)
            print(f"     区域: {resp.get('zone')} 怪物:{len(monsters)}只 玩家:{len(resp.get('players', []))}人")

        # 等待一些推送消息
        push_msgs = await drain_messages(ws, 2)
        for mid, mbody in push_msgs:
            if mid == 1105:  # DungeonInfo
                check("收到DungeonInfo推送 (1105)", True)
            elif mid == 1106:  # MonsterRefresh
                check("收到MonsterRefresh推送 (1106)", True)

        # ---------- 3. 移动 ----------
        print("\n🚶 [3/12] 移动模块")
        await ws.send(encode_msg(1501, {"x": 10.5, "y": 20.3}))
        push_msgs = await drain_messages(ws, 1)
        check("移动请求发送 (1501)", True)
        # 移动广播可能发给自己
        moved = any(mid == 1502 for mid, _ in push_msgs)
        if moved:
            check("收到PlayerMove广播 (1502)", True)

        # ---------- 4. 战斗 ----------
        print("\n⚔️ [4/12] 战斗模块")
        if monsters:
            target_id = monsters[0].get("id", 0)
            await ws.send(encode_msg(1201, {"target_id": target_id, "skill_id": 0}))
            push_msgs = await drain_messages(ws, 2)
            damage_msgs = [(mid, mbody) for mid, mbody in push_msgs if mid == 1202]
            check("普通攻击发送 (1201)", True)
            check("收到Damage响应 (1202)", len(damage_msgs) > 0)
            if damage_msgs:
                dmg = damage_msgs[0][1]
                check("伤害值>0", dmg.get("damage", 0) > 0, f"damage={dmg.get('damage')}")
                check("目标ID匹配", dmg.get("target_id") == target_id)
                print(f"     伤害:{dmg.get('damage')} 剩余HP:{dmg.get('curr_hp')} 死亡:{dmg.get('is_dead')}")
        else:
            print("     ⚠️ 无怪物，跳过战斗测试")

        # ---------- 5. 技能释放 ----------
        print("\n🔥 [5/12] 技能模块")
        if monsters:
            target_id = monsters[0].get("id", 0)
            await ws.send(encode_msg(1205, {"skill_id": 1, "target_id": target_id, "x": 0, "y": 0}))
            push_msgs = await drain_messages(ws, 2)
            skill_msgs = [(mid, mbody) for mid, mbody in push_msgs if mid == 1206]
            check("技能释放发送 (1205)", True)
            check("收到SkillEffect响应 (1206)", len(skill_msgs) > 0)
            if skill_msgs:
                eff = skill_msgs[0][1]
                check("施法者ID存在", eff.get("caster_id", 0) > 0)
                check("目标列表非空", len(eff.get("targets", [])) > 0)
                print(f"     技能ID:{eff.get('skill_id')} 命中目标:{len(eff.get('targets', []))}个")
        else:
            print("     ⚠️ 无怪物，跳过技能测试")

        # ---------- 6. 技能升级/重置 ----------
        print("\n📖 [6/12] 技能升级模块")
        resp = await send_and_wait(ws, 1901, {"skill_id": 1}, 1902)
        check("技能升级 (1901→1902)", resp is not None)
        if resp:
            check("返回码存在", "code" in resp)
            print(f"     code={resp.get('code')} skill_id={resp.get('skill_id')} new_level={resp.get('new_level')}")

        resp = await send_and_wait(ws, 1903, {}, 1904)
        check("技能重置 (1903→1904)", resp is not None)
        if resp:
            check("返回码存在", "code" in resp)
            print(f"     code={resp.get('code')} refund_points={resp.get('refund_points')}")

        # ---------- 7. 属性分配 ----------
        print("\n💪 [7/12] 属性模块")
        resp = await send_and_wait(ws, 2001, {"attr": "str", "val": 1}, 2002)
        check("属性分配 (2001→2002)", resp is not None)
        if resp:
            check("返回码存在", "code" in resp)
            print(f"     code={resp.get('code')} attr={resp.get('attr')} val={resp.get('val')} 剩余:{resp.get('attr_points')}")

        # ---------- 8. 商店 ----------
        print("\n🛒 [8/12] 商店模块")
        resp = await send_and_wait(ws, 1801, {"type": 0}, 1802)
        check("商店列表 (1801→1802)", resp is not None)
        if resp:
            check("返回码=0", resp.get("code") == 0, f"code={resp.get('code')}")
            items = resp.get("items", [])
            check("商品列表非空", len(items) > 0, f"count={len(items)}")
            if items:
                print(f"     商品数:{len(items)} 第一件:{items[0].get('name')} 价格:{items[0].get('price')}")

        # ---------- 9. 排行榜 ----------
        print("\n🏆 [9/12] 排行榜模块")
        for rtype, rname in [(0, "等级"), (1, "战力"), (2, "荣誉")]:
            resp = await send_and_wait(ws, 2101, {"type": rtype}, 2102)
            check(f"{rname}排行 (2101→2102 type={rtype})", resp is not None)
            if resp:
                check("返回码=0", resp.get("code") == 0, f"code={resp.get('code')}")

        # ---------- 10. 装备 ----------
        print("\n🛡️ [10/12] 装备模块")
        resp = await send_and_wait(ws, 1305, {"slot": 0, "equip_id": 1001}, 1306)
        check("穿戴装备 (1305→1306)", resp is not None)
        if resp:
            check("返回码存在", "code" in resp)
            print(f"     穿戴 code={resp.get('code')} slot={resp.get('slot')}")

        resp = await send_and_wait(ws, 1301, {"slot": 0}, 1302)
        check("装备强化 (1301→1302)", resp is not None)
        if resp:
            check("返回码存在", "code" in resp)
            print(f"     强化 code={resp.get('code')} new_level={resp.get('new_level')}")

        resp = await send_and_wait(ws, 1307, {"slot": 0}, 1308)
        check("卸下装备 (1307→1308)", resp is not None)
        if resp:
            check("返回码存在", "code" in resp)

        resp = await send_and_wait(ws, 1309, {"recipe_id": 1, "materials": []}, 1310)
        check("锻造 (1309→1310)", resp is not None)
        if resp:
            check("返回码存在", "code" in resp)
            print(f"     锻造 code={resp.get('code')} result={resp.get('result_name')}")

        # ---------- 11. 交易行 ----------
        print("\n💱 [11/12] 交易行模块")
        resp = await send_and_wait(ws, 1701, {"category": 0, "page": 1}, 1702)
        check("交易行列表 (1701→1702)", resp is not None)
        if resp:
            check("返回码=0", resp.get("code") == 0, f"code={resp.get('code')}")
            print(f"     商品数:{resp.get('total', 0)}")

        # ---------- 12. 自动战斗 ----------
        print("\n🤖 [12/12] 自动战斗模块")
        resp = await send_and_wait(ws, 1211, {"enable": True}, 1212)
        check("开启自动战斗 (1211→1212)", resp is not None)
        if resp:
            check("返回码存在", "code" in resp)
            check("状态=True", resp.get("enable") == True)
            print(f"     自动战斗: {resp.get('enable')}")

        resp = await send_and_wait(ws, 1211, {"enable": False}, 1212)
        check("关闭自动战斗 (1211→1212)", resp is not None)
        if resp:
            check("状态=False", resp.get("enable") == False)

        # ---------- 心跳 ----------
        print("\n💓 [附加] 心跳")
        heartbeat = {"timestamp": int(time.time() * 1000)}
        await ws.send(encode_msg(9002, heartbeat))
        check("心跳发送 (9002)", True)

    # ==================== 汇总 ====================
    print("\n" + "=" * 50)
    print(f"📊 测试结果: {passed}/{total} 通过", end="")
    if failed > 0:
        print(f"  ❌ {failed} 失败")
    else:
        print("  🎉 全部通过！")
    print("=" * 50)

if __name__ == "__main__":
    asyncio.run(test_all())
