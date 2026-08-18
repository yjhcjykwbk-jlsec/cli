# Troubleshooting

本文件覆盖 lark-slides 的通用创建前自检、XML 排障和常见失败处理。命令专属问题优先看对应 reference，例如 `+replace-slide`、`+media-upload`。

## XML Preflight

在真正创建或替换前，至少检查：

- 特殊字符已转义：正文和标题里的 `&`、`<`、`>` 不能裸写；属性值里的裸 `&` 也必须写成 `&amp;`。
- 属性引号安全：XML 属性、shell 引号、JSON 字符串包装之间没有互相打断。
- 结构合法：`<slide>` 下只放 `<style>`、`<data>`、`<note>`，文本都在 `<content>` 内。
- 图片路径正确：`<img src="@...">` 占位符由 `+create` 和 `+add-slide` 处理。

## Failure Order

遇到 `invalid param`、某一页创建失败、页面空白或布局错乱时，按顺序处理：

1. 记录 `xml_presentation_id`，不要假设失败代表什么都没创建。
2. 用 `slides +xml-get` 回读，确认是否已有部分页面写入。
3. 检查失败页是否含未转义字符：`Q&A -> Q&amp;A`，文本 `<` / `>` 写成 `&lt;` / `&gt;`，属性 URL `a=1&b=2 -> a=1&amp;b=2`。
4. 检查标签闭合、属性引号、`<content>` 结构，以及 `<slide>` 直接子元素。
5. 页面空白、溢出、重叠或越界时，按 [validation-xml.md](validation-xml.md) 运行 `xml_lint.py`；先修复所有 `error`，再对 `warning` 指向的页面和元素做截图复核。
6. 如果使用 `--slides '[...]'` 字面量，怀疑 shell 转义或截断时改用文件输入：`+create --slide @page-01.xml --slide @page-02.xml`。
7. 局部问题用 `+replace-slide` 块级修正；整页结构要改时用 `+delete-slide` 删旧页 + `+add-slide` 建新页。

## Symptom Fixes

| 看到的问题 | 处理方式 |
|-----------|----------|
| 文字被截断 / 看不全 | 增大 shape 的 `width` 或 `height`，或减少文本量 |
| 元素重叠 | 调整 `topLeftX` / `topLeftY`，拉开间距 |
| 页面大面积空白 | 回读确认内容是否写入；若内容存在，再缩小间距或增加主体元素 |
| 文字和背景色太接近 | 深色背景用浅色文字，浅色背景用深色文字 |
| 表格列宽不合理 | 调整 `colgroup` 中 `col` 的 `width` 值 |
| 图表没有显示 | 检查 `chartPlotArea` 和 `chartData` 是否都包含，`dim1` / `dim2` 数据数量是否匹配 |
| 图片被裁掉一部分 | `<img>` 的 `width` / `height` 是裁剪后尺寸；要整图显示就让 `width:height` 对齐原图比例 |
| 图片不显示 / `<img src>` 仍是 `@path` | `@` 占位符由 `+create` 和 `+add-slide` 替换 |
| 新插入的 `<img>` 挡住原有元素 | `slide.get` 读原页，对照已有块坐标挑空白位置；空间不够就在同一批 `--parts` 里先移动/缩小现有块再插图 |
| 渐变背景变成白色 | 渐变必须用 `rgba()` 格式 + 百分比停靠点，如 `linear-gradient(135deg,rgba(30,60,114,1) 0%,rgba(59,130,246,1) 100%)` |
| 整体风格不统一 | 封面页和结尾页用同一背景，内容页保持一致的配色和字号体系 |

## Common Errors

| 错误码 / 信号 | 含义 | 解决方案 |
|--------------|------|----------|
| 400 XML 格式错误 | XML 语法错误 | 检查标签闭合、属性引号、特殊字符转义 |
| 400 请求包装错误 | `--data` 未按 schema 包装 | 检查是否传入 `xml_presentation.content` 或 `slide.content` |
| 创建成功但页面空白 / 内容缺失 / 布局错乱 | 常见于 `--slides '[...]'` 字面量的 shell 转义或长参数传递问题 | 改用 `--slide @file`（每页一个文件）或 `--slides @deck.json`，并在创建后立即读取 XML 验证 |
| 403 权限不足 | scope 或文档权限不匹配 | 确认 scope 和文档权限；无权限时根据错误响应引导用户解决 |
| 404 演示文稿不存在 | `xml_presentation_id` 不正确或无权限 | 检查 token；wiki URL 需先解析真实 `obj_token` |
| 404 幻灯片不存在 | `slide_id` 不正确 | 重新读取 presentation 或 slide，确认最新 ID |
| 1061002 媒体上传 params error | slides 媒体上传参数不符合约定 | 用 `slides +media-upload`，不要手拼原生 `medias/upload_all`；slides 唯一可用 `parent_type` 是 `slide_file` |
| 1061004 forbidden | 当前用户对演示文稿无编辑权限 | 确认当前用户对目标 PPT 有编辑权限 |
| 3350001 | XML 非 well-formed、XML 结构不符合服务端要求，或 replace 片段问题 | 优先检查未转义字符；replace 场景再看 `block_id` 和 `<content/>` |
| 3350002 | `revision_id` 大于当前版本 | 用 `-1` 取当前版本，或重新用 `slides +xml-get` 取最新 `revision_id` |
| 4000153 `xml lint blocked` | 服务端版式门禁拒绝了这次写入，详见下节 | message 是 JSON，按 `issues[]` 里逐条 finding 修 XML 后重试；确认门禁判错且这页必须原样发布时才加 `--no-lint` |
| validation: unsafe file path | `--file` 给了绝对路径或上层路径 | `--file` 必须是 CWD 内相对路径；先 `cd` 到素材目录再执行 |

