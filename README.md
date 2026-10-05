# Operations Duty Roster

为小型运维团队生成并核查值班安排。仅使用 Go 标准库，不接入日历或通知服务。

## 构建与使用

```sh
go build -o roster .
./roster generate <problem.json> <out.json|->   # 生成排班
./roster verify <problem.json> <assignment.json> # 独立核查外部方案
./roster swap <problem.json> <assignment.json> <shiftA> <shiftB> [out.json|-]
```

无可行方案时 `generate` 以非零退出码报告 `no feasible assignment exists`，不输出缺人的排班。

## 输入格式

```json
{
  "people": [{"id": "A", "qualification": "medic", "max_minutes": 480,
              "unavailable": [{"start": "...", "end": "..."}]}],
  "shifts": [{"id": "s1", "interval": {"start": "...", "end": "..."}, "qualification": "medic"}],
  "config": {"min_rest_minutes": 60},
  "fixed": {"s2": "A"}
}
```

校验规则：

- 人员 1–8 名，班次最多 12 个；人员与班次标识必须唯一且非空。
- 时间一律为 UTC（RFC3339，如 `2026-01-01T08:00:00Z`），精确到分钟，结束必须晚于开始。
- `min_rest_minutes` 与各人 `max_minutes` 必须为非负整数。
- `fixed` 中引用的班次与人员必须存在；未知字段会被拒绝。

## 约束判定

- 区间一律按左闭右开 `[start, end)` 处理：前班结束时刻等于后班开始时刻不算重叠。
- 只有资格匹配且整个班次不与任何不可用区间相交的人才能承接。
- 同一人班次不得重叠；前班结束到后班开始的间隔必须 `>= min_rest_minutes`（恰好等于下限为合法）。
- 每人总值班时长不得超过其 `max_minutes`。

## 排序与最优性规则

- 班次按开始时间升序、开始相同再按班次编号字典序排列，该顺序用于搜索与结果汇总。
- 生成采用带回溯的分支限界搜索（不是贪心），在全部可行方案中：
  1. 先最小化“个人总时长的最大值”；
  2. 再取按上述班次顺序展开的人员编号序列中字典序最小者。
- `fixed` 中的分配在生成时保持不变；若固定分配本身违反约束，则判定无解。

## 换班

`swap` 交换两个已分配班次的承接人，随后对**整个**方案重新检查全部约束（资格、不可用、重叠、休息、时长上限及固定分配），而非仅检查两个班次是否重叠。任何一条不满足则拒绝并输出原因，原方案保持不变；成功时输出新方案与逐条变更说明。

## 测试

```sh
go test ./...
```

覆盖需要回溯才能可行的安排、休息下限边界（恰好等于/少一分钟）、资格不符的换班、违反固定分配的换班、换班后的休息违规以及字典序平局判定。
