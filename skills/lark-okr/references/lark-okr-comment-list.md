# okr +comment-list
> **前置条件：** 先阅读 [lark-shared/SKILL.md](../../lark-shared/SKILL.md) 了解认证、全局参数和安全规则；

分页获取单个 Cycle、Objective、KeyResult 或 Progress 下的评论。查询整个周期时使用 [+comment-detail](lark-okr-comment-detail.md)。

## 功能简介

该 shortcut 封装 okr.comment.list，只读取一个 target 的一页评论，并将评论正文按 style 转换为 SemiPlainContent 或 ContentBlock。

## 推荐命令

```bash
# 获取 Objective 下的第一页评论。
lark-cli okr +comment-list --target-type objective --target-id 2345678901234567890

# 使用上一页 token 获取 Progress 下的下一页评论。
lark-cli okr +comment-list --target-type progress --target-id 3456789012345678901 --page-size 100 --page-token "7000000000000000002"

# 以 richtext 输出 KeyResult 评论，但是仅预览请求，不实际获取。
lark-cli okr +comment-list --target-type key_result --target-id 4567890123456789012 --style richtext --dry-run
```

## 参数

| 参数 | 必填 | 默认值 | 说明 |
|---|---|---|---|
| --target-type | 是 | — | cycle、objective、key_result 或 progress。 |
| --target-id | 是 | — | 评论对象 ID，int64 正整数。 |
| --page-size | 否 | 100 | 每页数量，范围 1-100。 |
| --page-token | 否 | "" | 上一次响应中的 token；首页不传。 |
| --user-id-type | 否 | open_id | open_id、union_id、user_id 或 user_key。 |
| --style | 否 | simple | simple 返回 SemiPlainContent；richtext 返回 ContentBlock。 |
| --dry-run | 否 | — | 预览 API 调用而不实际执行。 |
| --format | 否 | json | 输出格式。 |

接口的 department_id_type 参数不在 shortcut 中暴露，也不会传递。

## 工作流程

1. 根据用户需求选择 target-type：周期用 cycle，目标用 objective，关键结果用 key_result，进展用 progress。
2. 如果缺少 ID，使用 [+cycle-list](lark-okr-cycle-list.md)、[+cycle-detail](lark-okr-cycle-detail.md) 或 [+progress-list](lark-okr-progress-list.md) 获取。
3. 执行 +comment-list --target-type "..." --target-id "..."。
4. has_more 为 true 且 page_token 非空时，将 page_token 原样作为下一次调用的 --page-token；不要自行解析或修改 token。
5. Objective/KeyResult 的结果可能同时包含实体级评论和划词评论；通过 selection.id 识别划词评论串。

## 输出

```json
{
  "comments": [
    {
      "id": "7000000000000000001",
      "target": {"target_type": "objective", "target_id": "2345678901234567890"},
      "commentator_id": "ou_xxx",
      "status": "open",
      "create_time": "2025-01-15 10:30:00",
      "update_time": "2025-01-15 10:30:00",
      "selection": {"id": "8000000000000000001", "selected_text": "提升核心接口稳定性"},
      "content": {"text": "请补充指标", "mention": [], "docs": [], "images": []}
    }
  ],
  "has_more": true,
  "page_token": "7000000000000000002",
  "style": "simple"
}
```

comments 是当前页数组，不会自动拉取所有分页；simple 风格返回 SemiPlainContent，richtext 风格返回 ContentBlock。

## 注意事项

- 实体级评论没有 selection；Objective/KeyResult 的划词评论带有 selection.id。
- 列表结果不按评论串嵌套整理，需要评论串结构时使用 +comment-detail。
- 该命令是只读操作，不会改变评论状态。

## 参考

- [lark-okr](../SKILL.md) — OKR 命令、路由和通用约定
- [OKR 实体定义](lark-okr-entities.md) — Comment、评论串和 target 类型
- [okr +comment-detail](lark-okr-comment-detail.md) — 聚合获取周期评论
- [okr +cycle-detail](lark-okr-cycle-detail.md) — 获取 Objective 和 KeyResult ID
- [okr +progress-list](lark-okr-progress-list.md) — 获取 Progress ID
- [lark-shared](../../lark-shared/SKILL.md) — 认证、身份、权限和安全规则