## 服务端版式门禁（4000153）

`+create`、`+add-slide`、`+update-slide`、`+replace-slide` 四条写入路径默认都请求服务端跑版式 lint，不合格就拒绝写入，错误码 `4000153`。

`error.message` 是一份 **JSON 文档**，不是人读散文；四条快捷命令和直接 `lark-cli api` 调接口拿到的是同一个形状，可以直接解析：

| 字段 | 含义 |
|------|------|
| `blocked` | 阻断条数，等于 `len(issues)`。**没有非阻断 finding 这回事**：报出来的每一条都得改掉才能写进去 |
| `issues[]` | 服务端报出的**每一条** finding，不截断、不按级别过滤；顺序是先文档级、再按页号，同页内 error → warning → info |
| `issues[].level` | 该条的级别（`error` / `warning` / `info`）。它不决定拦不拦——三级都拦——只说明先修哪条更值 |
| `issues[].code` | 规则名，如 `shape_out_of_canvas`、`blank_slide`、`bbox_overlap` |
| `issues[].slide_number` | 页号；整份提交时是真实页序，所以一次能报出横跨多页的问题。文档级 finding 不带这个字段（不是 0） |
| `issues[].message` | 具体到元素的现象，元素位置写在文本里（如 `shape slide[2]/data/shape[1] exceeds the 960x540 canvas`） |
| `issues[].hint` | 怎么改 |
| `summary` | 服务端对整份内容的判定：各级别计数、`status`、`release_ready`、`screenshot_review_required` 等 |
| `schema_issues` | schema 清洗报告（可选） |

**finding 里除上面这几个字段外还有什么，取决于 `code`**——服务端把每条规则自己的判据原样带回来了，不同规则带的是不同的数：`bbox_overlap` 带 `measurement.intersection_area` 和重叠区的宽高，`text_overflows_container` 带 `overflow` 每条边溢出多少 px，覆盖率类规则带 `measurement` 的比例和 `rule.threshold`。定位类的 finding 常带 `related_objects[]`，里面是 `element_id` / `xml_path` / `bbox`——`hint` 说「Locate via related_objects[].xml_path」时指的就是它，且**这类 finding 往往没有 `path`**，位置只能从 `related_objects` 取。解析时把这些都当可选字段读，不要假设固定形状。

`error.hint` 是 CLI 加的摘要——阻断条数、落在哪几页，以及 `--no-lint` 这个服务端不知道的参数。它不复述 finding，**具体怎么改只看 `message`**。

`issues[]` 是一份待办，不是一份分级报告：`info` 那条跟 `error` 那条一样拦着这次写入，挑着修不会让重试通过。

被拒时写入没有发生，不需要回读收拾：

- `+create` 带页面时是整份提交，任何一页不过就整体拒绝，**演示文稿本身也不会被创建**。
- `+add-slide` / `+update-slide` / `+replace-slide` 拒绝的单位是被写的那一页，页面维持原状。

注意 `+replace-slide` 的判定主体是**拼装后的整页**，不是提交的片段：片段本身合法、但把邻居挤出画布，同样会被拒；反过来，页面上已有的越界元素也会在这次提交里被报出来。

`--no-lint` 是逃生口，只在门禁判错、而这页必须原样发布时用。它只关这一次调用的检查，不改变服务端配置。

## Command-Specific References

- 图片上传、`@path` 占位符、`file_token`：见 [lark-slides-media-upload.md](../cli/lark-slides-media-upload.md) 和 [lark-slides-create.md](../cli/lark-slides-create.md)。
- 块级替换、`block_id`、3350001 replace 细节：见 [lark-slides-replace-slide.md](../cli/lark-slides-replace-slide.md)。
- 追加/插入单页、`--before-slide-id` 和 `--slide @file` 绕开转义：见 [lark-slides-add-slide.md](../cli/lark-slides-add-slide.md)。
