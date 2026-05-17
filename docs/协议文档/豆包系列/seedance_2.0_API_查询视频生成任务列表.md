`GET https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks?page_num={page_num}&page_size={page_size}&filter.status={filter.status}&filter.task_ids={filter.task_ids}&filter.model={filter.model}`  [运行](https://api.volcengine.com/api-explorer/?action=ListContentsGenerationsTasks&data=%7B%7D&groupName=%E8%A7%86%E9%A2%91%E7%94%9F%E6%88%90API&query=%7B%7D&serviceCode=ark&version=2024-01-01)
当您要查询符合条件的任务，您可以传入条件筛选参数，返回符合要求的任务。
:::tip
仅支持查询最近 7 天的历史数据。时间计算统一采用UTC时间戳，返回的7天历史数据范围以用户实际发起批量查询请求的时刻为基准（精确到秒），时间戳区间为 [T\-7天, T)。

:::
```mixin-react
return (<Tabs>
<Tabs.TabPane title="快速入口" key="opV4RT2k"><RenderMd content={` [ ](#)[体验中心](https://console.volcengine.com/ark/region:ark+cn-beijing/experience/vision)       <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_2abecd05ca2779567c6d32f0ddc7874d.png =20x) </span>[模型列表](https://www.volcengine.com/docs/82379/1330310)       <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_a5fdd3028d35cc512a10bd71b982b6eb.png =20x) </span>[模型计费](https://www.volcengine.com/docs/82379/1099320#%E8%A7%86%E9%A2%91%E7%94%9F%E6%88%90%E6%A8%A1%E5%9E%8B)       <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_afbcf38bdec05c05089d5de5c3fd8fc8.png =20x) </span>[API Key](https://console.volcengine.com/ark/region:ark+cn-beijing/apiKey?apikey=%7B%7D)
 <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_57d0bca8e0d122ab1191b40101b5df75.png =20x) </span>[调用教程](https://www.volcengine.com/docs/82379/1366799)       <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_f45b5cd5863d1eed3bc3c81b9af54407.png =20x) </span>[接口文档](https://www.volcengine.com/docs/82379/1521675)       <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_1609c71a747f84df24be1e6421ce58f0.png =20x) </span>[常见问题](https://www.volcengine.com/docs/82379/1359411)       <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_bef4bc3de3535ee19d0c5d6c37b0ffdd.png =20x) </span>[开通模型](https://console.volcengine.com/ark/region:ark+cn-beijing/openManagement?LLM=%7B%7D&OpenTokenDrawer=false)
`}></RenderMd></Tabs.TabPane>
<Tabs.TabPane title="鉴权说明" key="CPeW5vNl"><RenderMd content={`本接口支持 API Key 鉴权，详见[鉴权认证方式](https://www.volcengine.com/docs/82379/1298459)。
`}></RenderMd></Tabs.TabPane></Tabs>);
```


---


<span id="RxN8G2nH"></span>
## 请求参数
> 跳转 [响应参数](#7mi8G8RI)

:::tip
下面参数为Query String Parameters，在URL String中传入。

:::
---


**page_num** `integer / null`
取值范围：[1, 500]
返回结果的页码。

---


**page_size ** `integer / null`
取值范围：[1, 500]
返回结果的每页的结果数量。

---


**filter.status ** `string / null`
过滤参数，查询某个任务状态。

* `queued`：排队中的任务。
* `running`：运行中任务。
* `cancelled`：取消的任务。
* `succeeded`： 成功的任务。
* `failed`：失败的任务。


---


**filter.task_ids ** `string[] / null`
视频生成任务 ID，精确搜索，支持同时搜索多个任务 ID。多个任务 ID 之间通过 `&`连接。示例：`filter.task_ids=id1&filter.task_ids=id2`。

---


**filter.model ** `string / null`
与返回参数不同，该字段为任务使用的推理接入点 ID，精确搜索。

---


**filter.service_tier ** `string / null` `默认值 default`
处理任务使用的服务等级。

* `default`：在线推理模式
* `flex`：离线推理模式

<span id="7mi8G8RI"></span>
## 响应参数
> 跳转 [请求参数](#RxN8G2nH)


---


**items ** `object[]`
查询到的视频生成任务列表。

属性

---


items.**id ** `string`
视频生成任务 ID 。

---


items.**model** `string`
任务使用的模型名称和版本，`模型名称-版本`。

---


items.**status** `string`
任务状态，以及相关的信息：

* `queued`：排队中。
* `running`：任务运行中。
* `cancelled`：取消任务（只支持排队中状态的任务被取消）。
* `succeeded`： 任务成功。
* `failed`：任务失败。
* `expired`：任务超时。


---


items.**error** `object / null`
错误提示信息，任务成功返回`null`，任务失败时返回错误数据，错误信息具体参见 [错误处理](https://www.volcengine.com/docs/82379/1393047#653d2c40)。

属性

---


error.**code** `string`
错误码。

---


error.**message** `string`
错误提示信息。


---


items.**created_at** `integer`
任务创建时间的 Unix 时间戳（秒）。

---


items.**updated_at** `integer`
任务当前状态更新时间的 Unix 时间戳（秒）。

---


items.**content** `object`
当视频生成任务完成，会输出该字段，包含生成视频下载的 URL。

属性

---


content.**video_url** `string`
生成视频的URL。为保障信息安全，生成的视频会在24小时后被清理，请及时转存。

---


content.**last_frame_url ** `string`
视频的尾帧图像 URL。有效期为 24小时，请及时转存。
说明：[创建视频生成任务](https://www.volcengine.com/docs/82379/1520757) 时设置 `"return_last_frame": true` 时，会返回参数。


---


items.**seed** `integer`
本次请求使用的种子整数值。

---


items.**resolution **  `string`
生成视频的分辨率。

---


items.**ratio ** `string`
生成视频的宽高比。

---


items.**duration** `integer`
生成视频的时长，单位：秒。
说明：**duration 和 frames 参数只会返回一个**。[创建视频生成任务](https://www.volcengine.com/docs/82379/1520757) 时未指定 frames，会返回 duration。

---


items.**frames ** `integer`  
生成视频的帧数。
说明：**duration 和 frames 参数只会返回一个**。[创建视频生成任务](https://www.volcengine.com/docs/82379/1520757) 时指定了 frames，会返回 frames。

---


items.**framespersecond**  `integer`
生成视频的帧率。

---


items.**generate_audio** `boolean`
生成的视频是否包含与画面同步的声音。仅 Seedance 2.0 系列、Seedance 1.5 pro 会返回该参数。

* `true`：模型输出的视频包含同步音频。
* `false`：模型输出的视频为无声视频。


---


items.**tools==^new^==** ** ** `object[]`
本次请求模型实际使用的工具。未使用工具时不返回。

属性
items.tools.**type ** `string`
实际使用的工具类型

* web_search：联网搜索工具。


---


items.**safety_identifier==^new^==** `string`
终端用户的唯一标识符。若 [创建视频生成任务](https://www.volcengine.com/docs/82379/1520757) 时设置了该参数，接口会原样返回此信息。

---


items.**draft** `boolean`
生成的视频是否为 Draft 视频。仅 Seedance 1.5 pro 会返回该参数。

* `true`：表示当前输出为 Draft 视频。
* `false`：表示当前输出为正常视频。


---


items.**draft_task_id ** `string`
Draft 视频任务 ID。基于 Draft 视频生成正式视频时，会返回该参数。

---


items.**service_tier ** `string`
实际处理任务使用的服务等级。

---


items.**execution_expires_after** ** ** `integer`
任务超时阈值，单位：秒。

---


items.**usage** `object`
本次请求的 token 用量。

属性

---


items.usage.**completion_tokens** `integer`
模型输出视频花费的 token 数量，可作为计费对账依据。
:::tip
Seedance 2.0 系列模型存在最低 token 用量限制，如果实际 token 用量 ＜ 最低 token 用量，本字段会返回最低 token 用量，平台按最低 token 用量计费。

:::
---


items.usage.**total_tokens**`integer`
本次请求消耗的总 token 数量。视频生成模型不统计输入 token，输入 token 为 0，故 **total_tokens**=**completion_tokens**。

---


items.usage.**tool_usage==^new^==** ** ** `object`
使用工具的用量信息。

属性
items.usage.tool_usage.**web_search ** `integer`
实际调用联网搜索工具的次数，仅开启联网搜索时返回。





---


**total ** `integer`
符合筛选条件的任务数量。


