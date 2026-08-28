# okr +comment-create
> **前置条件：** 先阅读 [lark-shared/SKILL.md](../../lark-shared/SKILL.md) 了解认证、全局参数和安全规则；

创建一条 OKR 评论，或回复实体级评论、挂入已有的划词评论串。评论正文支持 simple（SemiPlainContent）和 richtext（ContentBlock）两种输入风格。

## 功能简介

- Cycle/Progress 只支持实体级评论；可选 ref-comment-id 回复已有评论。
- Objective/KeyResult 只支持划词评论，必须在 selected-text、select-all、ref-comment-id 三种方式中选择一种。
- selected-text 创建新的划词；ref-comment-id 将评论加入被引用评论所在的划词串。

## 推荐命令

```bash
# 在 Progress 下创建实体级评论。
lark-cli okr +comment-create --target-type progress --target-id 3456789012345678901 --content '{"text":"进展不错"}'

# 在 Objective 正文中创建指定文本的划词评论。
lark-cli okr +comment-create --target-type objective --target-id 2345678901234567890 --content '{"text":"请补充数据"}' --selected-text '提升核心接口稳定性'

# 在 Objective 正文中创建划词评论。
lark-cli okr +comment-create --target-type objective --target-id 2345678901234567890 --content '{"text":"请补充数据"}' --select-all

# 在 KeyResult 的已有划词评论串中追加回复。
lark-cli okr +comment-create --target-type key_result --target-id 4567890123456789012 --content '{"text":"已回复"}' --ref-comment-id 7000000000000000004

# 使用 richtext 文件作为评论正文。
lark-cli okr +comment-create --target-type progress --target-id 3456789012345678901 --style richtext --content '@comment.json'
```

```bash
# 写入前预览创建评论的 URL、参数和请求体。
lark-cli okr +comment-create --target-type progress --target-id 3456789012345678901 --content '{"text":"进展不错"}' --dry-run
```

## 参数

| 参数 | 必填 | 默认值 | 说明 |
|---|---|---|---|
| --target-type | 是 | — | cycle、progress、objective 或 key_result。 |
| --target-id | 是 | — | 评论对象 ID，int64 正整数。 |
| --content | 否¹ | — | 评论正文；simple 输入 SemiPlainContent JSON，richtext 输入 ContentBlock JSON。支持 @文件路径。 |
| --selected-text | 否¹ | — | Objective/KeyResult 新建划词时的完整纯文本。 |
| --select-all | 否¹ | false | Objective/KeyResult 使用全文 fallback；必须同时传 --content。 |
| --ref-comment-id | 否¹ | — | 回复 Progress/Cycle 评论，或将 Objective/KeyResult 评论挂入已有划词串。 |
| --style | 否 | simple | 输入/输出风格：simple 或 richtext。 |
| --user-id-type | 否 | open_id | open_id、union_id、user_id 或 user_key。 |
| --dry-run | 否 | — | 预览 API 调用而不实际执行。 |
| --format | 否 | json | 输出格式。 |

> ¹ content 对 Cycle/Progress 评论可选，但 select-all 必须有 content；Objective/KeyResult 新建评论需要 content。

接口的 department_id_type 参数不在 shortcut 中暴露，也不会传递。

## 工作流程

1. 确定评论 target：使用 [+cycle-detail](lark-okr-cycle-detail.md) 获取 Objective/KeyResult ID，使用 [+progress-list](lark-okr-progress-list.md) 获取 Progress ID；已有评论串时使用 [+comment-list](lark-okr-comment-list.md) 或 [+comment-get](lark-okr-comment-get.md) 获取 comment-id。
2. 根据 target-type 选择评论形式：
   - cycle/progress：不传 selected-text 或 select-all；需要回复时传 ref-comment-id。
   - objective/key_result：在 selected-text、select-all、ref-comment-id 中选择且只能选择一个。
3. 准备 content：simple 使用 SemiPlainContent JSON；需要样式、图片、文档链接或精确 mention 结构时使用 richtext 和 ContentBlock JSON。
4. 执行命令；真实写入前可以先使用 --dry-run 检查 URL、query 和 body。
5. Objective/KeyResult 使用 selected-text 时，只填写正文中真实存在的纯文本片段；不要包含 mention 占位符。select-all 会根据正文 SemiPlainContent 纯文本长度生成等长的 *，触发服务端全文 fallback。

## 输出

创建成功返回 JSON：

```json
{
  "comment_id": "7000000000000000004",
  "selection_id": "8000000000000000002"
}
```

- comment_id 是新评论 ID。
- selection_id 只在创建划词评论时返回，用于识别评论串。
- 创建接口不直接返回完整 Comment；需要详情时使用 [+comment-get](lark-okr-comment-get.md)。

## 注意事项

- Objective/KeyResult 的 ref-comment-id 只用于定位已有划词串，不会在新评论的 ref_comment_id 字段建立引用关系。
- Progress/Cycle 是实体级评论；Progress 的 ref-comment-id 会建立普通评论之间的引用关系。
- simple 输入不支持 docs/images 字段；需要这些内容时使用 richtext。simple 的 mention 应按 SemiPlainContent 约定填写。
- 评论创建是写操作，确认 target、正文和评论形式后再执行；不要把 select-all 当作普通的字面量选区。

## 参考

- [lark-okr](../SKILL.md) — OKR 命令、路由和通用约定
- [OKR 实体定义](lark-okr-entities.md) — Comment、评论串和 target 类型
- [ContentBlock 格式](lark-okr-contentblock.md) — simple/richtext 输入格式
- [okr +comment-list](lark-okr-comment-list.md) — 查询已有评论和 selection.id
- [okr +comment-get](lark-okr-comment-get.md) — 获取评论详情
- [okr +comment-solve / +comment-reopen](lark-okr-comment-solve-reopen.md) — 管理评论状态
- [lark-shared](../../lark-shared/SKILL.md) — 认证、身份、权限和安全规则
